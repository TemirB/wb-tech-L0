package kafka

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// MessageHandler — минимальный интерфейс, который мы ожидаем от твоего обработчика.
type MessageHandler interface {
	Handle(ctx context.Context, msg kafkago.Message) error
}

// Consumer инкапсулирует чтение из kafka-go Reader + worker pool обработки.
type Consumer struct {
	handler MessageHandler
	reader  *kafkago.Reader
	zlogger *zap.Logger

	workers int          // размер пула воркеров
	jobs    chan jobItem // очередь задач для воркеров
}

type jobItem struct {
	msg    kafkago.Message
	result chan error // канал-фиксация завершения конкретного сообщения
}

// New — конструктор консьюмера.
func New(handler MessageHandler, reader *kafkago.Reader, logger *zap.Logger) *Consumer {
	// workers по умолчанию: из env KAFKA_WORKERS или 4
	workers := 4
	if s := os.Getenv("KAFKA_WORKERS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			workers = n
		}
	}
	return &Consumer{
		handler: handler,
		reader:  reader,
		zlogger: logger,
		workers: workers,
		// буфер небольшой, чтобы не раздувать память; воркеры успеют разгребать
		jobs: make(chan jobItem, workers*2),
	}
}

// Start — запускает основной цикл потребления.
// Параметр retryCfg специально типом any: бизнес-ретраи уже внутри handler/circuit-breaker.
// Если понадобятся ретраи на уровне консьюмера — см. пометки // RETRY-HOOK ниже.
func (c *Consumer) Start(ctx context.Context, retryCfg any) {
	rc := c.reader.Config()
	c.zlogger.Info("Starting Kafka consumer",
		zap.Strings("brokers", rc.Brokers),
		zap.String("group", rc.GroupID),
		zap.String("topic", rc.Topic),              // в group-режиме будет пустым — это норм
		zap.Strings("group_topic", rc.GroupTopics), // здесь должен быть твой топик
	)

	// Стартуем пул воркеров
	for i := 0; i < c.workers; i++ {
		go c.worker(ctx, i)
	}

	// Основной fetch-цикл. Очень важно: мы отправляем задачу воркерам и
	// ЖДЁМ результат по per-message каналу. Это гарантирует, что коммит
	// оффсета идёт в порядке получения сообщений, т.е. без «перепрыгивания».
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		c.zlogger.Debug("Attempting to fetch message from Kafka")
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				// выходим по контексту
				return
			}
			if isBenignFetchTimeout(err) {
				c.zlogger.Debug("fetch timeout (idle), backing off", zap.Error(err))
				sleepWithContext(ctx, 10*time.Second)
				continue
			}

			// Частые временные ошибки при ребалансах/координаторе — просто подождём и продолжим
			c.zlogger.Warn("FetchMessage error, backing off", zap.Error(err))
			sleepWithContext(ctx, 500*time.Millisecond)
			continue
		}

		// Отдаём сообщение воркерам и ждём конкретно его завершения.
		done := make(chan error, 1)
		select {
		case c.jobs <- jobItem{msg: msg, result: done}:
		case <-ctx.Done():
			return
		}

		var procErr error
		select {
		case procErr = <-done:
		case <-ctx.Done():
			return
		}

		if procErr != nil {
			// Здесь ты можешь принять решение о локальном ретрае на уровне консьюмера.
			// Сейчас — ЛОГИРУЕМ и НЕ коммитим, чтобы сообщение читалось снова этой же группой.
			// RETRY-HOOK: если хочешь «жёстких» ретраев здесь — добавь цикл с backoff и т.п.
			c.zlogger.Error("handler failed; message will not be committed", zap.Error(procErr),
				zap.String("topic", msg.Topic), zap.Int("partition", msg.Partition), zap.Int64("offset", msg.Offset))
			// Небольшая задержка, чтобы не крутить проблему слишком агрессивно:
			sleepWithContext(ctx, 200*time.Millisecond)
			continue
		}

		// Обработано успешно — коммитим ЭТО сообщение (в порядке приёма).
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			// Если коммит не удался — мы залогируем и сделаем бэкофф.
			// На практике kafka-go сам переотправит при следующих попытках.
			c.zlogger.Warn("commit failed", zap.Error(err),
				zap.String("topic", msg.Topic), zap.Int("partition", msg.Partition), zap.Int64("offset", msg.Offset))
			sleepWithContext(ctx, 200*time.Millisecond)
			continue
		}
		c.zlogger.Debug("message committed",
			zap.String("topic", msg.Topic), zap.Int("partition", msg.Partition), zap.Int64("offset", msg.Offset))
	}
}

// worker — воркер обработки сообщений.
// Он получает конкретный msg и обязан обязательно отправить результат в result-канал.
func (c *Consumer) worker(ctx context.Context, id int) {
	logPrefix := fmt.Sprintf("worker-%d", id)
	stdlog := log.New(os.Stdout, logPrefix+" ", log.LstdFlags)

	for {
		select {
		case <-ctx.Done():
			return
		case it := <-c.jobs:
			// Защита от случайного закрытия канала
			if it.result == nil {
				continue
			}

			msg := it.msg
			start := time.Now()

			// Вызов бизнес-обработчика.
			// Если он оборачивает ретраи/брейкер — отлично. Тогда мы просто проксируем ошибку наружу.
			err := c.handler.Handle(ctx, msg)

			elapsed := time.Since(start)
			if err != nil {
				c.zlogger.Error("message handling failed",
					zap.Error(err),
					zap.String("topic", msg.Topic),
					zap.Int("partition", msg.Partition),
					zap.Int64("offset", msg.Offset),
					zap.Duration("elapsed", elapsed),
				)
				it.result <- err
				continue
			}

			// Удобный debug-трейс + короткий stdout для быстрого «грезинга» при нагрузке.
			c.zlogger.Debug("message handled",
				zap.String("topic", msg.Topic),
				zap.Int("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
				zap.Int("key_bytes", len(msg.Key)),
				zap.Int("value_bytes", len(msg.Value)),
				zap.Duration("elapsed", elapsed),
			)
			stdlog.Printf("ok topic=%s p=%d off=%d bytes=%d", msg.Topic, msg.Partition, msg.Offset, len(msg.Value))

			it.result <- nil
		}
	}
}

// sleepWithContext — безопасный sleep, прерываемый контекстом.
func sleepWithContext(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func isBenignFetchTimeout(err error) bool {
	s := err.Error()
	return strings.Contains(s, "Request Timed Out") ||
		strings.Contains(s, "no messages received from kafka within the allocated time")
}
