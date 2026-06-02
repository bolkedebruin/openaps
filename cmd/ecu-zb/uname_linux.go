//go:build linux

package main

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func unameString() string {
	var u unix.Utsname
	if err := unix.Uname(&u); err != nil {
		return ""
	}
	return fmt.Sprintf("%s %s %s",
		cstr(u.Sysname[:]), cstr(u.Release[:]), cstr(u.Machine[:]))
}

func cstr(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
