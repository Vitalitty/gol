package pkg

import (
	"os"
	"path/filepath"
)

const (
	TypeFile   = "file"
	TypeStdin  = "stdin"
	TypeSSH    = "ssh"
	TypeDocker = "docker"

	ErrorMsgSessionAlreadyStarted = "ssh: session already started"
)

var (
	TmpStdinPath     = filepath.Join(os.TempDir(), "GOL-STDIN-")
	TmpContainerPath = filepath.Join(os.TempDir(), "GOL-CONTAINER-")
)
