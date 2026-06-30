//go:build !windows

package main

import "os"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		runServerMode()
	} else {
		runLegacyMode()
	}
}
