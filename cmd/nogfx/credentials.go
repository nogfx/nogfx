package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// credentialsDir is the subdirectory under the per-user nogfx root where
// per-host credentials files live. The path is intentionally separate
// from logs/ to make it easy to exclude from backups or sync.
const credentialsDir = "auth"

// loadCredentials reads a key=value credentials file for the given host
// from $directory/auth/<host>.env, returning a nil map if the file does
// not exist (the world's AutoLogin then becomes a pass-through).
//
// Format (one entry per line):
//
//	# comments and blank lines are ignored
//	user = …
//	pass = …
//
// Whitespace around the key and value is trimmed; values are not quoted.
// The file's permissions are inspected — if other users have read or
// write access, a warning is logged but loading continues. This protects
// against accidentally world-readable files without forcing a hard fail
// the user can't easily diagnose.
func loadCredentials(dir, host string) (map[string]string, error) {
	path := filepath.Join(dir, credentialsDir, host+".env")

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stat credentials %q: %w", path, err)
	}

	if mode := info.Mode().Perm(); mode&0o077 != 0 {
		log.Printf("credentials file %q is mode %#o; recommend chmod 600", path, mode)
	}

	// #nosec G304 -- path is built from caller-supplied dir and a host
	// that the composition root has already validated through
	// parseAddress; no traversal risk.
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open credentials %q: %w", path, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			log.Printf("close credentials %q: %v", path, cerr)
		}
	}()

	creds := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			return nil, fmt.Errorf("credentials %q: malformed line %q (missing =)", path, line)
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if key == "" {
			return nil, fmt.Errorf("credentials %q: empty key in line %q", path, line)
		}
		creds[key] = val
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read credentials %q: %w", path, err)
	}
	return creds, nil
}
