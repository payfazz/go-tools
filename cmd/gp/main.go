package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "gp is no longer available")
	os.Exit(1)
}
