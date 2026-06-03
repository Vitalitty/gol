package pkg

import (
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

var GlobalFilePaths []FileInfo
var GlobalPipeTmpFilePath string
var GlobalPathSSHConfig []SSHPathConfig
var GlobalSSHClients = make(map[string]*ssh.Client)
var globalStateMutex sync.RWMutex

type GlobalStateSnapshot struct {
	FilePaths       []FileInfo
	PipeTmpFilePath string
	SSHConfigs      []SSHPathConfig
}

func WatchFilePaths(seconds int64, filePaths SliceFlags, sshPaths SliceFlags, dockerPaths SliceFlags, limit int) {
	interval := time.Duration(seconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		slog.Info("Checking for filepaths", "interval", interval)
		UpdateGlobalFilePaths(filePaths, sshPaths, dockerPaths, limit)
	}
}

func HandleStdinPipe() {
	tmpFile, err := os.Create(GetTmpFileNameForSTDIN())
	if err != nil {
		slog.Error("creating temp file", "error", err)
		return
	}
	globalStateMutex.Lock()
	GlobalPipeTmpFilePath = tmpFile.Name()
	globalStateMutex.Unlock()
	go func(tmpFile *os.File) {
		defer tmpFile.Close()
		err := PipeLinesToTmp(tmpFile)
		if err != nil {
			slog.Error("piping lines to temp file", "path", tmpFile.Name(), "error", err)
			return
		}
	}(tmpFile)
}

func UpdateGlobalFilePaths(filePaths SliceFlags, sshPaths SliceFlags, dockerPaths SliceFlags, limit int) {
	fileInfos := []FileInfo{}
	sshConfigs := []SSHPathConfig{}

	for _, pattern := range filePaths {
		fileInfo := GetFileInfos(pattern, limit, false, nil)
		fileInfos = append(fileInfo, fileInfos...)
	}
	for sourceIndex, pattern := range sshPaths {
		sshFilePathConfig, err := StringToSSHPathConfig(pattern)
		if err != nil {
			slog.Error("parsing SSH path", "pattern", pattern, "error", err)
			continue
		}
		sshFilePathConfig.SourceID = newSSHSourceID(sshFilePathConfig, sourceIndex)
		sshConfig := SSHConfig{
			SourceID:             sshFilePathConfig.SourceID,
			Host:                 sshFilePathConfig.Host,
			Port:                 sshFilePathConfig.Port,
			User:                 sshFilePathConfig.User,
			Password:             sshFilePathConfig.Password,
			PrivateKeyPath:       sshFilePathConfig.PrivateKeyPath,
			PrivateKeyPassphrase: sshFilePathConfig.PrivateKeyPassphrase,
			KnownHostsPath:       sshFilePathConfig.KnownHostsPath,
		}
		sshConfigs = append(sshConfigs, *sshFilePathConfig)
		fileInfo := GetFileInfos(sshFilePathConfig.FilePath, limit, true, &sshConfig)
		fileInfos = append(fileInfo, fileInfos...)
	}

	for _, pattern := range dockerPaths {
		containers, err := ListDockerContainers()
		if err != nil {
			slog.Error("listing Docker containers", "pattern", pattern, "error", err)
			break
		}
		if pattern == "" || len(strings.Fields(pattern)) == 1 {
			for _, container := range containers {
				if pattern != "" && !strings.Contains(container.Names[0], pattern) {
					continue
				}
				tmpFile := ContainerStdoutToTmp(container.ID)
				if tmpFile == nil {
					slog.Error("creating temp file for container logs", "containerID", container.ID)
					continue
				}
				tmpFileName := tmpFile.Name()
				if err := tmpFile.Close(); err != nil {
					slog.Error("closing container log temp file", "path", tmpFileName, "error", err)
					continue
				}
				fileInfo := GetFileInfos(tmpFileName, limit, false, nil)
				if len(fileInfo) > 0 {
					fileInfo[0].Host = container.ID[:12]
					fileInfo[0].Type = TypeDocker
					fileInfo[0].Name = container.Names[0][1:]
					fileInfos = append(fileInfo, fileInfos...)
				}
			}
		}
		if len(strings.Fields(pattern)) == 2 {
			dockerFilePathConfig, err := StringToDockerPathConfig(pattern)
			if err != nil {
				slog.Error("parsing Docker path", "pattern", pattern, "error", err)
				break
			}
			fileInfo := GetContainerFileInfos(dockerFilePathConfig.FilePath, limit, dockerFilePathConfig.ContainerID)
			fileInfos = append(fileInfo, fileInfos...)
		}
	}

	setGlobalState(UniqueFileInfos(fileInfos), sshConfigs)
}

func setGlobalState(filePaths []FileInfo, sshConfigs []SSHPathConfig) {
	globalStateMutex.Lock()

	GlobalFilePaths = append([]FileInfo(nil), filePaths...)
	GlobalPathSSHConfig = append([]SSHPathConfig(nil), sshConfigs...)
	globalStateMutex.Unlock()

	pruneSFTPOperationLocks(sshConfigs)
}

func SnapshotGlobalState() GlobalStateSnapshot {
	globalStateMutex.RLock()
	defer globalStateMutex.RUnlock()

	return GlobalStateSnapshot{
		FilePaths:       append([]FileInfo(nil), GlobalFilePaths...),
		PipeTmpFilePath: GlobalPipeTmpFilePath,
		SSHConfigs:      append([]SSHPathConfig(nil), GlobalPathSSHConfig...),
	}
}

func (s GlobalStateSnapshot) FindFileInfo(filePath string, fileType string, host string, sourceID string) (FileInfo, bool) {
	for _, fileInfo := range s.FilePaths {
		if fileInfo.FilePath != filePath {
			continue
		}
		if fileType == "" && host == "" {
			return fileInfo, true
		}
		if fileType != "" && fileInfo.Type != fileType {
			continue
		}
		if host != "" && fileInfo.Host != host {
			continue
		}
		if sourceID != "" && fileInfo.SourceID != sourceID {
			continue
		}
		if fileInfo.Host == host {
			return fileInfo, true
		}
	}
	return FileInfo{}, false
}

func (s GlobalStateSnapshot) FindSSHConfig(sourceID string, host string) (SSHPathConfig, bool) {
	if sourceID != "" {
		for _, sshConfig := range s.SSHConfigs {
			if sshConfig.SourceID == sourceID && (host == "" || sshConfig.Host == host) {
				return sshConfig, true
			}
		}
	}
	for _, sshConfig := range s.SSHConfigs {
		if sshConfig.Host == host {
			return sshConfig, true
		}
	}
	return SSHPathConfig{}, false
}
