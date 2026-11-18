//go:build !windows

package bundle

import "os"

func privateFile(file *os.File) error   { return file.Chmod(0600) }
func replaceFile(from, to string) error { return os.Rename(from, to) }
