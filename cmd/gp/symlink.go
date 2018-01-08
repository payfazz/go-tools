package main

import (
	"os"
	"path/filepath"
	"strings"
)

func followLink(current string) (string, error) {
	var err error
	current, err = filepath.Abs(current)
	if err != nil {
		return "", err
	}
	for {
		stat, err := os.Lstat(current)
		if err != nil {
			return "", err
		}

		if stat.Mode()&os.ModeSymlink == 0 {
			return current, nil
		}

		target, err := os.Readlink(current)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(current), target)
		}
		current = target
	}
}

func pathInsideDir(current, target string) (string, string, error) {
	var err error
	subdir := "."
	for {
		prev := current
		current, err = followLink(current)
		if err != nil {
			return "", "", err
		}
		if current != prev && strings.HasPrefix(prev, current) {
			current = prev
		}

		if strings.HasPrefix(current, target) {
			return prev, filepath.Join(current, subdir), nil
		}

		prev = current
		current = filepath.Join(current, "..")
		if current == prev {
			return "", "", nil
		}
		subdir = filepath.Join(filepath.Base(prev), subdir)
	}
}
