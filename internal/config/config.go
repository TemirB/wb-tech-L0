package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Tables struct {
	Schema   string
	Order    string
	Delivery string
	Payment  string
	Item     string
}

type Kafka struct {
	Brokers []string
	Topic   string
	Group   string
	Workers int
}

type Postgres struct {
	Host     string
	Port     string
	DB       string
	User     string
	Password string
	SSLMode  string
}

type Breaker struct {
	Threshold   uint32
	OpenTimeout time.Duration
	MaxHalfOpen uint32
}

type Retry struct {
	Attempts     int
	Base         time.Duration
	Max          time.Duration
	JitterFactor float64
}

type Config struct {
	HTTPAddr string
	CacheCap int

	Pg      Postgres
	Tables  Tables
	Kafka   Kafka
	Breaker Breaker
	Retry   Retry
}

func get(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
func must(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("missing env %s", k)
	}
	return v
}

func Load() Config {
	_ = godotenv.Load("env/.env")

	capacity := 1000
	if s := os.Getenv("CACHE_CAP"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			capacity = n
		}
	}

	workers := 10
	if s := os.Getenv("KAFKA_WORKERS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			workers = n
		}
	}

	// Breaker
	var threshold uint32 = 5
	if s := os.Getenv("BREAKER_THRESHOLD"); s != "" {
		if n, err := strconv.ParseUint(s, 10, 32); err == nil {
			threshold = uint32(n)
		}
	}
	openTimeout := 10000 // milliseconds
	if s := os.Getenv("BREAKER_OPENTIMEOUT"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			openTimeout = n
		}
	}
	var maxHalfOpen uint32 = 3
	if s := os.Getenv("BREAKER_MAXHALFOPEN"); s != "" {
		if n, err := strconv.ParseUint(s, 10, 32); err == nil {
			maxHalfOpen = uint32(n)
		}
	}

	// Retry
	attempts := 5
	if s := os.Getenv("RETRY_ATTEMPTS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			attempts = n
		}
	}
	base := 100 // milliseconds
	if s := os.Getenv("RETRY_BASE"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			base = n
		}
	}
	max := 5000 // milliseconds
	if s := os.Getenv("RETRY_MAX"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			max = n
		}
	}
	jitterFactor := 0.3
	if s := os.Getenv("RETRY_JITTERFACTOR"); s != "" {
		if n, err := strconv.ParseFloat(s, 64); err == nil {
			jitterFactor = n
		}
	}

	cfg := Config{
		HTTPAddr: get("HTTP_ADDR", ":8081"),
		CacheCap: capacity,

		Pg: Postgres{
			Host:     must("PG_HOST"),
			Port:     get("PG_PORT", "5432"),
			DB:       must("PG_DB"),
			User:     must("PG_USER"),
			Password: must("PG_PASSWORD"),
			SSLMode:  get("PG_SSLMODE", "disable"),
		},

		Tables: Tables{
			Schema:   must("DB_SCHEMA"),
			Order:    must("TBL_ORDER"),
			Delivery: must("TBL_DELIVERY"),
			Payment:  must("TBL_PAYMENT"),
			Item:     must("TBL_ITEM"),
		},

		Kafka: Kafka{
			Brokers: splitCSV(must("KAFKA_BROKERS")),
			Topic:   must("KAFKA_TOPIC"),
			Group:   must("KAFKA_GROUP"),
			Workers: workers,
		},

		Breaker: Breaker{
			Threshold:   threshold,
			OpenTimeout: time.Duration(openTimeout) * time.Millisecond,
			MaxHalfOpen: maxHalfOpen,
		},

		Retry: Retry{
			Attempts:     attempts,
			Base:         time.Duration(base) * time.Millisecond,
			Max:          time.Duration(max) * time.Millisecond,
			JitterFactor: jitterFactor,
		},
	}
	return cfg
}

func (c Config) DSN() string {
	return "postgres://" + c.Pg.User + ":" + c.Pg.Password + "@" + c.Pg.Host + ":" + c.Pg.Port + "/" + c.Pg.DB + "?sslmode=" + c.Pg.SSLMode
}

func splitCSV(s string) []string {
	ps := strings.Split(s, ",")
	for i := range ps {
		ps[i] = strings.TrimSpace(ps[i])
	}
	return ps
}
