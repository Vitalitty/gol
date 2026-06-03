package pkg

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"

	"github.com/acarl005/stripansi"
	"github.com/kevincobain2000/go-human-uuid/lib"
)

const maxTempLogLines = 10000

func GetHomedir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// IsInputFromPipe checks if there is input from a pipe
func IsInputFromPipe() bool {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	// Check if the mode is a character device (i.e., a pipe)
	return (fileInfo.Mode() & os.ModeCharDevice) == 0
}

func PipeLinesToTmp(tmpFile *os.File) error {
	scanner := bufio.NewScanner(os.Stdin)
	tmpFilePath := tmpFile.Name()

	slog.Info("Temporary file created for stdin", "path", tmpFilePath)

	linesCount, fileSize, err := FileStats(tmpFilePath, false, nil)
	if err != nil {
		slog.Error("creating FileInfo for temp file", "path", tmpFilePath, "error", err)
		return err
	}
	tempFileInfo := FileInfo{FilePath: tmpFilePath, LinesCount: linesCount, FileSize: fileSize, Type: TypeStdin}

	globalStateMutex.Lock()
	GlobalFilePaths = append([]FileInfo{tempFileInfo}, GlobalFilePaths...)
	filePaths := append([]FileInfo(nil), GlobalFilePaths...)
	globalStateMutex.Unlock()
	slog.Info("Temporary file added to global file paths", "filePaths", filePaths)

	return writeRollingScannerLines(scanner, tmpFile, maxTempLogLines)
}

func writeRollingScannerLines(scanner *bufio.Scanner, tmpFile *os.File, maxLines int) error {
	lineCount := 0
	for scanner.Scan() {
		if lineCount >= maxLines {
			if err := tmpFile.Truncate(0); err != nil {
				return fmt.Errorf("truncate temp log: %w", err)
			}
			if _, err := tmpFile.Seek(0, 0); err != nil {
				return fmt.Errorf("seek temp log: %w", err)
			}
			lineCount = 0
		}
		if _, err := tmpFile.WriteString(stripansi.Strip(scanner.Text()) + "\n"); err != nil {
			return fmt.Errorf("write temp log: %w", err)
		}
		lineCount++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan temp log: %w", err)
	}
	return nil
}

func GetTmpFileNameForSTDIN() string {
	gen, _ := lib.NewGenerator([]lib.Option{
		func(opt *lib.Options) error {
			opt.Length = 2
			return nil
		},
	}...)
	return TmpStdinPath + gen.Generate()
}

func GetTmpFileNameForContainer() string {
	gen, _ := lib.NewGenerator([]lib.Option{
		func(opt *lib.Options) error {
			opt.Length = 6
			return nil
		},
	}...)
	return TmpContainerPath + gen.Generate()
}

func OpenBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = exec.Command("xdg-open", url).Start()
	}

	if err != nil {
		slog.Warn("Failed to open browser", "url", url, "error", err)
	}
}

func HandleCltrC(f func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		s := <-c
		slog.Warn("Got signal", "signal", s)
		f()
		close(c)
		os.Exit(1)
	}()
}

func Cleanup() {
	paths := make(map[string]struct{})
	state := SnapshotGlobalState()
	if state.PipeTmpFilePath != "" {
		paths[state.PipeTmpFilePath] = struct{}{}
	}
	for _, fileInfo := range state.FilePaths {
		if fileInfo.Type == TypeDocker && strings.HasPrefix(fileInfo.FilePath, TmpContainerPath) {
			paths[fileInfo.FilePath] = struct{}{}
		}
	}
	for path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			slog.Error("removing temp file", "path", path, "error", err)
			continue
		}
		slog.Info("temp file removed", "path", path)
	}
}
