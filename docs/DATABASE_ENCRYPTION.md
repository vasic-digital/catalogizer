# Database Encryption at Rest (CAT-001)

Catalogizer's main SQLite database is encrypted with **SQLCipher** (via
`go-sqlcipher`). PostgreSQL deployments are unaffected (use PostgreSQL's own
at-rest encryption / TLS).

## How it works

* **Key**: 32 random bytes (`crypto/rand`), hex-encoded, generated automatically
  on the first run. It is never logged and never appears in error messages.
* **Location** (outside the repository, mode `0600`), first match wins:
  1. `database.encryption_key_file` in `config.json`
  2. environment variable `DATABASE_ENCRYPTION_KEY_FILE`
  3. default: `<user config dir>/catalogizer/db.key`
     (Linux: `~/.config/catalogizer/db.key`, macOS: `~/Library/Application Support/catalogizer/db.key`)
* **Applied to every pooled connection** (driver DSN parameter `_pragma_key`).
* **Fail closed**: if the key file is unreadable, a symlink, owned by another
  user, group/world accessible (anything other than mode `0600`), malformed, or
  the key is wrong, the server refuses to start. There is **no plaintext
  fallback** for file databases. The only opt-out is an in-memory database
  (`:memory:`), which is never persisted.
* A key is only auto-generated for a **new or plaintext** database. If an
  encrypted database exists but the key file is missing, the server refuses to
  start rather than minting a key that cannot open it.

## Upgrading an existing (plaintext) installation

Start the server as usual. On startup a plaintext database is detected and
migrated automatically:

1. the data is exported into `<db>.encrypting-<ts>` with `sqlcipher_export`;
2. per-table row counts and `user_version` are verified to be identical;
3. the original is kept as `<db>.plaintext-backup-<ts>`, mode `0600`. During
   steps 1-4 the plaintext database is held under an exclusive lock (and taken
   out of WAL mode, which folds the WAL into the main file), so a concurrent
   commit is refused instead of being lost, and the migration refuses ("in
   use") when another process has the database open. A non-empty plaintext
   `-wal`/`-shm` that still exists at swap time makes the migration refuse;
   empty ones are deleted;
4. the encrypted file is atomically renamed into place.

Any failure before step 3 leaves the original data intact. Taking the lock
switches the plaintext database from WAL to rollback (DELETE) journal mode; this
is harmless and the server sets WAL again when it opens the database. The same
applies to a restore target after the in-use probe. On Windows the lock is
released just before the final link and rename (an open file cannot be renamed
over there), so a short unlocked window remains.
**The `.plaintext-backup-*` file contains your data unencrypted. After you have
verified the encrypted database, delete it securely** (and take your own
backups of the key file, see below).

## Rotating the key (re-key)

Stop the server, then run:

```bash
./catalog-api db-rekey
```

(or `podman run ... catalog-api db-rekey` in container deployments, with the same
`config.json`/environment and volumes). The command:

1. opens the database with the current key and takes a write lock;
2. makes a **§9.2 backup** before touching anything:
   `<db>.pre-rekey-<ts>` (encrypted with the OLD key; it is a
   `sqlcipher_export` snapshot, so it has no `-wal`/`-shm` sidecars) and `<keyfile>.pre-rekey-<ts>` (the OLD key, mode `0600`);
3. stages the new key in `<keyfile>.new`, runs `PRAGMA rekey`, verifies that the
   new key opens the database with identical row counts and the old key no
   longer does, then atomically installs the new key file.

If a previous run was interrupted, `<keyfile>.new` is left in place and the
next run refuses to start until you inspect it (it may hold the key the database
is currently encrypted with).
Keep the pre-rekey database and old-key backup only as long as you need the
rollback option, then delete both securely.

## Backups and restore

* **Admin backup** (`POST /api/v1/admin/backups`) now uses a database-aware
  backup: WAL checkpoint, write lock, and copy of the main file **and its
  `-wal`/`-shm` sidecars**. The backup stays encrypted (mode `0600`).
  (Previously the endpoint raw-copied only the main file, silently losing every
  commit still in the WAL.)
* **The key is not inside the backup.** Back up the key file separately and
  *not* next to the database backups. Without the key a backup is unreadable
  (by design).
* Backup file names are unique (`backup_<ts>_<n>.db`); a backup never overwrites
  an existing file, and a failed backup removes only the files it created.
* **Restore** (server stopped): `./catalog-api db-restore <backup-file>`. The
  backup is first copied to a staging file and **opened with the active key**;
  a plaintext backup, a backup made with another key, or a corrupt one is
  refused before anything is touched. A running server is detected (exclusive
  lock probe) **while it holds a connection**: the probe fails closed when the
  database is busy or locked. A server whose connection pool is currently idle
  (connections are closed after 3 minutes idle) holds no lock and cannot be
  detected, so stop the server before restoring or re-keying. The probe lock is
  kept until the restored file has been renamed into place (not on Windows). The live database is then hard-linked to
  `<db>.pre-restore-<ts>` (a hard link: it has the same file mode as the live
  database, so keep the data directory private) and the verified staging file is renamed over it, so
  there is no moment without a database file.

## Key file safety checks

* The key file must be a regular file (never a symlink; it is opened with
  `O_NOFOLLOW` and checked via `fstat` on the open descriptor, no TOCTOU), mode
  `0600`, owned by the running user, in a directory that is not group/world
  writable (use `0700`).
* On Windows the POSIX mode/owner/directory checks are skipped (not applicable);
  protect the file with NTFS ACLs. UNCONFIRMED: the Windows build is not tested
  in CI here.
* Migration and `PRAGMA rekey` swap files with hard link + atomic rename. The
  database directory must support hard links; if not, the error says so and
  nothing is changed.

## Crypto library and what the tool checks

The driver is `mutecomm/go-sqlcipher`, which bundles an OLD build: SQLCipher
3.4.2 on SQLite 3.20.1 (AES-256-CBC, HMAC-SHA1, PBKDF2 with 64000 iterations
for passphrases; Catalogizer supplies a raw 256-bit key so KDF cost does not
apply). Upgrading the bundled library is tracked separately.

The `db-*` tooling checks that: the file opens with the configured key
(`cipher_version` plus a read of `sqlite_master`), row counts of every table
match before and after migration/re-key, and the old key no longer opens the
database after a re-key. It does NOT check: page-level integrity beyond that
(run `PRAGMA cipher_integrity_check` yourself), key strength of a user-supplied
key, or protection against an attacker who can read process memory or the key
file.

## FAQ

**Where is my key?** See "Location" above; `ls -l` it, it must be `-rw-------`.
**I lost the key file.** The data cannot be recovered; restore the key from your
separate key backup. This is the point of encryption at rest.
**The server says "insecure permission".** `chmod 600 <keyfile>`.
**Can I use my own key?** Put 64 hex characters (32 bytes) in the key file (mode
`0600`) before first start (for a new database).
**Docker/Podman**: the shipped images set
`DATABASE_ENCRYPTION_KEY_FILE=/app/keys/db.key` and declare `/app/keys` as a
`0700` volume owned by the app user (`docker-compose.test.yml` mounts a named
`api-keys` volume). Keep that volume separate from the data volume and from
backups; never bake a key into an image. The root and `deployment/` compose files
mount a named key volume as well. Without a persistent key volume a recreated
container finds an existing encrypted database but no key and **refuses to
start** (fail closed: it never generates a new key for an existing database);
restore the key from your key backup.
