package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Test usage:
// $ go run main.go -addr 127.0.0.1:8288
// $ curl -d '{"tag_name": "test", "degrees_c": 123.34}' http://127.0.0.1:8288/

func main() {
	addr := flag.String("addr", ":8900", "address for web server")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var u update
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			log.Printf("error: %v", err)
			http.Error(w, "unable to parse request body", http.StatusUnprocessableEntity)
			return
		}
		log.Printf("update: %#v", u)

		// TODO send to influxdb

		fmt.Fprintf(w, "OK! %#v\r\n", u)
	})

	log.Printf("Listening on %s", *addr)
	http.ListenAndServe(*addr, newRequestLogger(mux))
}

type update struct {
	Name      string  `json:"tag_name"`
	ID        string  `json:"tag_id"`
	DegreesC  float32 `json:"degrees_c"`
	Humidity  float32 `json:"humidity"`
	Battery   float32 `json:"battery"`
	Timestamp string  `json:"now"`
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
