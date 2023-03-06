package main

import (
	"bytes"
	//"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) <= 1 {
		usage()
		return
	}

	switch os.Args[1] {
	case "lint":
		lintMain()
	default:
		usage()
	}
}

func usage() {
	fmt.Println("Usage: go run scripts/build.go CMD, supported commands: lint")
	os.Exit(1)
}

func lintMain() {
	roots := findGoModuleRoots()
	failed := false
	for _, m := range roots {
		var stderr, stdout bytes.Buffer
		cmd := exec.Command("golangci-lint", "run")
		cmd.Dir = m
		cmd.Stderr = &stderr
		cmd.Stdout = &stdout
		err := cmd.Run()
		if err != nil {
			fmt.Printf("cd %s && golangci-lint run\n", m)
			fmt.Println(stdout.String())
			fmt.Println(stderr.String())
			fmt.Println(err)
			fmt.Println()
			failed = true
		}
	}
	if failed {
		log.Fatalf("lint failed")
	}
}

// Finds directories containing go.mod files in the repository.
func findGoModuleRoots() (result []string) {
	var buf bytes.Buffer
	cmd := exec.Command("git", "ls-files", "-z", "**/go.mod")
	cmd.Dir = "."
	cmd.Stderr = os.Stderr
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
	ms := strings.Split(buf.String(), string(rune(0)))
	for _, m := range ms {
		if m == "" {
			continue
		}
		d := filepath.Dir(m)
		result = append(result, d)
	}
	return result
}
