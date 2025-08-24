package kafka

import (
	"context"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader *kafka.Reader
}

func NewConsumer(brokers []string, topic, groupID string) *Consumer {
	// Читаем с самого начала топика
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		// GroupID:     groupID,
		StartOffset: kafka.FirstOffset, // <<< важная часть!
		MinBytes:    1,                 // минимальный размер сообщения
		MaxBytes:    10e6,              // максимальный размер сообщения
	})

	return &Consumer{reader: r}
}

func (c *Consumer) Start(ctx context.Context, handler func([]byte) error) {
	log.Println("Kafka consumer started...")
	for {
		m, err := c.reader.FetchMessage(ctx) // блокирующий вызов
		if err != nil {
			log.Printf("consumer fetch error: %v", err)
			time.Sleep(time.Second)
			continue
		}

		log.Printf("got message: topic=%s partition=%d offset=%d key=%s",
			m.Topic, m.Partition, m.Offset, string(m.Key))

		// обработка
		if err := handler(m.Value); err != nil {
			log.Printf("handler error: %v", err)
		}

		// коммитим, чтобы Kafka знала, что это сообщение прочитано
		if err := c.reader.CommitMessages(ctx, m); err != nil {
			log.Printf("commit error: %v", err)
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
