package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runCmd(cwd string, args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(output), "\r\n"), nil
}

func findRootOf(dir, what string) (string, error) {
	for {
		_, err := os.Stat(filepath.Join(dir, what))
		if err == nil {
			return dir, nil
		}
		if os.IsNotExist(err) {
			prev := dir
			dir = filepath.Join(dir, "..")
			if prev == dir {
				return "", nil
			}
		} else {
			return "", err
		}
	}
}

func getWd() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Abs(wd)
}
