package httpapi

import (
	"log"
	"net/http"
	"time"
)

// statusWriter records the response status written by the wrapped handler so
// the log reflects what actually reached the client (including 401/404/503).
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// LogRequests logs one line per request: method, path, status, and duration.
// It deliberately omits query strings, headers, and bodies so credentials and
// other sensitive data never reach the logs.
func LogRequests(logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)

		status := sw.status
		if status == 0 {
			status = http.StatusOK
		}
		logger.Printf("%s %s %d %s", r.Method, r.URL.Path, status, time.Since(start))
	})
}
