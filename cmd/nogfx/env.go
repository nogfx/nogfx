package main

import (
	"log"
	"os"
	"path/filepath"
)

// Version is the application version, set by linker flags at build time.
//
//nolint:gochecknoglobals // ldflag injection requires a package-level var.
var Version = "0.0.0"

// directory is the root for all nogfx user files (logs, future config).
// Initialised once at startup.
var directory string

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed acquiring home directory: %s", err)
	}

	dir := filepath.Join(home, "nogfx")

	if err := os.MkdirAll(dir, 0o750); err != nil {
		log.Fatalf("failed creating directory %q: %s", dir, err)
	}

	directory = dir
}
