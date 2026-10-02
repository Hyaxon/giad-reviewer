// Package instructions resolves scoped AGENTS.md guidance from the PR base revision.
package instructions

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/hyaxon/agentic-review/pkg/protocol"
)

type BaseReader interface {
	ReadBaseFile(context.Context, string) (string, bool, error)
}

// Resolve returns root-to-leaf guidance for changed and previous (renamed) paths.
// Scope is explicit: a nested instruction does not apply to unrelated changed files.
func Resolve(ctx context.Context, reader BaseReader, baseSHA string, files []protocol.ChangedFile) ([]protocol.Instruction, error) {
	paths := map[string]bool{"AGENTS.md": true}
	for _, file := range files {
		for _, name := range []string{file.Path, file.PreviousPath} {
			if name == "" {
				continue
			}
			if !fs.ValidPath(name) || strings.ContainsAny(name, "\\\x00\r\n") {
				return nil, errors.New("invalid changed file path")
			}
			for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
				paths[dir+"/AGENTS.md"] = true
			}
		}
	}
	names := make([]string, 0, len(paths))
	for name := range paths {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		a, b := strings.Count(names[i], "/"), strings.Count(names[j], "/")
		if a != b {
			return a < b
		}
		return names[i] < names[j]
	})
	result := []protocol.Instruction{}
	total := 0
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		text, found, err := reader.ReadBaseFile(ctx, name)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		total += len(text)
		if total > 64*1024 || len(result) >= 64 {
			return nil, errors.New("trusted instructions exceed 64 KiB or 64 files")
		}
		result = append(result, protocol.Instruction{Path: name, Scope: path.Dir(name), BaseSHA: baseSHA, Content: text})
	}
	return result, nil
}
