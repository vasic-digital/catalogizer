package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

// ErrCopyDestIsDirectory is returned when the destination of a copy is an existing directory. An external "cp src dir" would write
// dir/<base(src)> through whatever entry of that name already exists (including an in-root link pointing outside the root), a destination
// the confinement check never saw (CAT-FS-SYMLINK-1 review round 2, R2-1). The copy API names the destination file, never a directory.
var ErrCopyDestIsDirectory = errors.New("copy destination is an existing directory")

// copyConfinedFile copies srcFull to dstFull inside root without any external command. Both paths are checked with confineReal (source and
// destination followed at the last component: the copy reads and writes through the link, so the link's real target must lie inside the
// root), an existing-directory destination is refused, and the destination is written with os.Create on the checked path, so no
// implicit destination is ever derived. Shared by the darwin NFSClient and its linux-runnable test. The check-to-use race is documented
// in docs/security/PATH_CONFINEMENT.md.
func copyConfinedFile(ctx context.Context, root, srcFull, dstFull, srcShown, dstShown string, allowEscape bool) error {
	src, err := confineReal(root, srcFull, srcShown, allowEscape, true)
	if err != nil {
		return err
	}
	dst, err := confineReal(root, dstFull, dstShown, allowEscape, true)
	if err != nil {
		return err
	}
	if fi, err := os.Stat(dst); err == nil && fi.IsDir() {
		return fmt.Errorf("%w: %s", ErrCopyDestIsDirectory, dstShown)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("copy failed: %w", err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("copy failed: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copy failed: %w", err)
	}
	return out.Close()
}
