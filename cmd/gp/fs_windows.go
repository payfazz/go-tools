package main

import (
	"os"
)

func utilMove(src, dest string) error {
	// TODO: handle cross filesystem
	return os.Rename(src, dest)
}
