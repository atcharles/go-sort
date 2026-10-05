package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

type fileOptions struct {
	includeTests bool
	recursive    bool
}

func getDirGoFiles(dir string, options fileOptions) ([]string, error) {
	if dir == "./..." || dir == "./" || dir == "." || dir == "" {
		dir = "."
	}
	dir = filepath.Clean(dir)
	var files []string
	walkFn := func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "vendor":
				return filepath.SkipDir
			default:
			}
			if !options.recursive && path != dir {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if !options.includeTests && strings.HasSuffix(path, "_test.go") {
			return nil
		}
		absolute, absErr := filepath.Abs(path)
		if absErr != nil {
			return fmt.Errorf("resolve path %s: %w", path, absErr)
		}
		files = append(files, absolute)
		return nil
	}
	if walkErr := filepath.Walk(dir, walkFn); walkErr != nil {
		return nil, fmt.Errorf("walk %s: %w", dir, walkErr)
	}
	return files, nil
}

func sortFile(cfg config) error {
	path, pathErr := resolveInputPath(cfg.path)
	if pathErr != nil {
		return pathErr
	}
	files, walkErr := getDirGoFiles(path, fileOptions{includeTests: cfg.includeTests, recursive: cfg.recursive})
	if walkErr != nil {
		return walkErr
	}
	for _, filename := range files {
		if _, sortErr := sortActionByFilename(filename, cfg.write); sortErr != nil {
			return fmt.Errorf("sort file %s: %w", filename, sortErr)
		}
	}
	return nil
}
