package requestlogger

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func Handler(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reqData := reqLogData{
			method:     r.Method,
			url:        r.URL.String(),
			userAgent:  r.UserAgent(),
			remoteAddr: r.RemoteAddr,
			status:     http.StatusOK,
			start:      time.Now(),
		}
		defer log.Print(&reqData)

		h.ServeHTTP(newLogWriter(w, &reqData), r)
	}
}

type reqLogData struct {
	method       string
	url          string
	userAgent    string
	remoteAddr   string
	status       int
	start        time.Time
	headerSentAt time.Time
}

func (r *reqLogData) String() string {
	elapsed := ""
	if r.headerSentAt.After(r.start) {
		elapsed = fmt.Sprintf(" %0.3fs", r.headerSentAt.Sub(r.start).Seconds())
	}
	return fmt.Sprintf("%s - %s %s - %d%s",
		r.remoteAddr,
		r.method,
		r.url,
		r.status,
		elapsed)
}

func newRequestLogger(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
	}
}

func newLogWriter(w http.ResponseWriter, reqData *reqLogData) *loggingWriter {
	return &loggingWriter{w, reqData}
}

type loggingWriter struct {
	w http.ResponseWriter
	r *reqLogData
}

func (w *loggingWriter) Write(b []byte) (int, error) {
	if w.r.headerSentAt.IsZero() {
		w.r.headerSentAt = time.Now()
	}
	return w.w.Write(b)
}

func (w *loggingWriter) WriteHeader(status int) {
	w.r.status = status
	w.r.headerSentAt = time.Now()
	w.w.WriteHeader(status)
}

func (w *loggingWriter) Header() http.Header {
	return w.w.Header()
}
