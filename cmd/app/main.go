package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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

	if err := kafka.EnsureTopic(ctx, cfg.Kafka.Brokers, cfg.Kafka.Topic, 1, 1, logger); err != nil {
		logger.Fatal("failed to ensure kafka topic", zap.Error(err))
	}

	if strings.TrimSpace(cfg.Kafka.Topic) == "" {
		logger.Fatal("KAFKA_TOPIC is empty")
	}

	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:               cfg.Kafka.Brokers,
		GroupID:               cfg.Kafka.Group,
		GroupTopics:           []string{cfg.Kafka.Topic},
		WatchPartitionChanges: true,

		StartOffset: kafkago.FirstOffset,
		MinBytes:    1,
		MaxBytes:    10e6,
		MaxWait:     10 * time.Second,

		// Logger:      log.New(os.Stdout, "kafka ", log.LstdFlags),
		// ErrorLogger: log.New(os.Stderr, "kafka ERR ", log.LstdFlags),
	})

	breaker := breaker.New(cfg.Breaker)
	service := service.NewService(cache, repo, logger)
	handler := handler.NewHandler(service, breaker, cfg.Retry, logger)

	consumer := kafka.New(handler, reader, logger)
	go consumer.Start(ctx, cfg.Retry)

	srv := httpapi.New(service, logger)
	go func() {
		if err := srv.ListenAndServe(ctx, cfg.HTTPAddr); err != nil {
			logger.Error("http stopped", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down...")

	if err := reader.Close(); err != nil {
		logger.Error("failed to close kafka reader", zap.Error(err))
	}
}
