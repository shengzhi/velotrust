package agent

import (
	"encoding/json"
	"net/http"
)

// HealthHandler exposes readiness without returning config secrets or backend
// error details. A gateway is ready only after a complete config generation is
// applied successfully.
func (r *Reconciler) HealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet && req.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if req.URL.Path != "/healthz" && req.URL.Path != "/readyz" {
			http.NotFound(w, req)
			return
		}
		status := r.Status()
		code := http.StatusServiceUnavailable
		if status.Phase == PhaseReady {
			code = http.StatusOK
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		if req.Method == http.MethodHead {
			return
		}
		_ = json.NewEncoder(w).Encode(status)
	})
}
