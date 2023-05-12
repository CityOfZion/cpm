package main

import "strings"

func EnsureSuffix(path string) string {
	if strings.HasSuffix(path, "/") {
		return path
	} else {
		return path + "/"
	}
}
