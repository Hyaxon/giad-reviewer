package repo

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"unicode/utf8"
)

// ReadBaseFile uses commit objects, never the head checkout or symlink targets.
func (c *Checkout) ReadBaseFile(ctx context.Context, path string) (string, bool, error) {
	if !commitSHA.MatchString(c.BaseSHA) || !fs.ValidPath(path) || strings.ContainsAny(path, "\\\x00\n\r") {
		return "", false, errors.New("invalid base revision or instruction path")
	}
	tree, err := runGit(ctx, c.Path, nil, "--literal-pathspecs", "ls-tree", c.BaseSHA, "--", path)
	if err != nil {
		return "", false, err
	}
	if tree == "" {
		return "", false, nil
	}
	entry, _, ok := strings.Cut(tree, "\t")
	parts := strings.Fields(entry)
	if !ok || len(parts) != 3 || (parts[0] != "100644" && parts[0] != "100755") || parts[1] != "blob" || !commitSHA.MatchString(parts[2]) {
		return "", false, errors.New("base instruction must be a regular Git blob")
	}
	text, err := runGitBounded(ctx, c.Path, nil, 32*1024, "cat-file", "blob", parts[2])
	if err != nil {
		return "", false, err
	}
	if !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return "", false, errors.New("base instruction is not UTF-8 text")
	}
	return text, true, nil
}
