// go-sort 按声明规则排序 Go 源文件。
package main

import (
	"errors"
	"flag"
	"log"
	"os"
)

//go:generate go mod tidy
//go:generate go install -v -trimpath -ldflags "-s -w" .
func main() {
	logger := log.New(os.Stderr, "", 0)
	cfg, parseErr := parseFlags(os.Args[1:], os.Stderr)
	if errors.Is(parseErr, flag.ErrHelp) {
		return
	}
	if parseErr != nil {
		logger.Fatal(parseErr)
	}
	if sortErr := sortFile(cfg); sortErr != nil {
		logger.Fatal(sortErr)
	}
}
