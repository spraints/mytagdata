package influxdb

import (
	"context"
	"log"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"

	"github.com/spraints/mytagdata/pkg/data"
)

func New(ctx context.Context, configFile string) (*Updater, error) {
	cfg, err := readConfig(ctx, configFile)
	if err != nil {
		return nil, err
	}

	client := influxdb2.NewClient(cfg.URL, cfg.Token)
	writeAPI := client.WriteAPI(cfg.Org, cfg.Bucket)

	return &Updater{
		client:   client,
		writeAPI: writeAPI,
	}, nil
}

type Updater struct {
	client   influxdb2.Client
	writeAPI api.WriteAPI
}

func (u *Updater) Close() {
	if u.writeAPI != nil {
		log.Printf("flushing influxdb writes...")
		u.writeAPI.Flush()
	}
	log.Printf("stopping influxdb client...")
	u.client.Close()
}

func (u *Updater) Update(ctx context.Context, update data.WirelessTagUpdate) error {
	tags := map[string]string{
		"tag_number": update.ID,
		"tag_name":   update.Name,
	}
	ts := update.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	u.writePoint(ctx, "temperature", update.DegreesC, tags, ts)
	u.writePoint(ctx, "humidity", update.Humidity, tags, ts)
	u.writePoint(ctx, "battery_voltage", update.Battery, tags, ts)

	return nil
}

func (u *Updater) writePoint(ctx context.Context, name string, value float64, tags map[string]string, ts time.Time) {
	point := influxdb2.NewPoint(name, tags, map[string]interface{}{"value": value}, ts)
	u.writeAPI.WritePoint(point)
}
