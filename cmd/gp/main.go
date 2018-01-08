package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	gopath, _ := runCmd("", "go", "env", "GOPATH")
	if gopath == "" {
		fmt.Fprintln(os.Stderr, "\"go env GOPATH\" return nothing, propably misconfiguration of GOPATH")
		fmt.Fprintln(os.Stderr, "see: https://github.com/golang/go/wiki/GOPATH")
		os.Exit(1)
	}

	if len(os.Args) == 1 {
		showUsage()
	}

	if os.Args[1] == "init" {
		path := ""
		if len(os.Args) > 2 {
			path = os.Args[2]
		}
		gpInit(gopath, path)
	} else if os.Args[1] == "deinit" {
		gpDeinit(gopath)
	}

	showUsage()
}

func showUsage() {
	fmt.Fprintln(os.Stderr, "Move project to GOPATH, based on import comment")
	fmt.Fprintln(os.Stderr, "see: https://golang.org/cmd/go/#hdr-Import_path_checking")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "USAGE:")
	fmt.Fprintln(os.Stderr, "  gp init [import_path]    move project to GOPATH and replace it with symlink")
	fmt.Fprintln(os.Stderr, "  gp deinit                move project from GOPATH it to current symlink")
	os.Exit(1)
}

func gpInit(gopath, importpath string) {
	curdir, err := getWd()
	if err != nil {
		panic(err)
	}

	linkToInside, _, err := pathInsideDir(curdir, gopath)
	if err != nil {
		panic(err)
	}
	if linkToInside != "" {
		os.Exit(0)
	}

	projectPath := importpath
	projectDir := curdir
	if projectPath == "" {
		for {
			tmp, _ := runCmd(projectDir, "go", "list", "-f", "{{.ImportComment}}")
			if tmp != "" {
				projectPath = tmp
				break
			}
			prev := projectDir
			projectDir = filepath.Join(projectDir, "..")
			if projectDir == prev {
				break
			}
		}
		if projectPath == "" {
			fmt.Fprintln(os.Stderr, "no package with ImportComment found up to root directory")
			fmt.Fprintln(os.Stderr, "see: https://golang.org/cmd/go/#hdr-Import_path_checking")
			os.Exit(1)
		}
	}

	projectPath = strings.Replace(projectPath, "/", string(filepath.Separator), -1)
	targetDir := filepath.Join(gopath, "src", projectPath)
	_, err = os.Stat(targetDir)
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}
	if err == nil {
		fmt.Fprintln(os.Stderr, targetDir+" is already exists")
		os.Exit(1)
	}

	os.Chdir(filepath.Join(projectDir, ".."))

	fmt.Println("mkdir", "-p", filepath.Dir(targetDir))
	os.MkdirAll(filepath.Dir(targetDir), 0755)

	fmt.Println("mv", projectDir, targetDir)
	if err = os.Rename(projectDir, targetDir); err != nil {
		panic(err)
	}

	fmt.Println("ln", "-s", targetDir, projectDir)
	if err = os.Symlink(targetDir, projectDir); err != nil {
		panic(err)
	}

	os.Exit(0)
}

func gpDeinit(gopath string) {
	curdir, err := getWd()
	if err != nil {
		panic(err)
	}

	if strings.HasPrefix(curdir, gopath) {
		os.Exit(0)
	}

	linkToInside, _, err := pathInsideDir(curdir, gopath)
	if err != nil {
		panic(err)
	}
	if linkToInside == "" {
		os.Exit(0)
	}

	targetDir, err := followLink(linkToInside)
	if err != nil {
		panic(err)
	}

	os.Chdir(filepath.Join(linkToInside, ".."))

	fmt.Println("rm", linkToInside)
	if err = os.Remove(linkToInside); err != nil {
		panic(err)
	}

	fmt.Println("mv", targetDir, linkToInside)
	if err = os.Rename(targetDir, linkToInside); err != nil {
		panic(err)
	}

	os.Exit(0)
}
