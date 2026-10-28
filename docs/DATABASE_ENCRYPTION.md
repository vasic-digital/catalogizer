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
3. the original is kept as `<db>.plaintext-backup-<ts>` (plus its `-wal`/`-shm`
   if present), mode `0600`;
4. the encrypted file is atomically renamed into place.

Any failure before step 3 leaves the original untouched.
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
   `<db>.pre-rekey-<ts>` (encrypted with the OLD key, with `-wal`/`-shm` if
   present) and `<keyfile>.pre-rekey-<ts>` (the OLD key, mode `0600`);
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
* **Restore** (server stopped): `./catalog-api db-restore <backup-file>`. The
  existing database is moved aside to `<db>.pre-restore-<ts>` first; the key file
  must be the one the backup was made with.

## FAQ

**Where is my key?** See "Location" above; `ls -l` it, it must be `-rw-------`.
**I lost the key file.** The data cannot be recovered; restore the key from your
separate key backup. This is the point of encryption at rest.
**The server says "insecure permission".** `chmod 600 <keyfile>`.
**Can I use my own key?** Put 64 hex characters (32 bytes) in the key file (mode
`0600`) before first start (for a new database).
**Docker/Podman**: mount a volume for the key file and set
`DATABASE_ENCRYPTION_KEY_FILE`; do not bake the key into the image.
