package pkg

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestNewAPIHandler(t *testing.T) {
	handler := NewAPIHandler()
	assert.NotNil(t, handler)
}

func TestAPIHandler_Get(t *testing.T) {
	e := echo.New()
	previous := SnapshotGlobalState()
	t.Cleanup(func() {
		setGlobalState(previous.FilePaths, previous.SSHConfigs)
		globalStateMutex.Lock()
		GlobalPipeTmpFilePath = previous.PipeTmpFilePath
		globalStateMutex.Unlock()
	})

	filePaths := []FileInfo{
		{
			FilePath:   "test.log",
			LinesCount: 4,
			FileSize:   0,
			Type:       TypeFile,
		},
	}
	setGlobalState(filePaths, nil)
	globalStateMutex.Lock()
	GlobalPipeTmpFilePath = "temp.log"
	globalStateMutex.Unlock()

	// Create a temporary log file for testing
	// nolint:goconst
	content := `INFO Starting service
ERROR An error occurred
INFO Service running
ERROR Another error occurred`
	err := os.WriteFile(filePaths[0].FilePath, []byte(content), 0600)
	assert.NoError(t, err)
	defer os.Remove(filePaths[0].FilePath)

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/api?query=ERROR&page=1&per_page=10", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Create the API handler and execute the Get method
	handler := NewAPIHandler()
	if assert.NoError(t, handler.Get(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		expected := `{
			"result": {
				"file_path": "test.log",
				"host": "",
				"source_id": "",
				"type": "file",
				"match_pattern": "ERROR",
				"total": 2,
				"lines": [
				{
					"line_number": 2,
					"content": "ERROR An error occurred",
					"level": "error",
					"date": "",
					"agent": {
						"device": "server"
					}
				},
				{
					"line_number": 4,
					"content": "ERROR Another error occurred",
					"level": "error",
					"date": "",
					"agent": {
						"device": "server"
					}
				}
				]
			},
			"file_paths": [
				{
					"file_path": "test.log",
					"lines_count": 4,
					"file_size": 0,
					"type": "file",
					"host": "",
					"source_id": "",
					"name": ""
				}
			]
		}`
		fmt.Println(rec.Body.String())
		assert.JSONEq(t, expected, rec.Body.String())
	}
}
func TestAPIHandler_Get404(t *testing.T) {
	e := echo.New()
	previous := SnapshotGlobalState()
	t.Cleanup(func() {
		setGlobalState(previous.FilePaths, previous.SSHConfigs)
		globalStateMutex.Lock()
		GlobalPipeTmpFilePath = previous.PipeTmpFilePath
		globalStateMutex.Unlock()
	})

	filePaths := []FileInfo{
		{
			FilePath:   "test.log",
			LinesCount: 4,
			FileSize:   0,
			Type:       TypeFile,
		},
	}
	setGlobalState(filePaths, nil)
	globalStateMutex.Lock()
	GlobalPipeTmpFilePath = "temp.log"
	globalStateMutex.Unlock()

	// nolint:goconst
	content := `INFO Starting service
	ERROR An error occurred
	INFO Service running
	ERROR Another error occurred`
	err := os.WriteFile(filePaths[0].FilePath, []byte(content), 0600)
	assert.NoError(t, err)
	defer os.Remove(filePaths[0].FilePath)

	handler := NewAPIHandler()

	req := httptest.NewRequest(http.MethodGet, "/api?file_path=wrong", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	resp := handler.Get(c)

	assert.Error(t, resp)
	// nolint: errorlint
	if he, ok := resp.(*echo.HTTPError); ok {
		assert.Equal(t, http.StatusUnprocessableEntity, he.Code)
	} else {
		assert.Fail(t, "response is not an HTTP error")
	}

	req = httptest.NewRequest(http.MethodGet, "/api?file_path=wrong&type=file", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	resp = handler.Get(c)

	assert.Error(t, resp)
	// nolint: errorlint
	if he, ok := resp.(*echo.HTTPError); ok {
		assert.Equal(t, http.StatusNotFound, he.Code)
	} else {
		assert.Fail(t, "response is not an HTTP error")
	}
}

func TestAPIHandler_GetRejectsUnlistedAbsolutePath(t *testing.T) {
	e := echo.New()
	previous := SnapshotGlobalState()
	t.Cleanup(func() {
		setGlobalState(previous.FilePaths, previous.SSHConfigs)
	})

	dir := t.TempDir()
	listedPath := filepath.Join(dir, "listed.log")
	unlistedPath := filepath.Join(dir, "unlisted.log")
	if err := os.WriteFile(listedPath, []byte("ERROR listed\n"), 0600); err != nil {
		t.Fatalf("failed to write listed file: %v", err)
	}
	if err := os.WriteFile(unlistedPath, []byte("ERROR unlisted\n"), 0600); err != nil {
		t.Fatalf("failed to write unlisted file: %v", err)
	}

	setGlobalState([]FileInfo{
		{
			FilePath:   listedPath,
			LinesCount: 1,
			Type:       TypeFile,
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api?file_path="+url.QueryEscape(unlistedPath)+"&type=file", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	resp := NewAPIHandler().Get(c)
	assert.Error(t, resp)
	// nolint: errorlint
	if he, ok := resp.(*echo.HTTPError); ok {
		assert.Equal(t, http.StatusNotFound, he.Code)
	} else {
		assert.Fail(t, "response is not an HTTP error")
	}
}

func TestAPIHandler_GetSamePathRequiresMatchingTypeAndHost(t *testing.T) {
	previous := SnapshotGlobalState()
	t.Cleanup(func() {
		setGlobalState(previous.FilePaths, previous.SSHConfigs)
	})

	setGlobalState([]FileInfo{
		{FilePath: "/var/log/app.log", Type: TypeFile, Host: ""},
		{FilePath: "/var/log/app.log", Type: TypeSSH, Host: "host-a", SourceID: "ssh-a"},
	}, []SSHPathConfig{{Host: "host-a", Port: "22", User: "user", SourceID: "ssh-a"}})

	state := SnapshotGlobalState()
	if fileInfo, ok := state.FindFileInfo("/var/log/app.log", TypeFile, "", ""); !ok || fileInfo.Type != TypeFile {
		t.Fatalf("FindFileInfo did not resolve the local file entry: %+v", fileInfo)
	}
	if fileInfo, ok := state.FindFileInfo("/var/log/app.log", TypeSSH, "host-a", "ssh-a"); !ok || fileInfo.Type != TypeSSH {
		t.Fatalf("FindFileInfo did not resolve the SSH file entry: %+v", fileInfo)
	}
	if _, ok := state.FindFileInfo("/var/log/app.log", TypeSSH, "host-b", "ssh-a"); ok {
		t.Fatal("FindFileInfo matched SSH file with the wrong host")
	}
	if _, ok := state.FindFileInfo("/var/log/app.log", TypeDocker, "host-a", "ssh-a"); ok {
		t.Fatal("FindFileInfo matched file with the wrong type")
	}
}

func TestAPIHandler_GetRejectsWrongHost(t *testing.T) {
	e := echo.New()
	previous := SnapshotGlobalState()
	t.Cleanup(func() {
		setGlobalState(previous.FilePaths, previous.SSHConfigs)
	})

	setGlobalState([]FileInfo{
		{
			FilePath:   "/var/log/app.log",
			LinesCount: 4,
			FileSize:   0,
			Type:       TypeSSH,
			Host:       "host-a",
		},
	}, []SSHPathConfig{{Host: "host-a", Port: "22", User: "user", SourceID: "ssh-a"}})

	req := httptest.NewRequest(http.MethodGet, "/api?file_path=/var/log/app.log&type=ssh&host=host-b", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	resp := NewAPIHandler().Get(c)
	assert.Error(t, resp)
	// nolint: errorlint
	if he, ok := resp.(*echo.HTTPError); ok {
		assert.Equal(t, http.StatusNotFound, he.Code)
	} else {
		assert.Fail(t, "response is not an HTTP error")
	}
}

func TestAPIHandler_GetSelectsSSHConfigBySourceID(t *testing.T) {
	e := echo.New()
	previous := SnapshotGlobalState()
	t.Cleanup(func() {
		setGlobalState(previous.FilePaths, previous.SSHConfigs)
	})

	setGlobalState(
		[]FileInfo{
			{FilePath: "/var/log/app.log", Type: TypeSSH, Host: "same-host", SourceID: "ssh-a"},
			{FilePath: "/var/log/app.log", Type: TypeSSH, Host: "same-host", SourceID: "ssh-b"},
		},
		[]SSHPathConfig{
			{Host: "same-host", Port: "22", User: "user-a", SourceID: "ssh-a"},
			{Host: "same-host", Port: "2222", User: "user-b", SourceID: "ssh-b"},
		},
	)

	state := SnapshotGlobalState()
	fileInfo, ok := state.FindFileInfo("/var/log/app.log", TypeSSH, "same-host", "ssh-b")
	if !ok {
		t.Fatal("FindFileInfo did not find SSH file by source_id")
	}
	assert.Equal(t, "ssh-b", fileInfo.SourceID)

	sshConfig, ok := state.FindSSHConfig(fileInfo.SourceID, fileInfo.Host)
	if !ok {
		t.Fatal("FindSSHConfig did not find SSH config by source_id")
	}
	assert.Equal(t, "2222", sshConfig.Port)

	req := httptest.NewRequest(http.MethodGet, "/api?file_path=/var/log/app.log&type=ssh&host=same-host&source_id=ssh-missing", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	resp := NewAPIHandler().Get(c)
	assert.Error(t, resp)
	// nolint: errorlint
	if he, ok := resp.(*echo.HTTPError); ok {
		assert.Equal(t, http.StatusNotFound, he.Code)
	} else {
		assert.Fail(t, "response is not an HTTP error")
	}
}
