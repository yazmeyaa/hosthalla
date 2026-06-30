package middlewares

import (
	"bufio"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type responseStatusWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseStatusWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseStatusWriter) Flush() {
	http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *responseStatusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}

func (w *responseStatusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func RequestLoggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		statusWriter := &responseStatusWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(statusWriter, r)

		logger.Info("http request completed",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", statusWriter.statusCode),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
	})
}
