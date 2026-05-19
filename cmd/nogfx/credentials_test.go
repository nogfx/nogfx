package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/processors/generic"
)

func writeCreds(t *testing.T, dir, host, content string, mode os.FileMode) string {
	t.Helper()

	authDir := filepath.Join(dir, credentialsDir)
	require.NoError(t, os.MkdirAll(authDir, 0o700))
	path := filepath.Join(authDir, host+".env")
	require.NoError(t, os.WriteFile(path, []byte(content), mode))

	return path
}

func TestLoadCredentials_ReadsLineFormat(t *testing.T) {
	dir := t.TempDir()
	writeCreds(t, dir, "example.com", "testuser testpass\n", 0o600)

	creds, err := loadCredentials(dir, "example.com")
	require.NoError(t, err)
	assert.Equal(t, []generic.Credential{{Name: "testuser", Password: "testpass"}}, creds)
}

func TestLoadCredentials_PreservesOrderAcrossMultipleEntries(t *testing.T) {
	dir := t.TempDir()
	writeCreds(t, dir, "example.com", "first firstpass\nsecond secondpass\nthird thirdpass\n", 0o600)

	creds, err := loadCredentials(dir, "example.com")
	require.NoError(t, err)
	assert.Equal(t, []generic.Credential{
		{Name: "first", Password: "firstpass"},
		{Name: "second", Password: "secondpass"},
		{Name: "third", Password: "thirdpass"},
	}, creds)
}

func TestLoadCredentials_IgnoresCommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	writeCreds(t, dir, "example.com", "# header\n\ntestuser testpass\n  # indented comment\n", 0o600)

	creds, err := loadCredentials(dir, "example.com")
	require.NoError(t, err)
	assert.Equal(t, []generic.Credential{{Name: "testuser", Password: "testpass"}}, creds)
}

func TestLoadCredentials_TrimsSurroundingWhitespace(t *testing.T) {
	dir := t.TempDir()
	writeCreds(t, dir, "example.com", "  testuser\ttestpass  \n", 0o600)

	creds, err := loadCredentials(dir, "example.com")
	require.NoError(t, err)
	assert.Equal(t, []generic.Credential{{Name: "testuser", Password: "testpass"}}, creds)
}

func TestLoadCredentials_PreservesInternalPasswordWhitespace(t *testing.T) {
	dir := t.TempDir()
	writeCreds(t, dir, "example.com", "testuser pass with spaces\n", 0o600)

	creds, err := loadCredentials(dir, "example.com")
	require.NoError(t, err)
	assert.Equal(t, []generic.Credential{{Name: "testuser", Password: "pass with spaces"}}, creds)
}

func TestLoadCredentials_ReturnsNilWhenMissing(t *testing.T) {
	dir := t.TempDir()
	creds, err := loadCredentials(dir, "nope.example")
	require.NoError(t, err)
	assert.Nil(t, creds)
}

func TestLoadCredentials_RejectsLineMissingPassword(t *testing.T) {
	dir := t.TempDir()
	writeCreds(t, dir, "example.com", "testuser\n", 0o600)

	_, err := loadCredentials(dir, "example.com")
	assert.ErrorContains(t, err, "malformed line")
}

func TestLoadCredentials_TrailingWhitespaceOnNameIsMalformed(t *testing.T) {
	dir := t.TempDir()
	writeCreds(t, dir, "example.com", "testuser   \t  \n", 0o600)

	_, err := loadCredentials(dir, "example.com")
	assert.ErrorContains(t, err, "malformed line")
}
