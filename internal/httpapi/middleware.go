package httpapi

import (
	"net/http"
	"time"

	"github.com/TemirB/wb-tech-L0/internal/observability"

	"github.com/go-chi/chi/v5/middleware"
)

// ServerTimingApp — middleware that measures the total request processing time and
// writes app;dur=... to Server-Timing + sends an event to Metrics.ObserveHTTP.
func ServerTimingApp(m observability.Metrics) func(http.Handler) http.Handler {
	if m == nil {
		m = observability.Noop{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			dur := float64(time.Since(start).Microseconds()) / 1000.0
			observability.AppendServerTiming(w, "app", dur, "")
			route := r.URL.Path
			m.ObserveHTTP(r.Method, route, ww.Status(), dur)
		})
	}
}
