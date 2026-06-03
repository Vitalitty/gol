package pkg

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var sftpOperationLocks = sftpLockRegistry{
	entries: make(map[string]*sftpOperationLockEntry),
}

type sftpLockRegistry struct {
	mutex   sync.Mutex
	entries map[string]*sftpOperationLockEntry
}

type sftpOperationLockEntry struct {
	mutex sync.Mutex
	refs  int
}

// IsReadableFile checks if the file is readable and optionally checks for valid UTF-8 encoded content
func IsReadableFile(filename string, isRemote bool, sshConfig *SSHConfig, checkUTF8 bool) (bool, error) {
	var file *os.File
	var err error

	if isRemote {
		file, err = sshOpenFile(filename, sshConfig)
	} else {
		file, err = os.Open(filename)
	}
	if err != nil {
		return false, err
	}
	defer file.Close()

	// Check if the file is empty
	fileInfo, err := file.Stat()
	if err != nil {
		return false, err
	}
	if fileInfo.Size() == 0 {
		return true, nil
	}

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil {
		return false, err
	}

	// Check if the file is gzip compressed
	if IsGzip(buffer[:n]) {
		_, err = file.Seek(0, io.SeekStart) // Reset file pointer
		if err != nil {
			return false, err
		}

		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return false, err
		}
		defer gzipReader.Close()

		n, err = gzipReader.Read(buffer)
		if err != nil && !errors.Is(err, io.EOF) {
			return false, err
		}

		if checkUTF8 {
			return utf8.Valid(buffer[:n]), nil
		}
		return true, nil
	}

	if checkUTF8 {
		return utf8.Valid(buffer[:n]), nil
	}
	return true, nil
}

// IsGzip checks if the given buffer starts with the gzip magic number
func IsGzip(buffer []byte) bool {
	return len(buffer) >= 2 && buffer[0] == 0x1f && buffer[1] == 0x8b
}

func FilesByPattern(pattern string, isRemote bool, sshConfig *SSHConfig) ([]string, error) {
	if isRemote {
		return sshFilesByPattern(pattern, sshConfig)
	}

	// Check if the pattern is a directory
	info, err := os.Stat(pattern)
	if err == nil && info.IsDir() {
		// List all files in the directory
		var files []string
		err := filepath.Walk(pattern, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		return files, nil
	}

	// If pattern is not a directory, use Glob to match the pattern
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	return files, nil
}

func detectMimeType(file *os.File) (string, error) {
	buffer := make([]byte, 512)
	_, err := file.Read(buffer)
	if err != nil {
		return "", err
	}
	// Reset the file pointer to the beginning of the file
	_, err = file.Seek(0, 0)
	if err != nil {
		return "", err
	}
	return http.DetectContentType(buffer), nil
}

// FileStats returns the number of lines and size of the file at the given path.
func FileStats(filePath string, isRemote bool, sshConfig *SSHConfig) (int, int64, error) {
	var file *os.File
	var err error

	if isRemote {
		file, err = sshOpenFile(filePath, sshConfig)
	} else {
		file, err = os.Open(filePath)
	}
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	mimeType, err := detectMimeType(file)
	if err != nil {
		return 0, 0, err
	}

	var reader *bufio.Reader
	if mimeType == "application/x-gzip" {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return 0, 0, err
		}
		defer gzReader.Close()
		reader = bufio.NewReader(gzReader)
	} else {
		reader = bufio.NewReader(file)
	}

	var linesCount int
	scanner := bufio.NewScanner(reader)
	// Increase buffer size to 10MB
	buf := make([]byte, 10*1024*1024) // 10MB buffer
	scanner.Buffer(buf, len(buf))

	for scanner.Scan() {
		linesCount++
	}

	if err := scanner.Err(); err != nil {
		return 0, 0, err
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return 0, 0, err
	}
	fileSize := fileInfo.Size()

	return linesCount, fileSize, nil
}

