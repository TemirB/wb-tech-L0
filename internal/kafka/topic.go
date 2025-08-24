package kafka

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// EnsureTopic гарантирует существование топика: если его нет — создаёт и ждёт,
// пока партиции станут видимы в метаданных. Идемпотентно и безопасно при конкурентных вызовах.
func EnsureTopic(ctx context.Context, brokers []string, topic string, numPartitions, replicationFactor int, log *zap.Logger) error {
	if len(brokers) == 0 {
		return fmt.Errorf("no kafka brokers configured")
	}
	if strings.TrimSpace(topic) == "" {
		return fmt.Errorf("empty topic")
	}

	dialer := &kafkago.Dialer{Timeout: 10 * time.Second}

	// Подключаемся к любому брокеру
	conn, err := dialer.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		return fmt.Errorf("dial broker: %w", err)
	}
	defer conn.Close()

	// Если топик уже есть — выходим
	if parts, err := conn.ReadPartitions(topic); err == nil && len(parts) > 0 {
		log.Info("kafka topic exists", zap.String("topic", topic), zap.Int("partitions", len(parts)))
		return nil
	}

	// Находим контроллер и создаём топик там
	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("get controller: %w", err)
	}
	ctrlAddr := net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port))

	ctrlConn, err := dialer.DialContext(ctx, "tcp", ctrlAddr)
	if err != nil {
		return fmt.Errorf("dial controller %s: %w", ctrlAddr, err)
	}
	defer ctrlConn.Close()

	log.Info("creating kafka topic (if not exists)",
		zap.String("topic", topic),
		zap.Int("partitions", numPartitions),
		zap.Int("replication", replicationFactor),
	)
	err = ctrlConn.CreateTopics(kafkago.TopicConfig{
		Topic:             topic,
		NumPartitions:     numPartitions,
		ReplicationFactor: replicationFactor,
	})
	// Если топик уже создан кем-то другим — это не ошибка
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "exists") {
		return fmt.Errorf("create topic: %w", err)
	}

	// Ждём, пока партиции появятся (до 10с)
	deadline := time.Now().Add(10 * time.Second)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		parts, err := conn.ReadPartitions(topic)
		if err == nil && len(parts) >= numPartitions {
			log.Info("kafka topic is ready", zap.String("topic", topic), zap.Int("partitions", len(parts)))
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("topic %s not visible after creation", topic)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
