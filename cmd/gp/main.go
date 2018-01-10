package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	curdir       string
	gopath       string
	linkToInside string
)

func main() {
	var err error

	gopath, err = runCmd("", "go", "env", "GOPATH")
	if gopath == "" {
		fmt.Fprintln(os.Stderr, "\"go env GOPATH\" return nothing, propably misconfiguration of GOPATH")
		fmt.Fprintln(os.Stderr, "see: https://github.com/golang/go/wiki/GOPATH")
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				fmt.Fprintln(os.Stderr)
				fmt.Fprint(os.Stderr, "Subcommand error: ")
				fmt.Fprintln(os.Stderr, exitErr.Error())
				os.Stderr.Write(exitErr.Stderr)
			}
		}
		os.Exit(1)
	}

	if len(os.Args) == 1 {
		showUsage()
	}

	curdir, err = getWd()
	if err != nil {
		panic(err)
	}

	linkToInside, _, err = pathInsideDir(curdir, gopath)
	if err != nil {
		panic(err)
	}

	switch os.Args[1] {
	case "init":
		var (
			force      bool
			targetPath string
		)
		if len(os.Args) > 2 {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.SetOutput(ioutil.Discard)
			fs.BoolVar(&force, "f", false, "")
			if err := fs.Parse(os.Args[2:]); err != nil {
				showUsage()
			}
			if fs.NArg() > 0 {
				targetPath = fs.Arg(0)
			}
		}
		gpInit(force, targetPath)
	case "deinit":
		gpDeinit()
	case "fix":
		if len(os.Args) < 3 {
			showUsage()
		}
		gpFix(os.Args[2])
	case "list-ext":
		gpListExt()
	case "status":
		gpStatus()
	default:
		showUsage()
	}
}

func showUsage() {
	fmt.Fprintln(os.Stderr, "Move project to GOPATH, based on import path comment")
	fmt.Fprintln(os.Stderr, "see: https://golang.org/cmd/go/#hdr-Import_path_checking")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "USAGE:")
	fmt.Fprintln(os.Stderr, "  gp init [-f] [TargetPath]    move the project to GOPATH and replace it with symlink")
	fmt.Fprintln(os.Stderr, "                               if TargetPath is not specified, it will")
	fmt.Fprintln(os.Stderr, "                               run \"go list -f '{{.ImportComment}}'\"")
	fmt.Fprintln(os.Stderr, "                               \"-f\" will remove what already inside GOPATH")
	fmt.Fprintln(os.Stderr, "  gp deinit                    move out project from GOPATH")
	fmt.Fprintln(os.Stderr, "  gp fix <BrokenPath>          rewrite go source code recursively, replace BrokenPath")
	fmt.Fprintln(os.Stderr, "                               with project ImportPath")
	fmt.Fprintln(os.Stderr, "  gp list-ext                  list external dependencies")
	fmt.Fprintln(os.Stderr, "  gp status                    show status")
	os.Exit(1)
}

