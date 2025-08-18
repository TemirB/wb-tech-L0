package shared

import (
	"encoding/json"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	_ = e.Encode(v)
}
