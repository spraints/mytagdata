package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/spraints/mytagdata/pkg/data"
	"github.com/spraints/mytagdata/pkg/http/requestlogger"
	"github.com/spraints/mytagdata/pkg/updater"
	"github.com/spraints/mytagdata/pkg/updater/influxdb"
)

// Test usage:
// $ go run main.go -addr 127.0.0.1:8288
// $ curl -d '{"tag_name": "test", "degrees_c": 123.34}' http://127.0.0.1:8288/

func main() {
	addr := flag.String("addr", ":8900", "address for web server")
	influxConfig := flag.String("influx", "", "config file for influxdb")
	flag.Parse()

	var updaters updater.MultiUpdater
	if *influxConfig != "" {
		influx, err := influxdb.New(context.Background(), *influxConfig)
		if err != nil {
			log.Fatalf("error setting up influxdb: %v", err)
		}
		defer influx.Close()
		log.Printf("configured influxdb using %s", *influxConfig)
		updaters.Updaters = append(updaters.Updaters, influx)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("error reading request body: %v", err)
			http.Error(w, "unable to read request body", http.StatusInternalServerError)
			return
		}

		var update data.WirelessTagUpdate
		if err := json.NewDecoder(bytes.NewReader(body)).Decode(&update); err != nil {
			log.Printf("error: %v", err)
			log.Printf("request body (cl=%d, size=%d): %s", r.ContentLength, len(body), string(body))
			http.Error(w, "unable to parse request body", http.StatusUnprocessableEntity)
			return
		}
		log.Printf("update: %#v", update)

		if err := updaters.Update(r.Context(), update); err != nil {
			log.Printf("error: %v", err)
			http.Error(w, "unable to save data", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "OK! %#v\r\n", update)
	})

	log.Printf("Listening on %s", *addr)
	http.ListenAndServe(*addr, requestlogger.Handler(mux))
}
