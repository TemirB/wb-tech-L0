package observability

import (
	"fmt"
	"net/http"
)

func AppendServerTiming(w http.ResponseWriter, name string, durMs float64, desc string) {
	if durMs > 0 && desc != "" {
		w.Header().Add("Server-Timing", fmt.Sprintf("%s;dur=%.2f;desc=%q", name, durMs, desc))
		return
	}
	if durMs > 0 {
		w.Header().Add("Server-Timing", fmt.Sprintf("%s;dur=%.2f", name, durMs))
		return
	}
	if desc != "" {
		w.Header().Add("Server-Timing", fmt.Sprintf("%s;desc=%q", name, desc))
	}
}
func SetIfPos(w http.ResponseWriter, key string, ms float64) {
	if ms > 0 {
		w.Header().Set(key, fmt.Sprintf("%.2f", ms))
	}
}
