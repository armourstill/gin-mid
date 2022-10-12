package bodylogger

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/gin-gonic/gin"
)

type Config struct {
	gin.LoggerConfig

	BodyLogFormatter Formatter
	Request          bool
	BeforeRequest    bool
}

type Formatter func(params FormatterParams) string

type FormatterParams struct {
	gin.LogFormatterParams

	RequestBody []byte
}

var defaultBodyLogFormatter = func(param FormatterParams) string {
	var statusColor, methodColor, resetColor string
	if param.IsOutputColor() {
		statusColor = param.StatusCodeColor()
		methodColor = param.MethodColor()
		resetColor = param.ResetColor()
	}

	if param.Latency > time.Minute {
		param.Latency = param.Latency.Truncate(time.Second)
	}
	baseLog := fmt.Sprintf("[GIN] %v |%s %3d %s| %13v | %15s |%s %-7s %s %#v\n%s",
		param.TimeStamp.Format("2006/01/02 - 15:04:05"),
		statusColor, param.StatusCode, resetColor,
		param.Latency,
		param.ClientIP,
		methodColor, param.Method, resetColor,
		param.Path,
		param.ErrorMessage,
	)
	if len(param.RequestBody) > 0 {
		return baseLog + "\n" + string(param.RequestBody)
	}
	return baseLog
}

var beforeRequestFormatter = func(param FormatterParams) string {
	var statusColor, methodColor, resetColor string
	if param.IsOutputColor() {
		statusColor = param.StatusCodeColor()
		methodColor = param.MethodColor()
		resetColor = param.ResetColor()
	}

	baseLog := fmt.Sprintf("[GIN] %v |%s %3d %s| %15s |%s %-7s %s\n%s",
		param.TimeStamp.Format("2006/01/02 - 15:04:05"),
		statusColor, param.StatusCode, resetColor,
		param.ClientIP,
		methodColor, param.Method, resetColor,
		param.Path,
	)
	if len(param.RequestBody) > 0 {
		return baseLog + "\n" + string(param.RequestBody)
	}
	return baseLog
}

func BodyLogger(request, beforeRequest bool) gin.HandlerFunc {
	return WithConfig(Config{
		LoggerConfig: gin.LoggerConfig{}, Request: request, BeforeRequest: beforeRequest,
	})
}

func LoggerWithWriter(request, beforeRequest bool, out io.Writer, notlogged ...string) gin.HandlerFunc {
	return WithConfig(Config{
		LoggerConfig: gin.LoggerConfig{Output: out, SkipPaths: notlogged}, Request: request, BeforeRequest: beforeRequest,
	})
}

func WithConfig(conf Config) gin.HandlerFunc {
	formatter := conf.BodyLogFormatter
	if formatter == nil {
		formatter = defaultBodyLogFormatter
	}
	if conf.BeforeRequest {
		formatter = beforeRequestFormatter
	}

	out := conf.Output
	if out == nil {
		out = gin.DefaultWriter
	}

	notlogged := conf.SkipPaths

	var skip map[string]struct{}

	if length := len(notlogged); length > 0 {
		skip = make(map[string]struct{}, length)

		for _, path := range notlogged {
			skip[path] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if _, ok := skip[path]; !ok {
			return
		}

		param := FormatterParams{
			LogFormatterParams: gin.LogFormatterParams{
				Request:  c.Request,
				Keys:     c.Keys,
				ClientIP: c.ClientIP(),
				Method:   c.Request.Method,
			},
		}
		// Start timer
		start := time.Now()
		raw := c.Request.URL.RawQuery

		if conf.Request {
			body, _ := ioutil.ReadAll(c.Request.Body)
			param.RequestBody = body
			// Because the HTTP request body can be read only once, we write back the request body with NopCloser
			c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		}

		if !conf.BeforeRequest {
			// Process request
			c.Next()
			// Stop timer
			param.TimeStamp = time.Now()
			param.Latency = param.TimeStamp.Sub(start)

			param.StatusCode = c.Writer.Status()
			param.ErrorMessage = c.Errors.ByType(gin.ErrorTypePrivate).String()
			param.BodySize = c.Writer.Size()
		} else {
			param.TimeStamp = start
		}

		if raw != "" {
			path = path + "?" + raw
		}

		param.Path = path

		fmt.Fprint(out, formatter(param))
	}
}
