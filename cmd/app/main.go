package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/TemirB/wb-tech-L0/internal/application/handler"
	"github.com/TemirB/wb-tech-L0/internal/application/service"
	"github.com/TemirB/wb-tech-L0/internal/cache"
	"github.com/TemirB/wb-tech-L0/internal/config"
	"github.com/TemirB/wb-tech-L0/internal/database"
	"github.com/TemirB/wb-tech-L0/internal/httpapi"
	"github.com/TemirB/wb-tech-L0/internal/kafka"
	"github.com/TemirB/wb-tech-L0/internal/pkg/breaker"
	kafkago "github.com/segmentio/kafka-go"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}

	pool := database.Connect(ctx, cfg.DSN())
	repo := database.New(pool, cfg.Tables)
	cache, err := cache.New(cfg.CacheCap)
	if err != nil {
		panic(err)
	}

	// Warm
	if ids, err := repo.RecentOrderIDs(ctx, cfg.CacheCap); err == nil {
		for _, id := range ids {
			if o, err := repo.GetByUID(ctx, id); err == nil {
				cache.Set(o)
			}
		}
	}

	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers: cfg.Kafka.Brokers,
		Topic:   cfg.Kafka.Topic,
		GroupID: cfg.Kafka.Group,
		// StartOffset: kafkago.FirstOffset,
		MinBytes: 10e3,
		MaxBytes: 10e6,
		// RebalanceTimeout:  60 * time.Second,
		// SessionTimeout:    45 * time.Second,
		// HeartbeatInterval: 3 * time.Second,
		// MaxWait:           10 * time.Second,
		// ReadBackoffMin:    100 * time.Millisecond,
		// ReadBackoffMax:    1 * time.Second,
		// CommitInterval:    time.Second,
	})
	breaker := breaker.New(cfg.Breaker)
	service := service.NewService(cache, repo, logger)
	handler := handler.NewHandler(service, reader, breaker, logger)

	consumer := kafka.New(handler, reader, logger)
	consumer.Start(ctx, cfg.Retry)

	srv := httpapi.New(service, logger)
	if err := srv.ListenAndServe(ctx, cfg.HTTPAddr); err != nil {
		log.Printf("http stopped: %v", err)
	}
}
