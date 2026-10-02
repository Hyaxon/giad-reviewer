package tools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"unicode/utf8"
)

const MaxOutputBytes = 64 * 1024
const MaxFileBytes = 1024 * 1024
const MaxSearchEntries = 10000
const MaxMatches = 100

// Result reports incomplete output explicitly so reviewers cannot mistake it for full coverage.
type Result struct {
	Text         string
	Truncated    bool
	SkippedFiles int
}

// Repository exposes read-only operations inside an owned checkout.
// It is a filesystem boundary, not an execution sandbox.
type Repository struct {
	root *os.Root
	diff string
}

func Open(directory, diff string) (*Repository, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	return &Repository{root: root, diff: diff}, nil
}
func (r *Repository) Close() error { return r.root.Close() }

func (r *Repository) read(name string) ([]byte, error) {
	if !fs.ValidPath(name) || strings.Contains(name, "\\") {
		return nil, fmt.Errorf("invalid repository path %q", name)
	}
	parts := strings.Split(name, "/")
	for i, part := range parts {
		if strings.EqualFold(part, ".git") {
			return nil, errors.New("Git metadata is unavailable")
		}
		info, err := r.root.Lstat(strings.Join(parts[:i+1], "/"))
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("symbolic links are unavailable")
		}
		if i == len(parts)-1 && !info.Mode().IsRegular() {
			return nil, errors.New("only regular files can be read")
		}
	}
	f, err := r.root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("only regular files can be read")
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFileBytes {
		return nil, errors.New("file exceeds 1 MiB read limit")
	}
	if !utf8.Valid(data) || strings.IndexByte(string(data), 0) >= 0 {
		return nil, errors.New("file is not UTF-8 text")
	}
	return data, nil
}

func bounded(text string) Result {
	if len(text) <= MaxOutputBytes {
		return Result{Text: text}
	}
	text = text[:MaxOutputBytes]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return Result{Text: text, Truncated: true}
}

// ReadLines uses inclusive, one-based line numbers. End zero means the end of the file.
func (r *Repository) ReadLines(ctx context.Context, name string, start, end int) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if start < 1 || end < 0 || (end != 0 && end < start) {
		return Result{}, errors.New("invalid line range")
	}
	data, err := r.read(name)
	if err != nil {
		return Result{}, err
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(data) == 0 {
		lines = nil
	}
	if end == 0 || end > len(lines) {
		end = len(lines)
	}
	var out strings.Builder
	for i := start; i <= end; i++ {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		fmt.Fprintf(&out, "%d: %s\n", i, lines[i-1])
		if out.Len() > MaxOutputBytes {
			return bounded(out.String()), nil
		}
	}
	return bounded(out.String()), nil
}

// Diff returns the fetched GitHub diff, with the same output budget as file reads.
func (r *Repository) Diff() Result { return bounded(r.diff) }

// Search finds literal, case-sensitive text. Symlinks and Git metadata are excluded.
func (r *Repository) Search(ctx context.Context, query string) (Result, error) {
	if query == "" || len(query) > 1024 || strings.ContainsAny(query, "\r\n") {
		return Result{}, errors.New("search requires 1–1024 bytes of single-line text")
	}
	var out strings.Builder
	result := Result{}
	entries, matches := 0, 0
	stop := errors.New("search budget reached")
	err := fs.WalkDir(r.root.FS(), ".", func(name string, d fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries++
		if entries > MaxSearchEntries {
			result.Truncated = true
			return stop
		}
		if walkErr != nil {
			return walkErr
		}
		if strings.EqualFold(d.Name(), ".git") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		data, err := r.read(name)
		if err != nil {
			result.SkippedFiles++
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			if !strings.Contains(line, query) {
				continue
			}
			if matches == MaxMatches {
				result.Truncated = true
				return stop
			}
			fmt.Fprintf(&out, "%s:%d:%s\n", name, i+1, line)
			matches++
			if out.Len() > MaxOutputBytes {
				result.Truncated = true
				return stop
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, stop) {
		return Result{}, err
	}
	limited := bounded(out.String())
	result.Text = limited.Text
	result.Truncated = result.Truncated || limited.Truncated
	return result, nil
}
