package pkg

import (
	"bufio"
	"compress/gzip"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/acarl005/stripansi"
)

type Watcher struct {
	filePath      string
	matchPattern  string
	ignorePattern string
	mutex         sync.Mutex
	sshConfig     *SSHConfig
	isRemote      bool
}

func NewWatcher(
	filePath string,
	matchPattern string,
	ignorePattern string,
	isRemote bool,
	sshHost string,
	sshPort string,
	sshUser string,
	sshPassword string,
	sshPrivateKeyPath string,
) (*Watcher, error) {
	var sshConfig *SSHConfig
	if isRemote {
		sshConfig = &SSHConfig{
			Host:           sshHost,
			Port:           sshPort,
			User:           sshUser,
			Password:       sshPassword,
			PrivateKeyPath: sshPrivateKeyPath,
			KnownHostsPath: filepath.Join(userHomeDir(), ".ssh", "known_hosts"),
		}
	}

	return newWatcher(filePath, matchPattern, ignorePattern, isRemote, sshConfig), nil
}

func newWatcher(filePath string, matchPattern string, ignorePattern string, isRemote bool, sshConfig *SSHConfig) *Watcher {
	watcher := &Watcher{
		filePath:      filePath,
		matchPattern:  matchPattern,
		ignorePattern: ignorePattern,
		isRemote:      isRemote,
		sshConfig:     sshConfig,
	}

	return watcher
}

type LineResult struct {
	LineNumber int    `json:"line_number"`
	Content    string `json:"content"`
	Level      string `json:"level"`
	Date       string `json:"date"`
	Agent      struct {
		Device string `json:"device"`
	} `json:"agent"`
}

type ScanResult struct {
	FilePath     string       `json:"file_path"`
	Host         string       `json:"host"`
	SourceID     string       `json:"source_id"`
	Type         string       `json:"type"`
	MatchPattern string       `json:"match_pattern"`
	Total        int          `json:"total"`
	Lines        []LineResult `json:"lines"`
}

func (w *Watcher) Scan(page, pageSize int, reverse bool) (*ScanResult, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	file, scanner, err := w.initializeScanner()
	if err != nil {
		return nil, err
	}
	if file != nil {
		defer file.Close()
	}

	allLines, counts, err := w.collectMatchingLines(scanner)
	if err != nil {
		return nil, err
	}

	lines := w.paginateLines(allLines, page, pageSize, reverse)

	AppendGeneralInfo(&lines)
	return &ScanResult{
		FilePath:     w.filePath,
		Host:         w.host(),
		MatchPattern: w.matchPattern,
		Total:        counts,
		Lines:        lines,
	}, nil
}

func (w *Watcher) initializeScanner() (*os.File, *bufio.Scanner, error) {
	if w.isRemote {
		return w.initializeRemoteScanner()
	}

	file, err := os.Open(w.filePath)
	if err != nil {
		return nil, nil, err
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if fileInfo.Size() == 0 {
		return file, bufio.NewScanner(file), nil
	}

	buffer := make([]byte, 2)
	if _, err := file.Read(buffer); err != nil {
		return nil, nil, err
	}
	_, err = file.Seek(0, 0)
	if err != nil {
		return nil, nil, err
	}

	if IsGzip(buffer) {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, nil, err
		}
		return file, bufio.NewScanner(gzipReader), nil
	}

	return file, bufio.NewScanner(file), nil
}

func (w *Watcher) initializeRemoteScanner() (*os.File, *bufio.Scanner, error) {
	if w.sshConfig == nil {
		return nil, nil, os.ErrInvalid
	}

	file, err := sshOpenFile(w.filePath, w.sshConfig)
	if err != nil {
		return nil, nil, err
	}

	return file, bufio.NewScanner(file), nil
}

func (w *Watcher) host() string {
	if w.sshConfig == nil {
		return ""
	}
	return w.sshConfig.Host
}

func (w *Watcher) collectMatchingLines(scanner *bufio.Scanner) ([]LineResult, int, error) {
	re, err := regexp.Compile(w.matchPattern)
	if err != nil {
		return nil, 0, err
	}

	var reIgnore *regexp.Regexp
	if w.ignorePattern != "" {
		reIgnore, err = regexp.Compile(w.ignorePattern)
		if err != nil {
			return nil, 0, err
		}
	}

	var allLines []LineResult
	lineNumber := 0
	counts := 0

	for scanner.Scan() {
		line := scanner.Text()
		line = stripansi.Strip(line)
		lineNumber++
		if reIgnore != nil && reIgnore.MatchString(line) {
			continue
		}
		if re.MatchString(line) {
			allLines = append(allLines, LineResult{
				LineNumber: lineNumber,
				Content:    line,
			})
			counts++
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}

	return allLines, counts, nil
}

func (w *Watcher) paginateLines(allLines []LineResult, page, pageSize int, reverse bool) []LineResult {
	var start, end int
	if reverse {
		start = len(allLines) - (page * pageSize)
		if start < 0 {
			start = 0
		}
		end = start + pageSize
		if end > len(allLines) {
			end = len(allLines)
		}
	} else {
		start = (page - 1) * pageSize
		end = start + pageSize
		if end > len(allLines) {
			end = len(allLines)
		}
	}

	if start < len(allLines) {
		return allLines[start:end]
	}

	return []LineResult{}
}
