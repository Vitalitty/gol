package pkg

import (
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
	req.FilePath = fileInfo.FilePath
	req.Type = fileInfo.Type
	req.Host = fileInfo.Host
	req.SourceID = fileInfo.SourceID

	var watcher *Watcher
	if req.Type == TypeDocker {
		if !strings.HasPrefix(req.FilePath, TmpContainerPath) {
			result, err := ContainerLogsFromFile(req.Host, req.Query, req.Ignore, req.FilePath, req.Page, req.PerPage, req.Reverse)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err)
			}
			result.Type = req.Type
			return c.JSON(http.StatusOK, APIResponse{
				Result:    *result,
				FilePaths: state.FilePaths,
			})
		}

		watcher, err = NewWatcher(req.FilePath, req.Query, req.Ignore, false, "", "", "", "", "")
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err)
		}
	}

	if req.Type == TypeSSH {
		sshConfig, ok := state.FindSSHConfig(req.SourceID, req.Host)
		if !ok {
			return echo.NewHTTPError(http.StatusNotFound, "ssh config not found")
		}
		config := sshConfig.toSSHConfig()
		watcher = newWatcher(req.FilePath, req.Query, req.Ignore, true, &config)
	}
	if req.Type == TypeFile || req.Type == TypeStdin {
		watcher, err = NewWatcher(req.FilePath, req.Query, req.Ignore, false, "", "", "", "", "")
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
	result.Type = req.Type
	result.Host = req.Host
	result.SourceID = req.SourceID

	return c.JSON(http.StatusOK, APIResponse{
		Result:    *result,
		FilePaths: state.FilePaths,
	})
}
