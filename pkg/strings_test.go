package pkg

import (
	"testing"
)

func TestFindGlobalFileInfo(t *testing.T) {
	previous := SnapshotGlobalState()
	t.Cleanup(func() {
		setGlobalState(previous.FilePaths, previous.SSHConfigs)
	})

	setGlobalState([]FileInfo{
		{FilePath: "/var/log/app.log", Type: TypeFile, Host: ""},
		{FilePath: "/var/log/app.log", Type: TypeSSH, Host: "host-a"},
		{FilePath: "/var/log/app.log", Type: TypeSSH, Host: "host-b"},
	}, nil)

	fileInfo, ok := FindGlobalFileInfo("/var/log/app.log", TypeSSH, "host-b")
	if !ok {
		t.Fatal("FindGlobalFileInfo did not find SSH file for host-b")
	}
	if fileInfo.Type != TypeSSH || fileInfo.Host != "host-b" {
		t.Fatalf("FindGlobalFileInfo returned %+v, want SSH host-b", fileInfo)
	}

	if _, ok := FindGlobalFileInfo("/var/log/app.log", TypeSSH, "host-c"); ok {
		t.Fatal("FindGlobalFileInfo matched the wrong SSH host")
	}
	if _, ok := FindGlobalFileInfo("/var/log/app.log", TypeDocker, "host-b"); ok {
		t.Fatal("FindGlobalFileInfo matched the wrong file type")
	}
}

func TestJudgeLogLevel(t *testing.T) {
	tests := []struct {
		line            string
		keywordPosition int
		want            string
	}{
		{"INFO: Everything is working fine.", 0, "info"},
		{"error: Failed to connect to the database.", 0, "error"},
		{"warn: Deprecated API usage.", 0, "warn"},
		{"fatal: Unexpected null pointer exception.", 0, "danger"},
		{"debug: Variable x has value 10.", 0, "debug"},
		{"INFO: Just another log entry.", 0, "info"},
		{"This is just a normal log entry.", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := JudgeLogLevel(tt.line, tt.keywordPosition)
			if got != tt.want {
				t.Errorf("JudgeLogLevel(%s, %d) = %s; want %s", tt.line, tt.keywordPosition, got, tt.want)
			}
		})
	}
}

func TestConsistentFormat(t *testing.T) {
	tests := []struct {
		logLines       []string
		wantConsistent bool
		wantPosition   int
	}{
		{
			[]string{
				"INFO: Everything is working fine.",
				"ERROR: Failed to connect to the database.",
				"WARNING: Deprecated API usage.",
				"FATAL: Unexpected null pointer exception.",
				"DEBUG: Variable x has value 10.",
			},
			true,
			0,
		},
		{
			[]string{
				"INFO - Everything is working fine.",
				"ERROR - Failed to connect to the database.",
				"WARNING - Deprecated API usage.",
				"FATAL - Unexpected null pointer exception.",
				"DEBUG - Variable x has value 10.",
			},
			true,
			0,
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			gotConsistent, gotPosition := ConsistentFormat(tt.logLines)
			if gotConsistent != tt.wantConsistent || gotPosition != tt.wantPosition {
				t.Errorf("ConsistentFormat(%v) = %v, %d; want %v, %d", tt.logLines, gotConsistent, gotPosition, tt.wantConsistent, tt.wantPosition)
			}
		})
	}
}
