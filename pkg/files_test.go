package pkg

import (
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestIsReadableFile(t *testing.T) {
	// Create a temporary directory for the test files
	dir := t.TempDir()

	// Create a temporary UTF-8 encoded file
	utf8File := filepath.Join(dir, "utf8.txt")
	if err := os.WriteFile(utf8File, []byte("hello, world!"), 0600); err != nil {
		t.Fatalf("failed to create UTF-8 file: %v", err)
	}

	// Create a temporary gzip-compressed UTF-8 file
	gzipFile := filepath.Join(dir, "utf8.txt.gz")
	gzipContent := []byte("hello, gzipped world!")
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	if _, err := gzipWriter.Write(gzipContent); err != nil {
		t.Fatalf("failed to write gzip content: %v", err)
	}
	gzipWriter.Close()
	if err := os.WriteFile(gzipFile, buf.Bytes(), 0600); err != nil {
		t.Fatalf("failed to create gzip file: %v", err)
	}

	// Test cases
	tests := []struct {
		filename   string
		expectErr  bool
		expectBool bool
	}{
		{utf8File, false, true},
		{gzipFile, false, true},
		{"nonexistent.txt", true, false},
	}

	for _, test := range tests {
		t.Run(test.filename, func(t *testing.T) {
			result, err := IsReadableFile(test.filename, false, nil, false)
			if (err != nil) != test.expectErr {
				t.Errorf("IsReadableFile(%q) error = %v, wantErr %v", test.filename, err, test.expectErr)
				return
			}
			if result != test.expectBool {
				t.Errorf("IsReadableFile(%q) = %v, want %v", test.filename, result, test.expectBool)
			}
		})
	}
}

func TestIsGzip(t *testing.T) {
	tests := []struct {
		name   string
		buffer []byte
		want   bool
	}{
		{"gzip header", []byte{0x1f, 0x8b}, true},
		{"not gzip header", []byte{0x00, 0x00}, false},
		{"empty buffer", []byte{}, false},
		{"partial gzip header", []byte{0x1f}, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsGzip(test.buffer); got != test.want {
				t.Errorf("IsGzip(%v) = %v, want %v", test.buffer, got, test.want)
			}
		})
	}
}

func TestFilesByPattern(t *testing.T) {
	// Create a temporary directory for the test files
	dir := t.TempDir()

	// Create some test files
	files := []string{
		filepath.Join(dir, "file1.txt"),
		filepath.Join(dir, "file2.txt"),
		filepath.Join(dir, "file3.log"),
	}
	for _, file := range files {
		if err := os.WriteFile(file, []byte("test"), 0600); err != nil {
			t.Fatalf("failed to create file %q: %v", file, err)
		}
	}

	tests := []struct {
		pattern     string
		expectErr   bool
		expectFiles []string
	}{
		{dir, false, files},
		{filepath.Join(dir, "*.txt"), false, files[:2]},
		{filepath.Join(dir, "*.log"), false, files[2:3]},
		{filepath.Join(dir, "*.none"), false, []string{}},
		{"nonexistent", false, nil},
	}

	for _, test := range tests {
		t.Run(test.pattern, func(t *testing.T) {
			result, err := FilesByPattern(test.pattern, false, nil)
			if (err != nil) != test.expectErr {
				t.Errorf("FilesByPattern(%q) error = %v, wantErr %v", test.pattern, err, test.expectErr)
				return
			}
			if len(result) != len(test.expectFiles) {
				t.Errorf("FilesByPattern(%q) = %v, want %v", test.pattern, result, test.expectFiles)
			}
		})
	}
}

func TestStringToSSHPathConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	config, err := StringToSSHPathConfig("user@example.com:2222 private_key=/keys/id_ed25519 key_passphrase=pa#ss.phrase known_hosts=/keys/known_hosts /var/log/*.log")
	if err != nil {
		t.Fatalf("StringToSSHPathConfig returned error: %v", err)
	}

	if config.User != "user" {
		t.Fatalf("User = %q, want user", config.User)
	}
	if config.Host != "example.com" {
		t.Fatalf("Host = %q, want example.com", config.Host)
	}
	if config.Port != "2222" {
		t.Fatalf("Port = %q, want 2222", config.Port)
	}
	if config.Password != "" {
		t.Fatalf("Password = %q, want empty password", config.Password)
	}
	if config.PrivateKeyPath != "/keys/id_ed25519" {
		t.Fatalf("PrivateKeyPath = %q, want /keys/id_ed25519", config.PrivateKeyPath)
	}
	if config.PrivateKeyPassphrase != "pa#ss.phrase" {
		t.Fatalf("PrivateKeyPassphrase = %q, want pa#ss.phrase", config.PrivateKeyPassphrase)
	}
	if config.KnownHostsPath != "/keys/known_hosts" {
		t.Fatalf("KnownHostsPath = %q, want /keys/known_hosts", config.KnownHostsPath)
	}
	if config.FilePath != "/var/log/*.log" {
		t.Fatalf("FilePath = %q, want /var/log/*.log", config.FilePath)
	}

	config, err = StringToSSHPathConfig("user@example.com password=secret known_hosts=/keys/known_hosts /var/log/auth.log")
	if err != nil {
		t.Fatalf("StringToSSHPathConfig returned error for password auth: %v", err)
	}
	if config.Password != "secret" {
		t.Fatalf("Password = %q, want secret", config.Password)
	}
	if config.PrivateKeyPath != "" {
		t.Fatalf("PrivateKeyPath = %q, want empty private key for password auth", config.PrivateKeyPath)
	}
	if config.FilePath != "/var/log/auth.log" {
		t.Fatalf("FilePath = %q, want /var/log/auth.log", config.FilePath)
	}

	config, err = StringToSSHPathConfig("user@example.com /var/log/app.log")
	if err != nil {
		t.Fatalf("StringToSSHPathConfig returned error: %v", err)
	}
	if config.Port != "22" {
		t.Fatalf("Port = %q, want 22", config.Port)
	}
	if config.PrivateKeyPath != filepath.Join(home, ".ssh", "id_rsa") {
		t.Fatalf("PrivateKeyPath = %q, want default id_rsa path", config.PrivateKeyPath)
	}
	if config.KnownHostsPath != filepath.Join(home, ".ssh", "known_hosts") {
		t.Fatalf("KnownHostsPath = %q, want default known_hosts path", config.KnownHostsPath)
	}

	if _, err := StringToSSHPathConfig("user@example.com password=secret private_key=/keys/id_ed25519 /var/log/*.log"); err == nil {
		t.Fatal("StringToSSHPathConfig returned nil error for mixed password and private_key auth")
	}
	if _, err := StringToSSHPathConfig("user@example.com password=secret key_passphrase=secret /var/log/*.log"); err == nil {
		t.Fatal("StringToSSHPathConfig returned nil error for key_passphrase with password auth")
	}
}

func TestNewSSHSourceIDDoesNotExposeSecrets(t *testing.T) {
	config := &SSHPathConfig{
		User:                 "user",
		Host:                 "example.com",
		Port:                 "22",
		Password:             "secret-password",
		PrivateKeyPassphrase: "secret-passphrase",
		KnownHostsPath:       "/home/gol/.ssh/known_hosts",
		FilePath:             "/var/log/*.log",
	}

	sourceID := newSSHSourceID(config, 3)
	if sourceID == "" {
		t.Fatal("newSSHSourceID returned empty source id")
	}
	if strings.Contains(sourceID, config.Password) || strings.Contains(sourceID, config.PrivateKeyPassphrase) {
		t.Fatalf("source id %q exposes secrets", sourceID)
	}
}

func TestNewSFTPClientWithRetryRetriesCreationOnly(t *testing.T) {
	calls := 0
	finalErr := errors.New("permission denied")
	_, err := newSFTPClientWithRetryFactory(
		&SSHConfig{SourceID: "ssh-test", Host: "host-a", Port: "22"},
		"test",
		func() (*sftp.Client, error) {
			calls++
			if calls == 1 {
				return nil, errors.New("ssh: rejected: connect failed (open failed)")
			}
			return nil, finalErr
		},
	)

	if !errors.Is(err, finalErr) {
		t.Fatalf("newSFTPClientWithRetryFactory error = %v, want %v", err, finalErr)
	}
	if calls != 2 {
		t.Fatalf("newSFTPClientWithRetryFactory calls = %d, want 2", calls)
	}
}

