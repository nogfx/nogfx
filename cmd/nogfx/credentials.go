package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/nogfx/nogfx/processors/generic"
)

// credentialsDir is the subdirectory under the per-user nogfx root where
// per-host credentials files live. The path is intentionally separate
// from logs/ to make it easy to exclude from backups or sync.
const credentialsDir = "auth"

// loadCredentials reads a per-host credentials file from
// $directory/auth/<host>.env, returning a nil slice if the file does
// not exist (AutoLogin then becomes a pass-through).
//
// Format (one character per line):
//
//	# comments and blank lines are ignored
//	name password
//	othername otherpassword
//
// The name is the first whitespace-delimited token; the password is the
// remainder of the line with surrounding whitespace trimmed. Order is
// preserved — the first line is the credential the GMCP auto-login
// currently uses. Multiple lines are accepted now so files can be
// prepared ahead of the future per-character selection flow.
//
// The file's permissions are inspected — if other users have read or
// write access, a warning is logged but loading continues. This protects
// against accidentally world-readable files without forcing a hard fail
// the user can't easily diagnose.
func loadCredentials(dir, host string) ([]generic.Credential, error) {
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

	var creds []generic.Credential

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		sep := strings.IndexFunc(line, unicode.IsSpace)
		if sep < 0 {
			return nil, fmt.Errorf("credentials %q: malformed line %q (expected \"name password\")", path, line)
		}

		creds = append(creds, generic.Credential{
			Name:     line[:sep],
			Password: strings.TrimSpace(line[sep+1:]),
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read credentials %q: %w", path, err)
	}

	return creds, nil
}
