package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ErrCopyDestIsDirectory is returned when the destination of a copy is an existing directory. An external "cp src dir" would write
// dir/<base(src)> through whatever entry of that name already exists (including an in-root link pointing outside the root), a destination
// the confinement check never saw (CAT-FS-SYMLINK-1 review round 2, R2-1). The copy API names the destination file, never a directory.
var ErrCopyDestIsDirectory = errors.New("copy destination is an existing directory")

// ErrCopySameFile is returned when source and destination are the same file (same path, an alias through a directory link, a link to the
// source, or a hard link). Writing the destination would destroy the source (GNU cp refuses with "are the same file"). CAT-FS-SYMLINK-1 round 3, R3-1.
var ErrCopySameFile = errors.New("copy source and destination are the same file")

// ErrCopySourceNotRegular is returned when the copy source is not a regular file (directory, FIFO, device, socket).
var ErrCopySourceNotRegular = errors.New("copy source is not a regular file")

// copyChunk is the size of one read/write step; the context is checked between steps.
const copyChunk = 1 << 20

// Test seams (never set in production): copyTempHook receives the path of the temp file once created, copyChunkHook runs after each chunk.
var (
	copyTempHook  func(tmp string)
	copyChunkHook func()
)

// copyConfinedFile is THE copy implementation for every client that maps a remote path onto a local tree (LocalClient, NFSClient on linux
// and darwin; CAT-FS-SYMLINK-1 round 3, R3-1: one helper, so the class "destination truncated before the source is read" is closed once).
// No external command is used. Order of events:
//
//  1. ctx pre-check; src and dst confined with confineReal (both followed at the last component);
//  2. the source is stat'ed and opened: not a regular file => ErrCopySourceNotRegular (nothing created, nothing truncated);
//  3. an existing destination is stat'ed: a directory => ErrCopyDestIsDirectory, the same file as the source (os.SameFile: identical path,
//     alias through a directory link, link to the source, hard link) => ErrCopySameFile;
//  4. only then is the data written to a temp file created (O_EXCL) next to the destination's real target, given the source's
//     permission bits, and renamed over the destination. A failed, cancelled or partial copy removes the temp file and never touches an
//     existing destination.
//
// Consequences, by design: (a) a destination that is a link is resolved first and the rename replaces its REAL target (write-through, as
// os.Create did; the link itself survives, and its target is inside the root unless the per-root opt-out is on); (b) the destination appears
// atomically; (c) the destination's mode is the source's permission bits (no umask), not 0666&^umask; (d) the destination's parent
// directories are created (MkdirAll 0755) as the Linux clients always did; (e) a temp file ".copy-*.tmp" exists in the destination directory
// while the copy runs. The check-to-use race is documented in docs/security/PATH_CONFINEMENT.md.
func copyConfinedFile(ctx context.Context, root, srcFull, dstFull, srcShown, dstShown string, allowEscape bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	src, err := confineReal(root, srcFull, srcShown, allowEscape, true)
	if err != nil {
		return err
	}
	dst, err := confineReal(root, dstFull, dstShown, allowEscape, true)
	if err != nil {
		return err
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", srcShown, err)
	}
	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", ErrCopySourceNotRegular, srcShown)
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", srcShown, err)
	}
	defer in.Close()
	if openedInfo, err := in.Stat(); err != nil || !os.SameFile(srcInfo, openedInfo) || !openedInfo.Mode().IsRegular() {
		return fmt.Errorf("%w: %s changed while opening", ErrCopySourceNotRegular, srcShown)
	}
	if dstInfo, err := os.Stat(dst); err == nil {
		if dstInfo.IsDir() {
			return fmt.Errorf("%w: %s", ErrCopyDestIsDirectory, dstShown)
		}
		if os.SameFile(srcInfo, dstInfo) {
			return fmt.Errorf("%w: %s and %s", ErrCopySameFile, srcShown, dstShown)
		}
	}
	// The destination is resolved by real path (links followed, as os.Create did): an in-root link is written THROUGH, to its real target,
	// which confineReal has just shown to lie inside the root (unless the per-root opt-out is on). The temp file lives next to that real
	// target, so the rename never crosses a directory the check did not see.
	realDst, err := realpathLax(dst)
	if err != nil {
		return err
	}
	realDir := filepath.Dir(realDst)
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", realDir, err)
	}
	tmp, err := os.CreateTemp(realDir, ".copy-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dstShown, err)
	}
	tmpName := tmp.Name()
	if copyTempHook != nil {
		copyTempHook(tmpName)
	}
	fail := func(e error) error {
		tmp.Close()
		os.Remove(tmpName)
		return e
	}
	if err := tmp.Chmod(srcInfo.Mode().Perm()); err != nil {
		return fail(fmt.Errorf("failed to set destination mode %s: %w", dstShown, err))
	}
	buf := make([]byte, copyChunk)
	for {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, werr := tmp.Write(buf[:n]); werr != nil {
				return fail(fmt.Errorf("failed to copy file from %s to %s: %w", srcShown, dstShown, werr))
			}
		}
		if copyChunkHook != nil {
			copyChunkHook()
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return fail(fmt.Errorf("failed to copy file from %s to %s: %w", srcShown, dstShown, rerr))
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to copy file from %s to %s: %w", srcShown, dstShown, err)
	}
	if err := os.Rename(tmpName, realDst); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to copy file from %s to %s: %w", srcShown, dstShown, err)
	}
	return nil
}
