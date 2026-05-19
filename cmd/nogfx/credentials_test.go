package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeCreds(t *testing.T, dir, host, content string, mode os.FileMode) string {
	t.Helper()

	authDir := filepath.Join(dir, credentialsDir)
	require.NoError(t, os.MkdirAll(authDir, 0o700))
	path := filepath.Join(authDir, host+".env")
	require.NoError(t, os.WriteFile(path, []byte(content), mode))

	return path
}

func TestLoadCredentials_ReadsKeyValueFile(t *testing.T) {
	dir := t.TempDir()
	writeCreds(t, dir, "example.com", "user=testuser\npass=testpass\n", 0o600)

	creds, err := loadCredentials(dir, "example.com")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"user": "testuser", "pass": "testpass"}, creds)
}

func TestLoadCredentials_IgnoresCommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	writeCreds(t, dir, "example.com", "# header\n\nuser=testuser\n  # indented comment\npass=testpass\n", 0o600)

	creds, err := loadCredentials(dir, "example.com")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"user": "testuser", "pass": "testpass"}, creds)
}

func TestLoadCredentials_TrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	writeCreds(t, dir, "example.com", "  user = testuser  \n\tpass\t=\ttestpass\n", 0o600)

	creds, err := loadCredentials(dir, "example.com")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"user": "testuser", "pass": "testpass"}, creds)
}

func TestLoadCredentials_ReturnsNilWhenMissing(t *testing.T) {
	dir := t.TempDir()
	creds, err := loadCredentials(dir, "nope.example")
	require.NoError(t, err)
	assert.Nil(t, creds)
}

func TestLoadCredentials_RejectsMalformedLine(t *testing.T) {
	dir := t.TempDir()
	writeCreds(t, dir, "example.com", "user testuser\n", 0o600)

	_, err := loadCredentials(dir, "example.com")
	assert.ErrorContains(t, err, "malformed line")
}

func TestLoadCredentials_RejectsEmptyKey(t *testing.T) {
	dir := t.TempDir()
	writeCreds(t, dir, "example.com", "=value\n", 0o600)

	_, err := loadCredentials(dir, "example.com")
	assert.ErrorContains(t, err, "empty key")
}
