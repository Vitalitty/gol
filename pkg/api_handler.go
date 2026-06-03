package pkg

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/mcuadros/go-defaults"
)

type APIHandler struct {
	API *API
}
type FileInfo struct {
	FilePath   string `json:"file_path"`
	LinesCount int    `json:"lines_count"`
	FileSize   int64  `json:"file_size"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Host       string `json:"host"`
	SourceID   string `json:"source_id"`
}

func NewAPIHandler() *APIHandler {
	return &APIHandler{
		API: NewAPI(),
	}
}

type APIRequest struct {
	Query    string `json:"query" query:"query"`
	Ignore   string `json:"ignore" query:"ignore"`
	FilePath string `json:"file_path" query:"file_path"`
	Host     string `json:"host" query:"host"`
	SourceID string `json:"source_id" query:"source_id"`
	Type     string `json:"type" query:"type"`
	Page     int    `json:"page" query:"page" default:"1" validate:"required,gte=1" message:"page >=1 is required"`
	PerPage  int    `json:"per_page" query:"per_page" default:"15" validate:"required" message:"per_page is required"`
	Reverse  bool   `json:"reverse" query:"reverse" default:"false"`
}

type APIResponse struct {
	Result    ScanResult `json:"result"`
	FilePaths []FileInfo `json:"file_paths"`
}

func (h *APIHandler) Get(c echo.Context) error {
	req := new(APIRequest)
	if err := BindRequest(c, req); err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err)
	}
	defaults.SetDefaults(req)
	msgs, err := ValidateRequest(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, msgs)
	}

	state := SnapshotGlobalState()
	if len(state.FilePaths) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "filepath not found")
	}

	if req.FilePath == "" {
		first := state.FilePaths[0]
		req.FilePath = first.FilePath
		req.Host = first.Host
		req.SourceID = first.SourceID
		req.Type = first.Type
	}
	if req.FilePath != "" && req.Type == "" {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "type and host are required")
	}
	if (req.Type == TypeDocker || req.Type == TypeSSH) && req.Host == "" {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "type and host are required")
	}

	fileInfo, ok := state.FindFileInfo(req.FilePath, req.Type, req.Host, req.SourceID)
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "file not found")
	}

	var watcher *Watcher
	if fileInfo.Type == TypeDocker {
		if !strings.HasPrefix(fileInfo.FilePath, TmpContainerPath) {
			result, err := ContainerLogsFromFile(fileInfo.Host, req.Query, req.Ignore, fileInfo.FilePath, req.Page, req.PerPage, req.Reverse)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err)
			}
			result.Type = fileInfo.Type
			result.Host = fileInfo.Host
			result.SourceID = fileInfo.SourceID
			return c.JSON(http.StatusOK, APIResponse{
				Result:    *result,
				FilePaths: state.FilePaths,
			})
		}

		watcher, err = newWatcherFromFileInfo(fileInfo, req.Query, req.Ignore, nil)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err)
		}
	}

	if fileInfo.Type == TypeSSH {
		sshConfig, ok := state.FindSSHConfig(fileInfo.SourceID, fileInfo.Host)
		if !ok {
			return echo.NewHTTPError(http.StatusNotFound, "ssh config not found")
		}
		config := sshConfig.toSSHConfig()
		watcher, err = newWatcherFromFileInfo(fileInfo, req.Query, req.Ignore, &config)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err)
		}
	}
	if fileInfo.Type == TypeFile || fileInfo.Type == TypeStdin {
		watcher, err = newWatcherFromFileInfo(fileInfo, req.Query, req.Ignore, nil)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err)
		}
	}
	if watcher == nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "unsupported type")
	}

	result, err := watcher.Scan(req.Page, req.PerPage, req.Reverse)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}
	if result == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "scan result is empty")
	}
	result.Type = fileInfo.Type
	result.Host = fileInfo.Host
	result.SourceID = fileInfo.SourceID

	return c.JSON(http.StatusOK, APIResponse{
		Result:    *result,
		FilePaths: state.FilePaths,
	})
}

func newWatcherFromFileInfo(fileInfo FileInfo, query string, ignore string, sshConfig *SSHConfig) (*Watcher, error) {
	switch fileInfo.Type {
	case TypeSSH:
		if sshConfig == nil {
			return nil, fmt.Errorf("ssh config is required")
		}
		return newWatcher(fileInfo.FilePath, query, ignore, true, sshConfig), nil
	case TypeFile, TypeStdin, TypeDocker:
		return NewWatcher(fileInfo.FilePath, query, ignore, false, "", "", "", "", "")
	default:
		return nil, fmt.Errorf("unsupported type: %s", fileInfo.Type)
	}
}
