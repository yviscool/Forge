//go:build !windows

package main

import "syscall"

func hardenFileCreation() {
	// Every runtime-created database, WAL file, test point, and bootstrap secret
	// is private to the service account by default.
	syscall.Umask(0o077)
}
