package config

import (
	"log"
	"net"
	"net/url"
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

// Load keeps the original API and fatals on error for simplicity in main().
func Load() Config {
	cfg, err := load()
	if err != nil {
		log.Fatalf("config load error: %v", err)
	}
	return cfg
}

func load() (Config, error) {
	_ = godotenv.Load("env/.env")

	cfg := Config{
		HTTPAddr: envDefault("HTTP_ADDR", ":8081"),
		CacheCap: envInt("CACHE_CAP", 1000),

		Pg: Postgres{
			Host:     strings.TrimSpace(os.Getenv("PG_HOST")),
			Port:     strings.TrimSpace(envDefault("PG_PORT", "5432")),
			DB:       strings.TrimSpace(os.Getenv("PG_DB")),
			User:     strings.TrimSpace(os.Getenv("PG_USER")),
			Password: strings.TrimSpace(os.Getenv("PG_PASSWORD")),
			SSLMode:  strings.TrimSpace(envDefault("PG_SSLMODE", "disable")),
		},

		Tables: Tables{
			Schema:   strings.TrimSpace(os.Getenv("DB_SCHEMA")),
			Order:    strings.TrimSpace(os.Getenv("TBL_ORDER")),
			Delivery: strings.TrimSpace(os.Getenv("TBL_DELIVERY")),
			Payment:  strings.TrimSpace(os.Getenv("TBL_PAYMENT")),
			Item:     strings.TrimSpace(os.Getenv("TBL_ITEM")),
		},

		Kafka: Kafka{
			Brokers: splitCSV(strings.TrimSpace(os.Getenv("KAFKA_BROKERS"))),
			Topic:   strings.TrimSpace(os.Getenv("KAFKA_TOPIC")),
			Group:   strings.TrimSpace(os.Getenv("KAFKA_GROUP")),
			Workers: envInt("KAFKA_WORKERS", 10),
		},

		Breaker: Breaker{
			Threshold:   envUint32("BREAKER_THRESHOLD", 5),
			OpenTimeout: envDurationMS("BREAKER_OPENTIMEOUT", 10*time.Second),
			MaxHalfOpen: envUint32("BREAKER_MAXHALFOPEN", 3),
		},

		Retry: Retry{
			Attempts:     envInt("RETRY_ATTEMPTS", 5),
			Base:         envDurationMS("RETRY_BASE", 100*time.Millisecond),
			Max:          envDurationMS("RETRY_MAX", 5*time.Second),
			JitterFactor: envFloat64("RETRY_JITTERFACTOR", 0.3),
		},
	}

	// Validate required envs and basic sanity.
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	var missing []string
	req := map[string]string{
		"PG_HOST":       c.Pg.Host,
		"PG_DB":         c.Pg.DB,
		"PG_USER":       c.Pg.User,
		"PG_PASSWORD":   c.Pg.Password,
		"DB_SCHEMA":     c.Tables.Schema,
		"TBL_ORDER":     c.Tables.Order,
		"TBL_DELIVERY":  c.Tables.Delivery,
		"TBL_PAYMENT":   c.Tables.Payment,
		"TBL_ITEM":      c.Tables.Item,
		"KAFKA_BROKERS": strings.Join(c.Kafka.Brokers, ","),
		"KAFKA_TOPIC":   c.Kafka.Topic,
		"KAFKA_GROUP":   c.Kafka.Group,
	}
	for k, v := range req {
		if strings.TrimSpace(v) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return &missingEnvError{Keys: missing}
	}

	if c.CacheCap <= 0 {
		log.Printf("CACHE_CAP is %d, adjusting to 1", c.CacheCap)
	}
	if c.Retry.Attempts < 0 {
		log.Printf("RETRY_ATTEMPTS is %d, adjusting to 0", c.Retry.Attempts)
	}
	if c.Retry.Base <= 0 {
		log.Printf("RETRY_BASE is %v, adjusting to 100ms", c.Retry.Base)
	}
	if c.Retry.Max < c.Retry.Base {
		log.Printf("RETRY_MAX (%v) < RETRY_BASE (%v), adjusting max to base", c.Retry.Max, c.Retry.Base)
	}
	if len(c.Kafka.Brokers) == 0 {
		return &missingEnvError{Keys: []string{"KAFKA_BROKERS"}}
	}
	return nil
}

type missingEnvError struct{ Keys []string }

func (e *missingEnvError) Error() string {
	return "missing required envs: " + strings.Join(e.Keys, ", ")
}

// DSN builds a proper Postgres URL, safely escaping user/pass and query.
func (c Config) DSN() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.Pg.User, c.Pg.Password),
		Host:   net.JoinHostPort(c.Pg.Host, c.Pg.Port),
		Path:   "/" + c.Pg.DB,
	}
	q := url.Values{}
	if c.Pg.SSLMode != "" {
		q.Set("sslmode", c.Pg.SSLMode)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func envDefault(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("invalid %s=%q, using default %d: %v", k, v, def, err)
		return def
	}
	return n
}

func envUint32(k string, def uint32) uint32 {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	u, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		log.Printf("invalid %s=%q, using default %d: %v", k, v, def, err)
		return def
	}
	return uint32(u)
}

func envFloat64(k string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		log.Printf("invalid %s=%q, using default %.3f: %v", k, v, def, err)
		return def
	}
	return f
}

// envDurationMS supports either plain integer milliseconds ("1500") or
// Go duration strings ("1.5s", "250ms", "2m").
func envDurationMS(k string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	// If it looks like a duration with units, try ParseDuration first.
	if strings.IndexFunc(v, func(r rune) bool { return r < '0' || r > '9' }) != -1 {
		d, err := time.ParseDuration(v)
		if err != nil {
			log.Printf("invalid %s=%q, using default %v: %v", k, v, def, err)
			return def
		}
		return d
	}
	// Otherwise treat as milliseconds.
	ms, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("invalid %s=%q, using default %v: %v", k, v, def, err)
		return def
	}
	return time.Duration(ms) * time.Millisecond
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
