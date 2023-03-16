package bodylogger

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Config struct {
	GinConfig *gin.LoggerConfig

	BodyLogFormatter Formatter
	// Log the request body or not
	WithBody bool
	// Only body of these methods can be logged, defaults to [POST, PATCH, PUT] when value is nil
	Methods map[string]struct{}
	// If true, the log will output after all other midllwares and the final func
	AfterRequest bool
}

type Formatter func(params *FormatterParams) string

type FormatterParams struct {
	ginParam *gin.LogFormatterParams

	RequestBody []byte
}

var defaultBodyLogFormatter = func(param *FormatterParams) string {
	var statusColor, methodColor, resetColor string
	if param.ginParam.IsOutputColor() {
		statusColor = param.ginParam.StatusCodeColor()
		methodColor = param.ginParam.MethodColor()
		resetColor = param.ginParam.ResetColor()
	}

	if param.ginParam.Latency > time.Minute {
		param.ginParam.Latency = param.ginParam.Latency.Truncate(time.Second)
	}
	baseLog := fmt.Sprintf("[GIN] %v |%s %3d %s| %13v | %15s |%s %-7s %s %#v\n%s",
		param.ginParam.TimeStamp.Format("2006/01/02 - 15:04:05"),
		statusColor, param.ginParam.StatusCode, resetColor,
		param.ginParam.Latency,
		param.ginParam.ClientIP,
		methodColor, param.ginParam.Method, resetColor, param.ginParam.Path,
		param.ginParam.ErrorMessage,
	)
	if len(param.RequestBody) > 0 {
		return baseLog + "\"" + string(param.RequestBody) + "\""
	}
	return baseLog
}

var beforeRequestFormatter = func(param *FormatterParams) string {
	var methodColor, resetColor string
	if param.ginParam.IsOutputColor() {
		methodColor = param.ginParam.MethodColor()
		resetColor = param.ginParam.ResetColor()
	}

	baseLog := fmt.Sprintf("[GIN] %v |%s %-7s %s %#v\n",
		param.ginParam.TimeStamp.Format("2006/01/02 - 15:04:05"),
		methodColor, param.ginParam.Method, resetColor,
		param.ginParam.Path,
	)
	if len(param.RequestBody) > 0 {
		return baseLog + "\"" + string(param.RequestBody) + "\""
	}
	return baseLog
}

func BodyLogger(withBody, AfterRequest bool, methods ...string) gin.HandlerFunc {
	if methods == nil {
		return WithConfig(&Config{
			GinConfig: &gin.LoggerConfig{}, WithBody: withBody, AfterRequest: AfterRequest,
		})
	}
	m := make(map[string]struct{})
	for _, method := range methods {
		m[method] = struct{}{}
	}
	return WithConfig(&Config{
		GinConfig: &gin.LoggerConfig{}, WithBody: withBody, Methods: m, AfterRequest: AfterRequest,
	})
}

func WithConfig(conf *Config) gin.HandlerFunc {
	formatter := conf.BodyLogFormatter
	if !conf.AfterRequest {
		formatter = beforeRequestFormatter
	}
	if formatter == nil {
		formatter = defaultBodyLogFormatter
	}
	if conf.Methods == nil {
		conf.Methods = map[string]struct{}{http.MethodPost: {}, http.MethodPatch: {}, http.MethodPut: {}}
	}

	out := conf.GinConfig.Output
	if out == nil {
		out = gin.DefaultWriter
	}

	notlogged := conf.GinConfig.SkipPaths

	var skip map[string]struct{}

	if length := len(notlogged); length > 0 {
		skip = make(map[string]struct{}, length)

		for _, path := range notlogged {
			skip[path] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if _, ok := skip[path]; ok {
			return
		}

		param := &FormatterParams{
			ginParam: &gin.LogFormatterParams{
				Request:  c.Request,
				Keys:     c.Keys,
				ClientIP: c.ClientIP(),
				Method:   c.Request.Method,
			},
		}
		// Start timer
		start := time.Now()
		raw := c.Request.URL.RawQuery

		if _, ok := conf.Methods[c.Request.Method]; conf.WithBody && ok {
			body, _ := ioutil.ReadAll(c.Request.Body)
			param.RequestBody = body
			// Because the HTTP request body can be read only once, we write back the request body with NopCloser
			c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		}

		param.ginParam.TimeStamp = start
		if conf.AfterRequest {
			// Process request
			c.Next()
			// Stop timer
			param.ginParam.TimeStamp = time.Now()
			param.ginParam.Latency = param.ginParam.TimeStamp.Sub(start)
			param.ginParam.StatusCode = c.Writer.Status()
			param.ginParam.ErrorMessage = c.Errors.ByType(gin.ErrorTypePrivate).String()
			param.ginParam.BodySize = c.Writer.Size()
		}

		if raw != "" {
			path = path + "?" + raw
		}

		param.ginParam.Path = path

		fmt.Fprint(out, formatter(param))
	}
}
