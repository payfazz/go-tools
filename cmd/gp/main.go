package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"os"
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

	if len(os.Args) == 1 {
		showUsage()
	}

	gopath, _ = runCmd("", "go", "env", "GOPATH")
	if gopath == "" {
		fmt.Fprintln(os.Stderr, "\"go env GOPATH\" return nothing, propably misconfiguration of GOPATH")
		fmt.Fprintln(os.Stderr, "see: https://github.com/golang/go/wiki/GOPATH")
		os.Exit(1)
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
		var force bool
		var importPath string
		if len(os.Args) > 2 {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.SetOutput(ioutil.Discard)
			fs.BoolVar(&force, "f", false, "")
			if err := fs.Parse(os.Args[2:]); err != nil {
				showUsage()
			}
			if fs.NArg() > 0 {
				importPath = fs.Arg(0)
			}
		}
		gpInit(force, importPath)
	case "deinit":
		gpDeinit()
	case "fix":
		if len(os.Args) <= 2 {
			showUsage()
		}
		gpFix(os.Args[2])
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
	fmt.Fprintln(os.Stderr, "  gp init [-f] [ImportPath]    move the project to GOPATH and replace it with symlink")
	fmt.Fprintln(os.Stderr, "                               if ImportPath is not specified, it will")
	fmt.Fprintln(os.Stderr, "                               run \"go list -f '{{.ImportComment}}'\"")
	fmt.Fprintln(os.Stderr, "                               \"-f\" will remove what already inside GOPATH")
	fmt.Fprintln(os.Stderr, "  gp deinit                    move out project from GOPATH")
	fmt.Fprintln(os.Stderr, "  gp fix <OldPath>             replace OldPath with project ImportPath recursively")
	fmt.Fprintln(os.Stderr, "  gp status                    show status")
	os.Exit(1)
}

func gpInit(force bool, importPath string) {
	var projectPath string
	var projectDir string

	if linkToInside != "" {
		os.Exit(0)
	}

	if importPath == "" {
		projectDir, importPath = getProject(curdir)
	} else {
		projectDir = curdir
	}

	if importPath == "" {
		fmt.Fprintln(os.Stderr, "project ImportPath is undefined")
		os.Exit(1)
	}

	projectPath = strings.Replace(importPath, "/", string(filepath.Separator), -1)
	targetDir := filepath.Join(gopath, "src", projectPath)
	_, err := os.Stat(targetDir)
	if err == nil {
		if !force {
			fmt.Fprintln(os.Stderr, targetDir+" is already exists")
			os.Exit(1)
		} else {
			if err = os.RemoveAll(targetDir); err != nil {
				panic(err)
			}
		}
	}
	if err != nil && !os.IsNotExist(err) {
		panic(err)
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

	fmt.Println("active: " + importPathFromDir(target))
	os.Exit(0)
}

func gpFix(oldPath string) {
	if linkToInside == "" {
		fmt.Fprintln(os.Stderr, "not in GOPATH and project is not initialized, run \"gp init\" first")
		os.Exit(1)
	}
	target, err := followLink(linkToInside)
	if err != nil {
		panic(err)
	}

	importPath := importPathFromDir(target)

	var printConfig = printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}

	filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
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
			if strings.HasPrefix(i, oldPath) {
				changed = true
				i = importPath + strings.TrimPrefix(i, oldPath)
				ispec.Path.Value = strconv.Quote(i)
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
	os.Exit(0)
}

func getProject(dir string) (string, string) {
	projectDir := dir
	projectPath, _ := runCmd(projectDir, "go", "list", "-f", "{{.ImportComment}}")
	return projectDir, projectPath
}

func importPathFromDir(target string) string {
	if !strings.HasPrefix(target, gopath) {
		return ""
	}
	target = strings.TrimPrefix(target, filepath.Join(gopath, "src"))
	target = strings.Replace(target, string(filepath.Separator), "/", -1)
	target = strings.TrimPrefix(target, "/")
	for {
		idx := strings.Index(target, "vendor/")
		if idx == -1 {
			break
		}
		target = target[idx+7:]
	}
	return target
}
