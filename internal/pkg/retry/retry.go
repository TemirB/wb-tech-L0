package retry

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// Config defines retry behavior parameters
type Config struct {
	Attempts               int           // Maximum number of attempts (including first)
	Initial, Max           time.Duration // Initial backoff duration
	Multiplier, JitterFrac float64       // Exponential backoff multiplier and jitter fraction (if JitterFrac == 0 => no jitter)
}

// Do executes the operation with exponential backoff and jitter
// Returns nil on success or last error after all attempts fail
// Context cancellation interrupts the retry loop immediately
func Do(ctx context.Context, cfg Config, fn func(int) error) error {
	if cfg.Attempts <= 0 {
		cfg.Attempts = 1
	}
	if cfg.Initial <= 0 {
		cfg.Initial = 50 * time.Millisecond
	}
	if cfg.Max <= 0 {
		cfg.Max = 2 * time.Second
	}
	if cfg.Multiplier <= 0 {
		cfg.Multiplier = 2
	}

	for i := 1; i <= cfg.Attempts; i++ {
		if err := fn(i); err != nil {
			if i == cfg.Attempts {
				return err
			}

			// Calculate exponential backoff: initial * multiplier^(attempt-1)
			d := time.Duration(float64(cfg.Initial) * math.Pow(cfg.Multiplier, float64(i-1)))

			if d > cfg.Max {
				d = cfg.Max
			}

			if cfg.JitterFrac > 0 {
				// Calculate jitter range
				j := time.Duration(rand.Float64() * cfg.JitterFrac * float64(d))
				d = d - j/2 + j
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(d):
			}
			continue
		}
		return nil
	}
	return nil
}
