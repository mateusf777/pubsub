package server

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

// HealthCheck provides HTTP endpoints for health and readiness checks.
// Useful for container orchestration platforms like Kubernetes.
type HealthCheck struct {
	server     *http.Server
	startTime  time.Time
	isReady    atomic.Bool
	pubsub     *PubSub
	tlsEnabled bool
}

// HealthResponse represents the JSON response for health checks.
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Uptime    string    `json:"uptime,omitempty"`
	TLS       bool      `json:"tls,omitempty"`
}

// ReadinessResponse represents the JSON response for readiness checks.
type ReadinessResponse struct {
	Ready     bool      `json:"ready"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message,omitempty"`
}

// NewHealthCheck creates a new health check server.
// The health check server runs on a separate HTTP port for monitoring.
func NewHealthCheck(addr string, ps *PubSub, tlsEnabled bool) *HealthCheck {
	hc := &HealthCheck{
		startTime:  time.Now(),
		pubsub:     ps,
		tlsEnabled: tlsEnabled,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", hc.healthHandler)
	mux.HandleFunc("/ready", hc.readyHandler)
	mux.HandleFunc("/healthz", hc.healthHandler) // Kubernetes compatibility
	mux.HandleFunc("/readyz", hc.readyHandler)   // Kubernetes compatibility

	hc.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return hc
}

// Start starts the health check HTTP server.
func (hc *HealthCheck) Start() error {
	return hc.server.ListenAndServe()
}

// SetReady marks the server as ready to accept traffic.
func (hc *HealthCheck) SetReady(ready bool) {
	hc.isReady.Store(ready)
}

// healthHandler responds to health check requests.
// Always returns 200 OK if the process is running.
func (hc *HealthCheck) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Uptime:    time.Since(hc.startTime).String(),
		TLS:       hc.tlsEnabled,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// readyHandler responds to readiness check requests.
// Returns 200 OK only if the server is ready to accept connections.
func (hc *HealthCheck) readyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ready := hc.isReady.Load()

	response := ReadinessResponse{
		Ready:     ready,
		Timestamp: time.Now(),
	}

	if !ready {
		response.Message = "Server is starting up"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(response)
		return
	}

	response.Message = "Server is ready"
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Close gracefully shuts down the health check server.
func (hc *HealthCheck) Close() error {
	return hc.server.Close()
}
