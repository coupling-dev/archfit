package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/coupling-dev/archfit/internal/toolrun"
)

func ignoreInitCache(ctx context.Context, runner toolrun.Runner, root string) error {
	const entry = cacheDirName + "/"
	path := filepath.Join(root, ".gitignore")
	mode := os.FileMode(0o644)
	info, err := os.Lstat(path)
	var data []byte
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return err
	case !info.Mode().IsRegular():
		return fmt.Errorf("%s is not a regular file", path)
	default:
		mode = info.Mode().Perm()
		data, err = os.ReadFile(path) //nolint:gosec // the repository's ignore file
		if err != nil {
			return err
		}
	}
	for line := range bytes.Lines(data) {
		if string(bytes.TrimRight(line, "\r\n")) != entry {
			continue
		}
		out, err := runner.Run(ctx, toolrun.ToolCmd{
			Name: gitBinary, Args: []string{"check-ignore", "--no-index", "--", entry}, WorkDir: root,
		})
		if err != nil {
			return err
		}
		switch out.ExitCode {
		case 0:
			return nil
		case 1: // A later rule overrides the existing entry.
		default:
			return fmt.Errorf("git check-ignore exited %d", out.ExitCode)
		}
		break
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	return writeFileAtomic(path, append(data, entry+"\n"...), mode)
}
