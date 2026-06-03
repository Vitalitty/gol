package pkg

import (
	"fmt"
	"sync"
	"testing"
)

func TestSnapshotGlobalStateFindsSSHFileAndConfig(t *testing.T) {
	previous := SnapshotGlobalState()
	t.Cleanup(func() {
		setGlobalState(previous.FilePaths, previous.SSHConfigs)
		globalStateMutex.Lock()
		GlobalPipeTmpFilePath = previous.PipeTmpFilePath
		globalStateMutex.Unlock()
	})

	setGlobalState(
		[]FileInfo{{FilePath: "/var/log/app.log", Type: TypeSSH, Host: "host-a", SourceID: "ssh-a"}},
		[]SSHPathConfig{{Host: "host-a", User: "user", Port: "22", FilePath: "/var/log/*.log", SourceID: "ssh-a"}},
	)

	state := SnapshotGlobalState()
	fileInfo, ok := state.FindFileInfo("/var/log/app.log", TypeSSH, "host-a", "ssh-a")
	if !ok {
		t.Fatal("snapshot did not find SSH file info")
	}
	if fileInfo.Host != "host-a" {
		t.Fatalf("fileInfo.Host = %q, want host-a", fileInfo.Host)
	}

	sshConfig, ok := state.FindSSHConfig("ssh-a", "host-a")
	if !ok {
		t.Fatal("snapshot did not find SSH config")
	}
	if sshConfig.User != "user" {
		t.Fatalf("sshConfig.User = %q, want user", sshConfig.User)
	}
}

func TestSnapshotGlobalStateDoesNotExposePartialSSHRefresh(t *testing.T) {
	previous := SnapshotGlobalState()
	t.Cleanup(func() {
		setGlobalState(previous.FilePaths, previous.SSHConfigs)
		globalStateMutex.Lock()
		GlobalPipeTmpFilePath = previous.PipeTmpFilePath
		globalStateMutex.Unlock()
	})

	const iterations = 1000
	var wg sync.WaitGroup
	ready := make(chan struct{})
	done := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ready
		for i := 0; i < iterations; i++ {
			host := fmt.Sprintf("host-%d", i)
			setGlobalState(
				[]FileInfo{{FilePath: "/var/log/app.log", Type: TypeSSH, Host: host, SourceID: host}},
				[]SSHPathConfig{{Host: host, User: "user", Port: "22", FilePath: "/var/log/*.log", SourceID: host}},
			)
		}
		close(done)
	}()

	close(ready)
	for {
		select {
		case <-done:
			wg.Wait()
			return
		default:
			state := SnapshotGlobalState()
			if len(state.FilePaths) == 0 || state.FilePaths[0].Type != TypeSSH {
				continue
			}
			host := state.FilePaths[0].Host
			if _, ok := state.FindSSHConfig(state.FilePaths[0].SourceID, host); !ok {
				t.Fatalf("missing SSH config for host %q", host)
			}
		}
	}
}
