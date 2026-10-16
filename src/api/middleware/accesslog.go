package middleware

import (
	"net/http"
	"time"

	"blob/src/lib/logger"

	"github.com/fatih/color"
)

// statusWriter captures the response status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// AccessLog mirrors the api request log: colored method and status, cyan URL.
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		msg := "%s <cyan>%s</> %s <dim>%s</dim>"
		logger.HTTP.Info(msg,
			method(r.Method),
			r.URL.RequestURI(),
			logger.StatusCode(sw.status),
			time.Since(start).Round(time.Microsecond).String(),
		)
	})
}

func method(m string) string {
	attr := color.FgCyan

	switch m {
	case "GET":
		attr = color.FgGreen
	case "POST":
		attr = color.FgYellow
	case "PUT":
		attr = color.FgBlue
	case "PATCH":
		attr = color.FgMagenta
	case "DELETE":
		attr = color.FgRed
	}

	return color.New(attr, color.Bold).Sprint(m)
}
