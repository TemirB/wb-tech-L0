package config

import (
	"os"
	"strconv"
	"strings"
)

type kafka struct {
	Brokers []string
	Topic   string
	Group   string
}

type db struct {
	DSN string
}

type http struct {
	Addr string
}

type cache struct {
	Size int
}

type Config struct {
	Kafka kafka
	DB    db
	HTTP  http
	Cache cache
}

func LoadFromEnv() *Config {
	var cfg Config
	cfg.Kafka.Brokers = split(os.Getenv("KAFKA_BROKERS"), ",", []string{"localhost:9092"})
	cfg.Kafka.Topic = def(os.Getenv("KAFKA_TOPIC"), "orders")
	cfg.Kafka.Group = def(os.Getenv("KAFKA_GROUP"), "orders-consumer")

	cfg.DB.DSN = def(os.Getenv("DB_DSN"), "postgres://orders_user:orders_pass@localhost:5432/ordersdb?sslmode=disable")
	cfg.HTTP.Addr = def(os.Getenv("HTTP_ADDR"), ":8081")
	cfg.Cache.Size = atoi(def(os.Getenv("CACHE_SIZE"), "1000"))

	return &cfg
}

func def(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

func split(s, sep string, d []string) []string {
	if s == "" {
		return d
	}
	return strings.Split(s, sep)
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
