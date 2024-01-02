package main

import (
	"bytes"
	"fmt"
	"io"
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
	case "update-pulumi-deps":
		updatePulumiDeps()
	case "latest-pulumi-version":
		fmt.Println(latestPulumiVersion())
	default:
		usage()
	}
}

func usage() {
	fmt.Println("Usage: go run scripts/build.go CMD, supported commands: lint, update-pulumi-deps")
	os.Exit(1)
}

func updatePulumiDeps() {
	ver := os.Args[2]
	ver = strings.TrimPrefix(ver, "v")
	roots := findGoModuleRoots()
	for _, m := range roots {
		edited := false
		if fileContains(filepath.Join(m, "go.mod"), "github.com/pulumi/pulumi/pkg/v3") {
			execCommandOrLogFatal(m, "go", "mod", "edit", "-droprequire", "github.com/pulumi/pulumi/pkg/v3")
			execCommandOrLogFatal(m, "go", "mod", "edit", "-require", "github.com/pulumi/pulumi/pkg/v3@v"+ver)
			edited = true
		}
		if fileContains(filepath.Join(m, "go.mod"), "github.com/pulumi/pulumi/sdk/v3") {
			execCommandOrLogFatal(m, "go", "mod", "edit", "-droprequire", "github.com/pulumi/pulumi/sdk/v3")
			execCommandOrLogFatal(m, "go", "mod", "edit", "-require", "github.com/pulumi/pulumi/sdk/v3@v"+ver)
			edited = true
		}
		if edited {
			execCommandOrLogFatal(m, "go", "mod", "tidy")
		}
	}
}

func fileContains(path string, search string) bool {
	b, _ := os.ReadFile(path)
	return bytes.Contains(b, []byte(search))
}

func lintMain() {
	roots := findGoModuleRoots()
	failed := false
	for _, m := range roots {
		fmt.Printf("%q: linting ...", m)
		err := execCommand(m, "golangci-lint", "run")
		if err == nil {
			fmt.Printf(" done\n")
		} else {
			fmt.Printf(" failed:\n")
			err.(*execError).MustWrite(os.Stderr)
			failed = true
		}
	}
	if failed {
		log.Fatalf("lint failed")
	}
}

type execError struct {
	err error
	cmd *exec.Cmd
}

func (e *execError) Error() string { return fmt.Sprintf("%s: %s", e.cmd, e.err.Error()) }

func (e *execError) Unwrap() error { return e.err }

func (e *execError) Write(sink io.Writer) (err error) {
	w := func(s string, a ...any) {
		if err != nil {
			return
		}
		_, err = fmt.Fprintf(sink, s, a...)
	}
	w("cd %s && %s %s\n", e.cmd.Dir, e.cmd.Path, strings.Join(e.cmd.Args, " "))
	w("%s\n", e.cmd.Stdout.(*bytes.Buffer).String())
	w("%s\n", e.cmd.Stderr.(*bytes.Buffer).String())
	w("%s\n", e.err.Error())
	w("\n")

	return
}

func (e *execError) MustWrite(sink io.Writer) {
	err := e.Write(sink)
	if err != nil {
		panic(err)
	}
}

func execCommand(cwd, name string, arg ...string) error {
	var stderr, stdout bytes.Buffer
	cmd := exec.Command(name, arg...)
	cmd.Dir = cwd
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return &execError{err, cmd}
	}
	return nil
}

func execCommandOrLogFatal(cwd, name string, arg ...string) {
	err := execCommand(cwd, name, arg...)
	if err != nil {
		if err, ok := err.(*execError); ok {
			err.MustWrite(os.Stderr)
		}
		log.Fatal(err)
	}
}

// Finds directories containing go.mod files in the repository.
func findGoModuleRoots() (result []string) {
	var buf bytes.Buffer
	cmd := exec.Command("git", "ls-files", "-z", "**go.mod")
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

func latestPulumiVersion() string {
	d, err := os.MkdirTemp("", "version-extractor")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(d); err != nil {
			log.Fatal(err)
		}
	}()

	{
		cmd := exec.Command("go", "mod", "init", "github.com/pulumi/version-extractor")
		cmd.Dir = d
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	}

	{
		cmd := exec.Command("go", "get", "-u", "github.com/pulumi/pulumi/pkg/v3")
		cmd.Dir = d
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	}

	var stdout bytes.Buffer

	{
		cmd := exec.Command("go", "list", "-f", "{{.Version}}", "-m", "github.com/pulumi/pulumi/pkg/v3")
		cmd.Dir = d
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	}

	return strings.TrimSpace(stdout.String())
}
