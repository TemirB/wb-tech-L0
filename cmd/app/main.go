package main

import (
	postgres "github.com/TemirB/wb-tech-L0/internal/infrastructure/database"
	"github.com/TemirB/wb-tech-L0/internal/infrastructure/kafka"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func main() {
	// TODO: Конфиг
	// Логгер
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	// База
	database := postgres.NewOrderRepository(&pgxpool.Pool{})
	// Кафка
	kafka := kafka.NewConsumer(
		cfg.Kafka.Brokers, cfg.Kafka.Topic, cfg.Kafka.Group, kafka.Handler,
	)
	// Кэш
	// Сервис
	// Хендлер
}
