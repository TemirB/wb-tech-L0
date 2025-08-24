package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/segmentio/kafka-go"
)

type Spammer struct {
	writer    *kafka.Writer
	isRunning atomic.Bool
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	topic     string
	brokers   []string
	totalSent atomic.Int64
	startedAt time.Time
}

type SpamRequest struct {
	Rate     int    `json:"rate"`
	Duration string `json:"duration"`
}

type SpamStats struct {
	TotalSent int64         `json:"total_sent"`
	Duration  time.Duration `json:"duration"`
	Rate      int           `json:"rate"`
}

func NewSpammer(brokers []string, topic string) *Spammer {
	ctx, cancel := context.WithCancel(context.Background())

	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		BatchSize:    100,
	}

	return &Spammer{
		writer:    writer,
		ctx:       ctx,
		cancel:    cancel,
		topic:     topic,
		brokers:   brokers,
		startedAt: time.Now(),
	}
}

func (s *Spammer) StartSpam(rate int, duration time.Duration) {
	if s.isRunning.Load() {
		return
	}
	s.isRunning.Store(true)
	s.totalSent.Store(0)

	log.Printf("Starting spam: rate=%d msg/s, duration=%v s", rate, duration)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.isRunning.Store(false)

		ticker := time.NewTicker(time.Second / time.Duration(rate))
		defer ticker.Stop()

		timer := time.NewTimer(duration)
		defer timer.Stop()

		for {
			select {
			case <-ticker.C:
				message := generateFakeOrder()
				jsonData, err := json.Marshal(message)
				if err != nil {
					log.Printf("Error marshaling message: %v", err)
					continue
				}

				err = s.writer.WriteMessages(s.ctx, kafka.Message{
					Value: jsonData,
					Time:  time.Now(),
				})
				if err != nil {
					log.Printf("Error sending message to Kafka: %v", err)
				} else {
					s.totalSent.Add(1)
				}

			case <-timer.C:
				log.Printf("Spam completed. Total sent: %d", s.totalSent.Load())
				return

			case <-s.ctx.Done():
				log.Printf("Spam stopped. Total sent: %d", s.totalSent.Load())
				return
			}
		}
	}()
}

func (s *Spammer) StopSpam() {
	if s.isRunning.Load() {
		s.cancel()
		s.wg.Wait()

		// Recreate context for next run
		s.ctx, s.cancel = context.WithCancel(context.Background())
	}
}

func (s *Spammer) GetStats() SpamStats {
	return SpamStats{
		TotalSent: s.totalSent.Load(),
		Rate:      int(s.totalSent.Load()) / int(time.Since(s.startedAt).Seconds()),
	}
}

func (s *Spammer) Close() {
	s.StopSpam()
	s.writer.Close()
}

func generateFakeOrder() map[string]interface{} {
	// Базовая структура заказа
	order := map[string]interface{}{
		"order_uid":    fmt.Sprintf("test_%d_%d", time.Now().UnixNano(), rand.Intn(1000)),
		"track_number": fmt.Sprintf("WBILTEST%d", rand.Intn(10000)),
		"entry":        "WBIL",
		"delivery": map[string]interface{}{
			"name":    "Test User",
			"phone":   fmt.Sprintf("+7%d", 9000000000+rand.Intn(100000000)),
			"zip":     fmt.Sprintf("%d", 100000+rand.Intn(900000)),
			"city":    "Moscow",
			"address": fmt.Sprintf("Street %d", rand.Intn(100)),
			"region":  "Moscow Region",
			"email":   fmt.Sprintf("test%d@example.com", rand.Intn(1000)),
		},
		"payment": map[string]interface{}{
			"transaction":   fmt.Sprintf("trans_%d", time.Now().UnixNano()),
			"request_id":    "",
			"currency":      "USD",
			"provider":      "wbpay",
			"amount":        rand.Intn(10000) + 100,
			"payment_dt":    time.Now().Unix(),
			"bank":          "sberbank",
			"delivery_cost": rand.Intn(500) + 100,
			"goods_total":   rand.Intn(800) + 200,
			"custom_fee":    0,
		},
		"items": []map[string]interface{}{
			{
				"chrt_id":      rand.Intn(10000000),
				"track_number": fmt.Sprintf("WBILTEST%d", rand.Intn(10000)),
				"price":        rand.Intn(500) + 50,
				"rid":          fmt.Sprintf("rid_%d", rand.Intn(1000000)),
				"name":         "Test Product",
				"sale":         rand.Intn(50),
				"size":         "M",
				"total_price":  rand.Intn(400) + 100,
				"nm_id":        rand.Intn(1000000),
				"brand":        "Test Brand",
				"status":       202,
			},
		},
		"locale":             "en",
		"internal_signature": "",
		"customer_id":        fmt.Sprintf("customer_%d", rand.Intn(1000)),
		"delivery_service":   "meest",
		"shardkey":           fmt.Sprintf("%d", rand.Intn(10)),
		"sm_id":              rand.Intn(100),
		"date_created":       time.Now().Format(time.RFC3339),
		"oof_shard":          "1",
	}

	//
	// if messageSize > 0 {
	// 	extraData := make(map[string]interface{})
	// 	for i := 0; i < (messageSize/100)+1; i++ {
	// 		extraData[fmt.Sprintf("extra_field_%d", i)] = fmt.Sprintf("extra_value_%d_%s", i, generateRandomString(messageSize/10))
	// 	}
	// 	order["extra_data"] = extraData
	// }

	return order
}

// func generateRandomString(length int) string {
// 	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
// 	b := make([]byte, length)
// 	for i := range b {
// 		b[i] = charset[rand.Intn(len(charset))]
// 	}
// 	return string(b)
// }

func main() {
	brokers := []string{"kafka:9092"}
	if envBrokers := os.Getenv("KAFKA_BROKERS"); envBrokers != "" {
		brokers = []string{envBrokers}
	}

	topic := "orders"
	if envTopic := os.Getenv("KAFKA_TOPIC"); envTopic != "" {
		topic = envTopic
	}

	spammer := NewSpammer(brokers, topic)
	defer spammer.Close()

	http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req SpamRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if req.Rate <= 0 {
			req.Rate = 10
		}

		duration, err := time.ParseDuration(req.Duration)
		if err != nil {
			http.Error(w, "Invalid duration format: "+err.Error(), http.StatusBadRequest)
			return
		}

		spammer.StartSpam(req.Rate, duration)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "started",
			"rate":     req.Rate,
			"duration": duration.String(),
		})
	})

	http.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		spammer.StopSpam()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "stopped",
			"total_sent": spammer.totalSent.Load(),
		})
	})

	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"is_running": spammer.isRunning.Load(),
			"total_sent": spammer.totalSent.Load(),
		})
	})

	port := ":8082"
	if envPort := os.Getenv("SPAMMER_PORT"); envPort != "" {
		port = ":" + envPort
	}

	log.Printf("Spammer server started on %s", port)
	log.Printf("Endpoints: POST /start, POST /stop, GET /stats")
	log.Fatal(http.ListenAndServe(port, nil))
}