func TestNewSFTPClientWithRetryDoesNotRetryOperationErrors(t *testing.T) {
	calls := 0
	finalErr := errors.New("permission denied")
	_, err := newSFTPClientWithRetryFactory(
		&SSHConfig{SourceID: "ssh-test", Host: "host-a", Port: "22"},
		"test",
		func() (*sftp.Client, error) {
			calls++
			return nil, finalErr
		},
	)

	if !errors.Is(err, finalErr) {
		t.Fatalf("newSFTPClientWithRetryFactory error = %v, want %v", err, finalErr)
	}
	if calls != 1 {
		t.Fatalf("newSFTPClientWithRetryFactory calls = %d, want 1", calls)
	}
}

func TestSFTPOperationLockCleanup(t *testing.T) {
	t.Cleanup(func() {
		sftpOperationLocks = sftpLockRegistry{entries: make(map[string]*sftpOperationLockEntry)}
	})
	sftpOperationLocks = sftpLockRegistry{entries: make(map[string]*sftpOperationLockEntry)}

	unlock := acquireSFTPOperationLock("ssh-a")
	if len(sftpOperationLocks.entries) != 1 {
		t.Fatalf("lock entries = %d, want 1", len(sftpOperationLocks.entries))
	}
	unlock()
	if len(sftpOperationLocks.entries) != 0 {
		t.Fatalf("lock entries after unlock = %d, want 0", len(sftpOperationLocks.entries))
	}

	sftpOperationLocks.entries["stale"] = &sftpOperationLockEntry{}
	pruneSFTPOperationLocks([]SSHPathConfig{{SourceID: "ssh-a", Host: "host-a", Port: "22"}})
	if len(sftpOperationLocks.entries) != 0 {
		t.Fatalf("lock entries after prune = %d, want 0", len(sftpOperationLocks.entries))
	}
}

func TestKnownHostsCallback(t *testing.T) {
	if callback, err := knownHostsCallback(filepath.Join(t.TempDir(), "missing_known_hosts")); err == nil {
		t.Fatalf("knownHostsCallback returned callback %v for missing file", callback)
	}

	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatalf("NewPublicKey returned error: %v", err)
	}
	line := knownhosts.Line([]string{"example.com:22"}, sshPublicKey)
	if err := os.WriteFile(knownHostsPath, []byte(line+"\n"), 0600); err != nil {
		t.Fatalf("failed to write known_hosts: %v", err)
	}

	callback, err := knownHostsCallback(knownHostsPath)
	if err != nil {
		t.Fatalf("knownHostsCallback returned error: %v", err)
	}
	if callback == nil {
		t.Fatal("knownHostsCallback returned nil callback")
	}
}

func TestParsePrivateKey(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}

	block, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		t.Fatalf("MarshalPrivateKey returned error: %v", err)
	}
	signer, err := parsePrivateKey(pem.EncodeToMemory(block), "")
	if err != nil {
		t.Fatalf("parsePrivateKey returned error for unencrypted key: %v", err)
	}
	if signer == nil {
		t.Fatal("parsePrivateKey returned nil signer for unencrypted key")
	}

	encryptedBlock, err := ssh.MarshalPrivateKeyWithPassphrase(privateKey, "", []byte("pa#ss.phrase"))
	if err != nil {
		t.Fatalf("MarshalPrivateKeyWithPassphrase returned error: %v", err)
	}
	encryptedKey := pem.EncodeToMemory(encryptedBlock)

	signer, err = parsePrivateKey(encryptedKey, "pa#ss.phrase")
	if err != nil {
		t.Fatalf("parsePrivateKey returned error for encrypted key: %v", err)
	}
	if signer == nil {
		t.Fatal("parsePrivateKey returned nil signer for encrypted key")
	}

	if _, err := parsePrivateKey(encryptedKey, "wrong-passphrase"); err == nil {
		t.Fatal("parsePrivateKey returned nil error for wrong passphrase")
	}
}
