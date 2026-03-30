package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Stats хранит статистику запросов
type Stats struct {
	RequestCount int64  `json:"request_count"`
	Uptime       string `json:"uptime"`
	StartTime    string `json:"start_time"`
}

// EchoRequest структура запроса для echo эндпоинта
type EchoRequest struct {
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// EchoResponse структура ответа для echo эндпоинта
type EchoResponse struct {
	Original  EchoRequest `json:"original"`
	Processed string      `json:"processed"`
	Timestamp string      `json:"timestamp"`
}

// HealthResponse структура ответа для health эндпоинта
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
}

var (
	stats     = Stats{RequestCount: 0}
	statsMu   sync.RWMutex
	startTime time.Time
)

func main() {
	startTime = time.Now()
	stats.StartTime = startTime.Format(time.RFC3339)

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/api/stats", statsHandler)
	http.HandleFunc("/api/echo", echoHandler)

	port := ":8080"
	fmt.Printf("Go API сервер запущен на порту %s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

// healthHandler проверяет статус сервера
func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	incrementRequestCount()

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().Format(time.RFC3339),
		Version:   "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// statsHandler возвращает статистику запросов
func statsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	incrementRequestCount()

	statsMu.RLock()
	stats.Uptime = time.Since(startTime).String()
	response := Stats{
		RequestCount: stats.RequestCount,
		Uptime:       stats.Uptime,
		StartTime:    stats.StartTime,
	}
	statsMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// echoHandler принимает JSON и возвращает его с обработкой
func echoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	incrementRequestCount()

	var req EchoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	response := EchoResponse{
		Original:  req,
		Processed: fmt.Sprintf("Received: %s (processed by Go API)", req.Message),
		Timestamp: time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func incrementRequestCount() {
	statsMu.Lock()
	defer statsMu.Unlock()
	stats.RequestCount++
}
