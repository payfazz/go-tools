package main

func utilMove(src, dest string) error {
	_, err := runCmd("", "mv", src, dest)
	return err
}
