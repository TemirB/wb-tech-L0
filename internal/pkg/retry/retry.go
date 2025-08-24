package retry

import (
	"context"
	"math/rand"
	"time"

	"github.com/TemirB/wb-tech-L0/internal/config"
)

func Do(ctx context.Context, retryPolicy config.Retry, fn func() error) error {
	d := retryPolicy.Base
	var err error

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < retryPolicy.Attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}

		delay := d
		if retryPolicy.JitterFactor > 0 {
			jitter := 1 + retryPolicy.JitterFactor*(2*r.Float64()-1)
			delay = time.Duration(float64(delay) * jitter)
		}

		if retryPolicy.Max > 0 && delay > retryPolicy.Max {
			delay = retryPolicy.Max
		}

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}

		d *= 2
		if retryPolicy.Max > 0 && d > retryPolicy.Max {
			d = retryPolicy.Max
		}
	}
	return err
}
