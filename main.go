package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

// Test usage:
// $ go run main.go -addr 127.0.0.1:8288
// $ curl -d '{"tag_name": "test", "degrees_c": 123.34}' http://127.0.0.1:8288/

func main() {
	addr := flag.String("addr", ":8900", "address for web server")
	influxURL := flag.String("influx", "", "HTTP address of influxdb")
	influxToken := flag.String("influx-token", "", "token for influxdb")
	influxOrg := flag.String("influx-org", "wirelesstags", "influx org to write to")
	influxBucket := flag.String("influx-bucket", "wirelesstags", "influx bucket to write to")
	flag.Parse()

	var updater multiUpdater
	if *influxURL != "" {
		influx := &influxDBUpdater{
			Client: influxdb2.NewClient(*influxURL, *influxToken),
			Org:    *influxOrg,
			Bucket: *influxBucket,
		}
		// retention period arg says its units are 'hours', but then it complains that 24*365 is <1h.
		if err := influx.Init(context.Background(), "admin", "p@ssword!", int(24*365*time.Hour)); err != nil {
			log.Fatalf("error setting up influxdb: %v", err)
		}
		defer influx.Close() // Note: this doesn't actually do anything. We'd need to handle SIGINT and SIGTERM in this app in order to make it work.
		updater.AddUpdater(influx)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var u update
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			log.Printf("error: %v", err)
			http.Error(w, "unable to parse request body", http.StatusUnprocessableEntity)
			return
		}
		log.Printf("update: %#v", u)

		if err := updater.Update(r.Context(), u); err != nil {
			log.Printf("error: %v", err)
			http.Error(w, "unable to save data", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "OK! %#v\r\n", u)
	})

	log.Printf("Listening on %s", *addr)
	http.ListenAndServe(*addr, newRequestLogger(mux))
}

type update struct {
	Name      string    `json:"tag_name"`
	ID        string    `json:"tag_id"`
	DegreesC  float64   `json:"degrees_c"`
	Humidity  float64   `json:"humidity"`
	Battery   float64   `json:"battery"`
	Timestamp time.Time `json:"now"`
}

type updater interface {
	Update(context.Context, update) error
}

type multiUpdater struct {
	updaters []updater
}

func (m *multiUpdater) AddUpdater(u updater) {
	m.updaters = append(m.updaters, u)
}

func (m *multiUpdater) Update(ctx context.Context, u update) error {
	for _, updater := range m.updaters {
		if err := updater.Update(ctx, u); err != nil {
			return err
		}
	}
	return nil
}

type influxDBUpdater struct {
	Client      influxdb2.Client
	Org, Bucket string

	initWriteAPI sync.Once
	writeAPI     api.WriteAPI
}

func (i *influxDBUpdater) Init(ctx context.Context, username, password string, retentionPeriodHours int) error {
	onboarding, err := i.Client.Setup(ctx, username, password, i.Org, i.Bucket, retentionPeriodHours)
	if err != nil {
		if strings.Contains(err.Error(), "conflict: onboarding has already been completed") {
			return nil
		}
		return err
	}

	token := onboarding.Auth.Token
	if token == nil {
		log.Printf("no token!")
	} else {
		log.Printf("new db token is %q", *token)
	}

	return nil
}

func (i *influxDBUpdater) Close() {
	if i.writeAPI != nil {
		log.Printf("flushing influxdb writes...")
		i.writeAPI.Flush()
	}
	log.Printf("stopping influxdb client...")
	i.Client.Close()
}

func (i *influxDBUpdater) Update(ctx context.Context, update update) error {
	tags := map[string]string{
		"tag_number": update.ID,
		"tag_name":   update.Name,
	}
	ts := update.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	if err := i.writePoint(ctx, "temperature", update.DegreesC, tags, ts); err != nil {
		return err
	}

	if err := i.writePoint(ctx, "humidity", update.Humidity, tags, ts); err != nil {
		return err
	}

	if err := i.writePoint(ctx, "battery_voltage", update.Battery, tags, ts); err != nil {
		return err
	}

	return nil
}

func (i *influxDBUpdater) writePoint(ctx context.Context, name string, value float64, tags map[string]string, ts time.Time) error {
	i.initWriteAPI.Do(func() {
		i.writeAPI = i.Client.WriteAPI(i.Org, i.Bucket)
	})

	point := influxdb2.NewPoint(name, tags, map[string]interface{}{"value": value}, ts)
	i.writeAPI.WritePoint(point)
	return nil
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