func GetFileInfos(pattern string, limit int, isRemote bool, sshConfig *SSHConfig) []FileInfo {
	filePaths, err := FilesByPattern(pattern, isRemote, sshConfig)
	if err != nil {
		slog.Error("getting file paths by pattern", "pattern", pattern, "error", err)
		return nil
	}
	if len(filePaths) == 0 {
		slog.Error("No files found", "pattern", pattern)
		return nil
	}
	fileInfos := make([]FileInfo, 0)
	if len(filePaths) > limit {
		slog.Warn("Limiting to files", "limit", limit)
		filePaths = filePaths[:limit]
	}

	for _, filePath := range filePaths {
		isText, err := IsReadableFile(filePath, isRemote, sshConfig, false)
		if err != nil {
			slog.Error("checking if file is readable", "filePath", filePath, "error", err)
			return nil
		}
		if !isText {
			slog.Warn("File is not a text file", "filePath", filePath)
			continue
		}
		linesCount, fileSize, err := FileStats(filePath, isRemote, sshConfig)
		if err != nil {
			if errors.Is(err, io.EOF) {
				slog.Warn("File is empty", "filePath", filePath)
				linesCount = 0
				fileSize = 0
			} else {
				slog.Error("getting file stats", "filePath", filePath, "error", err)
				continue
			}
		}
		t := TypeFile
		h := ""
		sourceID := ""
		if isRemote {
			t = TypeSSH
			h = sshConfig.Host
			sourceID = sshConfig.SourceID
		}
		if filePath == GlobalPipeTmpFilePath {
			t = TypeStdin
		}
		fileInfos = append(fileInfos, FileInfo{FilePath: filePath, LinesCount: linesCount, FileSize: fileSize, Type: t, Host: h, SourceID: sourceID})
	}
	return fileInfos
}

// SSHConfig holds the SSH connection parameters
type SSHConfig struct {
	SourceID             string
	Host                 string
	Port                 string
	User                 string
	Password             string
	PrivateKeyPath       string
	PrivateKeyPassphrase string
	KnownHostsPath       string
}

type SSHPathConfig struct {
	SourceID             string
	Host                 string
	Port                 string
	User                 string
	Password             string
	PrivateKeyPath       string
	PrivateKeyPassphrase string
	KnownHostsPath       string
	FilePath             string
}

type DockerPathConfig struct {
	ContainerID string
	FilePath    string
}

// s is an input of the form "container_id /path/to/file"
func StringToDockerPathConfig(s string) (*DockerPathConfig, error) {
	// Split the input string into parts
	parts := strings.Fields(s)

	// There should be 2 parts: "container_id" and "/path/to/file"
	if len(parts) < 2 {
		return nil, fmt.Errorf("input string does not have the correct format")
	}
	return &DockerPathConfig{
		ContainerID: parts[0],
		FilePath:    parts[1],
	}, nil
}

// s is an input of the form "user@host[:port] [password=password] [private_key=/path/to/key] [key_passphrase=passphrase] [known_hosts=/path/to/known_hosts] /path/to/file"
func StringToSSHPathConfig(s string) (*SSHPathConfig, error) {
	config := &SSHPathConfig{}

	// Split the input string into parts
	parts := strings.Fields(s)

	// There should be at least 2 parts: "user@host[:port]" and "/path/to/file"
	if len(parts) < 2 {
		return nil, fmt.Errorf("input string does not have the correct format")
	}

	// Extract user@host[:port]
	userHostPort := strings.Split(parts[0], "@")
	if len(userHostPort) != 2 {
		return nil, fmt.Errorf("user@host[:port] part does not have the correct format")
	}

	userHost := strings.Split(userHostPort[1], ":")
	config.User = userHostPort[0]
	config.Host = userHost[0]

	// Set the default port if not specified
	if len(userHost) == 2 {
		config.Port = userHost[1]
	} else {
		config.Port = "22" // Default SSH port
	}

	config.KnownHostsPath = filepath.Join(userHomeDir(), ".ssh", "known_hosts")

	// Extract optional parts and file path
	for _, part := range parts[1:] {
		// nolint: gocritic
		if strings.HasPrefix(part, "password=") {
			config.Password = strings.TrimPrefix(part, "password=")
		} else if strings.HasPrefix(part, "private_key=") {
			config.PrivateKeyPath = strings.TrimPrefix(part, "private_key=")
		} else if strings.HasPrefix(part, "key_passphrase=") {
			config.PrivateKeyPassphrase = strings.TrimPrefix(part, "key_passphrase=")
		} else if strings.HasPrefix(part, "known_hosts=") {
			config.KnownHostsPath = strings.TrimPrefix(part, "known_hosts=")
		} else {
			config.FilePath = part
		}
	}

	if config.FilePath == "" {
		return nil, fmt.Errorf("file path is missing")
	}
	if config.Password != "" && config.PrivateKeyPath != "" {
		return nil, fmt.Errorf("password= and private_key= cannot be used together")
	}
	if config.Password != "" && config.PrivateKeyPassphrase != "" {
		return nil, fmt.Errorf("key_passphrase= requires private key authentication")
	}
	if config.Password == "" && config.PrivateKeyPath == "" {
		config.PrivateKeyPath = filepath.Join(userHomeDir(), ".ssh", "id_rsa")
	}

	return config, nil
}

func userHomeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	return os.Getenv("USERPROFILE")
}

func (c *SSHPathConfig) toSSHConfig() SSHConfig {
	return SSHConfig{
		SourceID:             c.SourceID,
		Host:                 c.Host,
		Port:                 c.Port,
		User:                 c.User,
		Password:             c.Password,
		PrivateKeyPath:       c.PrivateKeyPath,
		PrivateKeyPassphrase: c.PrivateKeyPassphrase,
		KnownHostsPath:       c.KnownHostsPath,
	}
}

func sshConnect(config *SSHConfig) (*ssh.Client, error) {
	var auth []ssh.AuthMethod

	if config.Password != "" && config.PrivateKeyPath != "" {
		return nil, fmt.Errorf("password and private key authentication cannot be combined")
	}
	if config.Password != "" && config.PrivateKeyPassphrase != "" {
		return nil, fmt.Errorf("private key passphrase requires private key authentication")
	}
	if config.Password != "" {
		auth = append(auth, ssh.Password(config.Password))
	}
	if config.PrivateKeyPath != "" {
		key, err := os.ReadFile(config.PrivateKeyPath)
		if err != nil {
			return nil, err
		}
		signer, err := parsePrivateKey(key, config.PrivateKeyPassphrase)
		if err != nil {
			return nil, err
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if len(auth) == 0 {
		return nil, fmt.Errorf("ssh authentication method is required")
	}

	hostKeyCallback, err := knownHostsCallback(config.KnownHostsPath)
	if err != nil {
		return nil, err
	}

	clientConfig := &ssh.ClientConfig{
		User:            config.User,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
	}

	client, err := ssh.Dial("tcp", config.Host+":"+config.Port, clientConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func parsePrivateKey(key []byte, passphrase string) (ssh.Signer, error) {
	if passphrase != "" {
		return ssh.ParsePrivateKeyWithPassphrase(key, []byte(passphrase))
	}
	return ssh.ParsePrivateKey(key)
}

func knownHostsCallback(knownHostsPath string) (ssh.HostKeyCallback, error) {
	if knownHostsPath == "" {
		return nil, fmt.Errorf("known_hosts path is required")
	}
	return knownhosts.New(knownHostsPath)
}

func newSSHSourceID(config *SSHPathConfig, sourceIndex int) string {
	authMethod := "key"
	if config.Password != "" {
		authMethod = "password"
	}
	hashInput := strings.Join([]string{
		config.User,
		config.Host,
		config.Port,
		authMethod,
		config.PrivateKeyPath,
		config.KnownHostsPath,
		config.FilePath,
	}, "\x00")
	sum := sha256.Sum256([]byte(hashInput))
	return fmt.Sprintf("ssh-%d-%s", sourceIndex, hex.EncodeToString(sum[:])[:12])
}

func sshOpenFile(filename string, config *SSHConfig) (*os.File, error) {
	tmpFile, err := os.Create(GetTmpFileNameForSTDIN())
	if err != nil {
		return nil, err
	}

	err = withSFTPClient(config, "open file", func(sftpClient *sftp.Client) error {
		remoteFile, err := sftpClient.Open(filename)
		if err != nil {
			return err
		}
		defer remoteFile.Close()

		if _, err := io.Copy(tmpFile, remoteFile); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if closeErr := tmpFile.Close(); closeErr != nil {
			slog.Debug("closing failed SSH temp file", "path", tmpFile.Name(), "error", closeErr)
		}
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil && !os.IsNotExist(removeErr) {
			slog.Debug("removing failed SSH temp file", "path", tmpFile.Name(), "error", removeErr)
		}
		return nil, err
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	return tmpFile, nil
}

func sshFilesByPattern(pattern string, config *SSHConfig) ([]string, error) {
	var matches []string
	err := withSFTPClient(config, "glob files", func(sftpClient *sftp.Client) error {
		var err error
		matches, err = sftpClient.Glob(pattern)
		return err
	})
	return matches, err
}

func withSFTPClient(config *SSHConfig, operation string, fn func(*sftp.Client) error) error {
	unlock := acquireSFTPOperationLock(sshClientKey(config))
	defer unlock()

	sftpClient, err := newSFTPClientWithRetry(config, operation)
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	return fn(sftpClient)
}

func newSFTPClientWithRetry(config *SSHConfig, operation string) (*sftp.Client, error) {
	return newSFTPClientWithRetryFactory(config, operation, func() (*sftp.Client, error) {
		return newSFTPClient(config)
	})
}

func newSFTPClientWithRetryFactory(config *SSHConfig, operation string, create func() (*sftp.Client, error)) (*sftp.Client, error) {
	sftpClient, err := create()
	if err == nil || !isRetryableSFTPChannelError(err) {
		return sftpClient, err
	}

	slog.Warn(
		"retrying SFTP client creation with a fresh SSH client",
		"host", config.Host,
		"port", config.Port,
		"operation", operation,
		"error", err,
	)
	removeCachedSSHClient(config)
	return create()
}

func newSFTPClient(config *SSHConfig) (*sftp.Client, error) {
	client, err := NewOrReusableClient(config)
	if err != nil {
		return nil, err
	}
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return nil, err
	}
	return sftpClient, nil
}

func acquireSFTPOperationLock(key string) func() {
	sftpOperationLocks.mutex.Lock()
	entry := sftpOperationLocks.entries[key]
	if entry == nil {
		entry = &sftpOperationLockEntry{}
		sftpOperationLocks.entries[key] = entry
	}
	entry.refs++
	sftpOperationLocks.mutex.Unlock()

	entry.mutex.Lock()

	return func() {
		entry.mutex.Unlock()

		sftpOperationLocks.mutex.Lock()
		defer sftpOperationLocks.mutex.Unlock()
		entry.refs--
		if entry.refs == 0 && sftpOperationLocks.entries[key] == entry {
			delete(sftpOperationLocks.entries, key)
		}
	}
}

func pruneSFTPOperationLocks(sshConfigs []SSHPathConfig) {
	validKeys := make(map[string]struct{}, len(sshConfigs))
	for _, sshConfig := range sshConfigs {
		config := sshConfig.toSSHConfig()
		validKeys[sshClientKey(&config)] = struct{}{}
	}

	sftpOperationLocks.mutex.Lock()
	defer sftpOperationLocks.mutex.Unlock()
	for key, entry := range sftpOperationLocks.entries {
		if entry.refs > 0 {
			continue
		}
		if _, ok := validKeys[key]; !ok {
			delete(sftpOperationLocks.entries, key)
		}
	}
}

func isRetryableSFTPChannelError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "ssh: rejected:") &&
		strings.Contains(message, "connect failed") &&
		strings.Contains(message, "open failed")
}

func UniqueFileInfos(fileInfos []FileInfo) []FileInfo {
	seen := make(map[string]struct{}, len(fileInfos))
	unique := make([]FileInfo, 0, len(fileInfos))
	for _, fileInfo := range fileInfos {
		key := fileInfo.Type + "\x00" + fileInfo.Host + "\x00" + fileInfo.SourceID + "\x00" + fileInfo.FilePath
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, fileInfo)
	}
	return unique
}
