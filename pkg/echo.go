package pkg

import (
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type EchoOptions struct {
	Host      string
	Port      int64
	Cors      int64
	BaseURL   string
	Access    bool
	PublicDir *embed.FS
}

type EchoOption func(*EchoOptions) error

func NewEcho(opts ...EchoOption) error {
	options := &EchoOptions{
		Cors:      0,
		BaseURL:   "/",
		Host:      "localhost", // default host
		Port:      3000,        // default port
		Access:    false,
		PublicDir: nil,
	}
	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return err
		}
	}
	e := echo.New()

	SetupMiddlewares(e)
	if options.Access {
		e.Use(middleware.RequestLogger())
	}
	SetupRoutes(e, options)
	SetupCors(e, options)

	e.Logger.Fatal(e.Start(fmt.Sprintf("%s:%d", options.Host, options.Port)))
	return nil
}

func SetupMiddlewares(e *echo.Echo) {
	e.HTTPErrorHandler = HTTPErrorHandler
	e.Use(middleware.Recover())
	e.Use(middleware.Gzip())
	e.Pre(middleware.RemoveTrailingSlash())
	e.Use(requestLoggerWithLTSV())
}

func SetupRoutes(e *echo.Echo, options *EchoOptions) {
	e.GET(options.BaseURL+"", NewAssetsHandler(options.PublicDir, "dist", "index.html").Get)
	e.GET(options.BaseURL+"favicon.ico", NewAssetsHandler(options.PublicDir, "dist", "favicon.ico").GetICO)
	e.GET(options.BaseURL+"api", NewAPIHandler().Get)
}

func SetupCors(e *echo.Echo, options *EchoOptions) {
	if options.Cors == 0 {
		return
	}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{fmt.Sprintf("http://localhost:%d", options.Cors)},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))
}

// HTTPErrorResponse is the response for HTTP errors
type HTTPErrorResponse struct {
	Error interface{} `json:"error"`
}

// HTTPErrorHandler handles HTTP errors for entire application
func HTTPErrorHandler(err error, c echo.Context) {
	SetHeadersResponseJSON(c.Response().Header())
	code := http.StatusInternalServerError
	var message interface{}
	// nolint: errorlint
	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		message = he.Message
	} else {
		message = err.Error()
	}

	if code == http.StatusInternalServerError {
		message = fmt.Sprintf("%v", err)
	}
	if err = c.JSON(code, &HTTPErrorResponse{Error: message}); err != nil {
		slog.Error("handling HTTP error", "error", err)
	}
}

func requestLoggerWithLTSV() echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogLatency:      true,
		LogRemoteIP:     true,
		LogHost:         true,
		LogMethod:       true,
		LogURI:          true,
		LogRequestID:    true,
		LogReferer:      true,
		LogUserAgent:    true,
		LogStatus:       true,
		LogError:        true,
		LogResponseSize: true,
		LogHeaders:      []string{echo.HeaderXForwardedFor},
		HandleError:     true,
		LogValuesFunc: func(_ echo.Context, v middleware.RequestLoggerValues) error {
			forwardedFor := ""
			if values := v.Headers[echo.HeaderXForwardedFor]; len(values) > 0 {
				forwardedFor = values[0]
			}

			fmt.Printf(
				"time:%s\thost:%s\tforwardedfor:%s\treq:-\tstatus:%d\tmethod:%s\turi:%s\tsize:%d\treferer:%s\tua:%s\treqtime_ns:%d\tcache:-\truntime:-\tapptime:-\tvhost:%s\treqtime_human:%s\tx-request-id:%s\thost:%s\n",
				time.Now().Format("2006-01-02 15:04:05"),
				v.RemoteIP,
				forwardedFor,
				v.Status,
				v.Method,
				v.URI,
				v.ResponseSize,
				v.Referer,
				v.UserAgent,
				v.Latency.Nanoseconds(),
				v.Host,
				v.Latency.String(),
				v.RequestID,
				v.Host,
			)
			return nil
		},
	})
}
