package jrpc

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	green   = "\033[97;42m"
	white   = "\033[90;47m"
	yellow  = "\033[90;43m"
	red     = "\033[97;41m"
	blue    = "\033[97;44m"
	magenta = "\033[97;45m"
	cyan    = "\033[97;46m"
	reset   = "\033[0m"
)

type ILog interface {

	// Errorf logs a message using ERROR as log level.
	Error(format string, args ...interface{})

	// Warningf logs a message using WARNING as log level.
	Warn(format string, args ...interface{})

	// Infof logs a message using INFO as log level.
	Info(format string, args ...interface{})

	// Debugf logs a message using DEBUG as log level.
	Debug(format string, args ...interface{})
}

//-----------------Logger------------------

type LogFormatterParams struct {
	Request   *http.Request
	RequestID string
	// TimeStamp shows the time after the server returns a response.
	TimeStamp time.Time
	// StatusCode is HTTP response code.
	StatusCode int
	// Latency is how much time the server cost to process a certain request.
	Latency time.Duration
	// ClientIP equals Context's ClientIP method.
	ClientIP string
	// Method is the RPC method given to the request.
	Method string
	// Params is the RPC request params.
	Params []byte
	// Path is a path the client requests.
	Path string
	// ErrorMessage is set if error has occurred in processing the request.
	ErrorMessage string
	// isTerm shows whether gin's output descriptor refers to a terminal.
	isTerm bool
	// BodySize is the size of the Response Body
	BodySize int
	// Keys are the keys set on the request's context.
	Keys map[string]any
}

type LogFormatter func(params LogFormatterParams) string

func (p *LogFormatterParams) StatusCodeColor() string {
	code := p.StatusCode
	switch {
	case code >= http.StatusOK && code < http.StatusMultipleChoices:
		return green
	case code >= http.StatusMultipleChoices && code < http.StatusBadRequest:
		return white
	case code >= http.StatusBadRequest && code < http.StatusInternalServerError:
		return yellow
	default:
		return red
	}
}
func (p *LogFormatterParams) ResetColor() string {
	return reset
}

var defaultLogFormatter = func(showParams bool) LogFormatter {
	return func(param LogFormatterParams) string {
		var statusColor, methodColor, resetColor string
		statusColor = param.StatusCodeColor()
		methodColor = magenta
		resetColor = param.ResetColor()

		if param.Latency > time.Minute {
			param.Latency = param.Latency.Truncate(time.Second)
		}
		if showParams {
			return fmt.Sprintf("[rpc]%v |%s %3d %s| %13v | %15s |%s|%s %-7s %s|%s\n",
				param.TimeStamp.Format("2006-01-02 15:04:05"),
				statusColor, param.StatusCode, resetColor,
				param.Latency,
				param.ClientIP,
				param.RequestID,
				methodColor, param.Method, resetColor, string(param.Params),
			)
		}
		return fmt.Sprintf("[rpc]%v |%s %3d %s| %13v | %15s |%s|%s %-7s %s\n",
			param.TimeStamp.Format("2006-01-02 15:04:05"),
			statusColor, param.StatusCode, resetColor,
			param.Latency,
			param.ClientIP,
			param.RequestID,
			methodColor, param.Method, resetColor,
		)
	}
}

func Logger(showParams bool) HandlerFunc {
	return loggerHandler(os.Stdout, defaultLogFormatter(showParams))
}

func loggerHandler(w io.Writer, formatter LogFormatter) HandlerFunc {
	return func(c *Context) {
		// Start timer
		start := time.Now()
		// Process request
		c.Next()

		param := LogFormatterParams{
			Request: c.HttpRequest,
		}
		// Stop timer
		param.TimeStamp = time.Now()
		param.Latency = param.TimeStamp.Sub(start)
		param.RequestID = c.RequestID()
		param.ClientIP = c.ClientIP()
		param.Method = c.ServiceName()
		param.StatusCode = c.Writer.Status()
		param.BodySize = c.Writer.Size()
		param.Params = c.RequestParams()
		fmt.Fprint(w, formatter(param))
	}
}
