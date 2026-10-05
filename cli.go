package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

type config struct {
	path         string
	recursive    bool
	includeTests bool
	write        bool
}

func parseFlags(args []string, output io.Writer) (config, error) {
	cfg := config{path: "."}
	flags := flag.NewFlagSet("go-sort", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.BoolVar(&cfg.recursive, "r", true, "recurse into subdirectories")
	flags.BoolVar(&cfg.includeTests, "tests", false, "include *_test.go files")
	flags.BoolVar(&cfg.write, "w", true, "write result back to file")
	if parseErr := flags.Parse(args); parseErr != nil {
		return config{}, fmt.Errorf("parse flags: %w", parseErr)
	}
	positional := flags.Args()
	if len(positional) > 0 && positional[0] == "test" {
		cfg.includeTests = true
		positional = positional[1:]
	}
	if len(positional) > 0 {
		cfg.path = positional[len(positional)-1]
	}
	return cfg, nil
}

func resolveInputPath(path string) (string, error) {
	executable, executableErr := os.Executable()
	if executableErr == nil && strings.HasSuffix(executable, path) {
		path = "."
	}
	if path == "" || path == "./..." {
		path = "."
	}
	if _, statErr := os.Stat(path); statErr != nil {
		return "", fmt.Errorf("stat %s: %w", path, statErr)
	}
	return path, nil
}
