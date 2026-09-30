//go:build !unix

package main

import "errors"

func execve(string, []string, []string) error {
	return errors.New("exec is not supported on this platform")
}
