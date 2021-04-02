package influxdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

const (
	defaultOrg             = "wirelesstags"
	defaultBucket          = "wirelesstags"
	defaultRetentionPeriod = 24 * 365 * time.Hour

	defaultUsername = "admin"
	defaultPassword = "password1"

	tokenFileSuffix = ".token"
)

func readConfig(ctx context.Context, path string) (config, error) {
	f, err := os.Open(path)
	if err != nil {
		return config{}, err
	}
	defer f.Close()

	var cfg config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return config{}, fmt.Errorf("%s: %w", path, err)
	}

	// Set Org and Bucket and RetentionPeriod to defaults if they're not set.
	if cfg.Org == "" {
		cfg.Org = defaultOrg
	}
	if cfg.Bucket == "" {
		cfg.Bucket = defaultBucket
	}
	if cfg.RetentionPeriod == 0 {
		cfg.RetentionPeriod = defaultRetentionPeriod
	}

	// URL must be set.
	if cfg.URL == "" {
		return config{}, fmt.Errorf("%s: \"url\" is missing", path)
	}

	// If token is set, we're done!
	if cfg.Token != "" {
		return cfg, nil
	}

	tokenFile := path + ".token"

	// Try to load a token from an adjacent file, if it exists.
	if token, err := loadTokenFromFile(tokenFile); err == nil {
		cfg.Token = token
		return cfg, nil
	}

	// Try to do the initial setup of the database.
	if token, err := setupDBAndGetToken(ctx, cfg); err == nil {
		cfg.Token = token
		if err := createTokenFile(tokenFile, token); err != nil {
			log.Printf("warning: influxdb setup was successful, but token could not be saved: %v", err)
		}
		return cfg, nil
	} else {
		log.Printf("warning: %v", err)
	}

	return cfg, fmt.Errorf("token was not configured and setup cannot be performed")
}

type config struct {
	URL             string        `json:"url"`
	Username        string        `json:"username"`         // optional override of defaultUsername, if token is not present, this will be used to Setup() the DB or create a token.
	Password        string        `json:"password"`         // optional override of defaultPassword, if token is not present, this will be used to Setup() the DB or create a token.
	Token           string        `json:"token"`            // optional, if not set, try to load from an adjacent token file or try to Setup() the DB.
	Org             string        `json:"org"`              // optional override of defaultOrg
	Bucket          string        `json:"bucket"`           // optional override of defaultBucket
	RetentionPeriod time.Duration `json:"retention_period"` // optional override of defaultRetentionPeriod
}

func (c config) makeClient() influxdb2.Client {
	return influxdb2.NewClient(c.URL, c.Token)
}

func loadTokenFromFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func createTokenFile(path, token string) error {
	return ioutil.WriteFile(path, []byte(token), 0400)
}

func setupDBAndGetToken(ctx context.Context, cfg config) (string, error) {
	username := cfg.Username
	if username == "" {
		username = defaultUsername
	}
	password := cfg.Password
	if password == "" {
		password = defaultPassword
	}

	client := cfg.makeClient()

	// In docker-compose, it takes a few seconds for influxdb to start.
	for i := 0; i < 10; i++ {
		token, retryable, err := trySetupDBAndGetToken(ctx, client, username, password, cfg.Org, cfg.Bucket, cfg.RetentionPeriod)
		if err == nil {
			return token, nil
		}
		if !retryable {
			return "", err
		}
		log.Printf("warning: try %d/10: %v", i+1, err)
		time.Sleep(2 * time.Second)
	}

	token, _, err := trySetupDBAndGetToken(ctx, client, username, password, cfg.Org, cfg.Bucket, cfg.RetentionPeriod)
	return token, err
}

func trySetupDBAndGetToken(ctx context.Context, client influxdb2.Client, username, password, org, bucket string, retentionPeriod time.Duration) (string, bool, error) {
	onboarding, err := client.Setup(ctx, username, password, org, bucket, int(retentionPeriod))
	if err != nil {
		return "", true, err
	}
	token := onboarding.Auth.Token
	if token == nil {
		return "", false, fmt.Errorf("setup completed successfully, but token was missing")
	}
	return *token, false, nil
}