func gpInit(force bool, targetPath string) {
	var err error
	var projectDir string

	if linkToInside != "" {
		os.Exit(0)
	}

	if targetPath == "" {
		projectDir, targetPath, err = getProject(curdir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	} else {
		projectDir = curdir
	}

	if targetPath == "" {
		fmt.Fprintln(os.Stderr, "project ImportPath is undefined")
		os.Exit(1)
	}

	targetDir := strings.Replace(targetPath, "/", string(filepath.Separator), -1)
	targetDir = filepath.Join(gopath, "src", targetDir)
	_, err = os.Stat(targetDir)
	if err == nil {
		if !force {
			fmt.Fprintln(os.Stderr, targetDir+" is already exists")
			os.Exit(1)
		} else if err := os.RemoveAll(targetDir); err != nil {
			panic(err)
		}
	} else if !os.IsNotExist(err) {
		panic(err)
	}

	stat, err := os.Lstat(projectDir)
	if err != nil {
		panic(err)
	}
	if stat.Mode()&os.ModeSymlink != 0 {
		fmt.Fprintln(os.Stderr, projectDir+" is a symlink")
		os.Exit(1)
	}

	os.Chdir(gopath)
	os.MkdirAll(filepath.Dir(targetDir), 0755)
	if err = utilMove(projectDir, targetDir); err != nil {
		panic(err)
	}
	if err = os.Symlink(targetDir, projectDir); err != nil {
		panic(err)
	}

	os.Exit(0)
}

func gpDeinit() {
	if strings.HasPrefix(curdir, gopath) {
		os.Exit(0)
	}

	if linkToInside == "" {
		os.Exit(0)
	}

	targetDir, err := followLink(linkToInside)
	if err != nil {
		panic(err)
	}

	os.Chdir(gopath)
	if err = os.Remove(linkToInside); err != nil {
		panic(err)
	}
	if err = utilMove(targetDir, linkToInside); err != nil {
		panic(err)
	}

	os.Exit(0)
}

func gpStatus() {
	if strings.HasPrefix(curdir, gopath) {
		os.Exit(0)
	}

	if linkToInside == "" {
		fmt.Println("not initialized")
		os.Exit(1)
	}

	target, err := followLink(linkToInside)
	if err != nil {
		panic(err)
	}

	fmt.Println("project: " + importPathFromDir(target))
	fmt.Println("symlink: " + linkToInside + " -> " + target)
	os.Exit(0)
}

func gpFix(brokenPath string) {
	if linkToInside == "" {
		fmt.Fprintln(os.Stderr, "not in GOPATH and project is not initialized, run \"gp init\" first")
		os.Exit(1)
	}
	targetDir, err := followLink(linkToInside)
	if err != nil {
		panic(err)
	}

	importPath := importPathFromDir(targetDir)

	printConfig := printer.Config{
		Mode:     printer.UseSpaces | printer.TabIndent,
		Tabwidth: 8,
	}

	err = filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() == "vendor" {
			return filepath.SkipDir
		}
		if !strings.HasSuffix(info.Name(), ".go") {
			return nil
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		changed := false
		for _, ispec := range f.Imports {
			i, err := strconv.Unquote(ispec.Path.Value)
			if err != nil {
				panic(err)
			}
			if i == brokenPath {
				changed = true
				ispec.Path.Value = strconv.Quote(brokenPath)
			} else if strings.HasPrefix(i, brokenPath) {
				tmp := strings.TrimPrefix(i, brokenPath)
				if strings.HasPrefix(tmp, "/") {
					changed = true
					ispec.Path.Value = strconv.Quote(importPath + tmp)
				}
			}
		}
		if changed {
			file, err := os.Create(path)
			if err != nil {
				return err
			}
			defer file.Close()
			writer := bufio.NewWriter(file)

			if err = printConfig.Fprint(writer, fset, f); err != nil {
				return err
			}

			if err = writer.Flush(); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		panic(err)
	}
	os.Exit(0)
}

func gpListExt() {
	if linkToInside == "" {
		fmt.Fprintln(os.Stderr, "not in GOPATH and project is not initialized, run \"gp init\" first")
		os.Exit(1)
	}
	targetDir, err := followLink(linkToInside)
	if err != nil {
		panic(err)
	}

	internal := map[string]bool{}
	external := map[string]bool{}

	cmd := exec.Command("go", "list", "./...")
	cmd.Dir = targetDir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Fprint(os.Stderr, "Subcommand error: ")
			fmt.Fprintln(os.Stderr, exitErr.Error())
			os.Stderr.Write(exitErr.Stderr)
		}
		panic(err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		text := scanner.Text()
		text2 := sanitizeImportPath(text)
		if text == text2 {
			internal[text] = true
		}
	}
	if scanner.Err() != nil {
		panic(err)
	}

	cmd = exec.Command("go", "list", "-f", `{{range .Deps}}{{.|printf "%s\n"}}{{end}}`, "./...")
	cmd.Dir = targetDir
	out, err = cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Fprint(os.Stderr, "Subcommand error: ")
			fmt.Fprintln(os.Stderr, exitErr.Error())
			os.Stderr.Write(exitErr.Stderr)
		}
		panic(err)
	}
	scanner = bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		text := scanner.Text()
		text2 := sanitizeImportPath(text)
		if text == text2 && !internal[text] {
			external[text] = true
		}
	}
	if scanner.Err() != nil {
		panic(err)
	}

	for k, v := range external {
		if v {
			fmt.Println(k)
		}
	}

	os.Exit(0)
}

func getProject(dir string) (string, string, error) {
	projectDir := dir
	projectPath, _ := runCmd(projectDir, "go", "list", "-f", "{{.ImportComment}}")
	if strings.Contains("/"+projectPath, "/vendor/") {
		return "", "", fmt.Errorf("ImportPath cannot contains \"vendor\"")
	}
	return projectDir, projectPath, nil
}

func importPathFromDir(target string) string {
	if !strings.HasPrefix(target, gopath) {
		return ""
	}
	target = strings.TrimPrefix(target, filepath.Join(gopath, "src"))
	target = strings.Replace(target, string(filepath.Separator), "/", -1)
	return sanitizeImportPath(target)
}

func sanitizeImportPath(target string) string {
	if !strings.HasPrefix(target, "/") {
		target = "/" + target
	}
	for {
		idx := strings.Index(target, "/vendor/")
		if idx == -1 {
			break
		}
		target = target[idx+8:]
	}
	target = strings.TrimPrefix(target, "/")
	return target
}
