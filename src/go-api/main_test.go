package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func init() {
	// Инициализация для тестов
	startTime = time.Now()
	stats.StartTime = startTime.Format(time.RFC3339)
}

// TestHealthHandler проверяет handler /health
func TestHealthHandler(t *testing.T) {
	// Создаём тестовый запрос
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	// Вызываем handler
	healthHandler(w, req)

	// Проверяем статус код
	if w.Code != http.StatusOK {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusOK, w.Code)
	}

	// Проверяем Content-Type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Ожидаемый Content-Type: application/json, получен: %s", contentType)
	}

	// Парсим ответ
	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	// Проверяем поля ответа
	if response.Status != "healthy" {
		t.Errorf("Ожидаемый статус: healthy, получен: %s", response.Status)
	}
	if response.Version != "1.0.0" {
		t.Errorf("Ожидаемая версия: 1.0.0, получена: %s", response.Version)
	}
	if response.Timestamp == "" {
		t.Error("Ожидался непустой timestamp")
	}
}

// TestHealthHandlerMethodNotAllowed проверяет неправильный метод для /health
func TestHealthHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()

	healthHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestStatsHandler проверяет handler /api/stats
func TestStatsHandler(t *testing.T) {
	// Сбрасываем только счётчик запросов
	statsMu.Lock()
	initialCount := stats.RequestCount
	stats.RequestCount = 0
	statsMu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()

	statsHandler(w, req)

	// Проверяем статус код
	if w.Code != http.StatusOK {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusOK, w.Code)
	}

	// Парсим ответ
	var response Stats
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	// Проверяем, что счётчик увеличился (как минимум 1 после запроса)
	if response.RequestCount < 1 {
		t.Errorf("Ожидался RequestCount >= 1, получен: %d", response.RequestCount)
	}

	// Проверяем, что uptime не пустой
	if response.Uptime == "" {
		t.Error("Ожидался непустой uptime")
	}

	// Проверяем, что start_time установлен (не сбрасываем его в тесте)
	if response.StartTime == "" {
		t.Error("Ожидался непустой start_time")
	}

	// Восстанавливаем счётчик
	statsMu.Lock()
	stats.RequestCount = initialCount
	statsMu.Unlock()
}

// TestStatsHandlerMethodNotAllowed проверяет неправильный метод для /api/stats
func TestStatsHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/stats", nil)
	w := httptest.NewRecorder()

	statsHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestEchoHandler проверяет handler /api/echo
func TestEchoHandler(t *testing.T) {
	// Тестовые данные
	testMessage := "Hello from test!"
	testData := map[string]interface{}{
		"key":    "value",
		"number": 42,
	}

	requestBody := EchoRequest{
		Message: testMessage,
		Data:    testData,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Ошибка сериализации JSON: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/echo", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	echoHandler(w, req)

	// Проверяем статус код
	if w.Code != http.StatusOK {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusOK, w.Code)
	}

	// Парсим ответ
	var response EchoResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	// Проверяем оригинальное сообщение
	if response.Original.Message != testMessage {
		t.Errorf("Ожидаемое сообщение: %s, получено: %s", testMessage, response.Original.Message)
	}

	// Проверяем обработанное сообщение
	expectedProcessed := "Received: " + testMessage + " (processed by Go API)"
	if response.Processed != expectedProcessed {
		t.Errorf("Ожидаемое processed: %s, получено: %s", expectedProcessed, response.Processed)
	}

	// Проверяем timestamp
	if response.Timestamp == "" {
		t.Error("Ожидался непустой timestamp")
	}
}

// TestEchoHandlerInvalidJSON проверяет обработку невалидного JSON
func TestEchoHandlerInvalidJSON(t *testing.T) {
	invalidJSON := []byte(`{"message": "invalid json`)

	req := httptest.NewRequest(http.MethodPost, "/api/echo", bytes.NewBuffer(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	echoHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusBadRequest, w.Code)
	}
}

// TestEchoHandlerMethodNotAllowed проверяет неправильный метод для /api/echo
func TestEchoHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/echo", nil)
	w := httptest.NewRecorder()

	echoHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestEchoHandlerEmptyMessage проверяет обработку пустого сообщения
func TestEchoHandlerEmptyMessage(t *testing.T) {
	requestBody := EchoRequest{
		Message: "",
		Data:    nil,
	}

	jsonBody, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/api/echo", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	echoHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusOK, w.Code)
	}

	var response EchoResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	if response.Original.Message != "" {
		t.Errorf("Ожидалось пустое сообщение, получено: %s", response.Original.Message)
	}
}

// TestIncrementRequestCount проверяет увеличение счётчика запросов
func TestIncrementRequestCount(t *testing.T) {
	// Сбрасываем статистику
	statsMu.Lock()
	initialCount := stats.RequestCount
	stats.RequestCount = 0
	statsMu.Unlock()

	// Увеличиваем счётчик несколько раз
	for i := 0; i < 5; i++ {
		incrementRequestCount()
	}

	// Проверяем результат
	statsMu.RLock()
	finalCount := stats.RequestCount
	statsMu.RUnlock()

	if finalCount != 5 {
		t.Errorf("Ожидался счётчик 5, получен: %d", finalCount)
	}

	// Восстанавливаем начальное значение
	statsMu.Lock()
	stats.RequestCount = initialCount
	statsMu.Unlock()
}

// TestConcurrentRequests проверяет потокобезопасность счётчика
func TestConcurrentRequests(t *testing.T) {
	// Сбрасываем статистику
	statsMu.Lock()
	stats.RequestCount = 0
	statsMu.Unlock()

	// Запускаем 100 горутин
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			incrementRequestCount()
			done <- true
		}()
	}

	// Ждём завершения всех горутин
	for i := 0; i < 100; i++ {
		<-done
	}

	// Проверяем результат
	statsMu.RLock()
	finalCount := stats.RequestCount
	statsMu.RUnlock()

	if finalCount != 100 {
		t.Errorf("Ожидался счётчик 100, получен: %d", finalCount)
	}
}

// TestHealthResponseJSONFormat проверяет формат JSON ответа health
func TestHealthResponseJSONFormat(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	healthHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	// Проверяем наличие всех ожидаемых полей
	expectedFields := []string{"status", "timestamp", "version"}
	for _, field := range expectedFields {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("Отсутствует поле в JSON ответе: %s", field)
		}
	}
}

// TestStatsResponseJSONFormat проверяет формат JSON ответа stats
func TestStatsResponseJSONFormat(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()

	statsHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	// Проверяем наличие всех ожидаемых полей
	expectedFields := []string{"request_count", "uptime", "start_time"}
	for _, field := range expectedFields {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("Отсутствует поле в JSON ответе: %s", field)
		}
	}
}

// TestEchoResponseJSONFormat проверяет формат JSON ответа echo
func TestEchoResponseJSONFormat(t *testing.T) {
	requestBody := EchoRequest{Message: "test"}
	jsonBody, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/api/echo", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	echoHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	// Проверяем наличие всех ожидаемых полей
	expectedFields := []string{"original", "processed", "timestamp"}
	for _, field := range expectedFields {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("Отсутствует поле в JSON ответе: %s", field)
		}
	}
}

// TestMainWithServer проверяет запуск сервера (интеграционный тест)
func TestMainWithServer(t *testing.T) {
	// Создаём тестовый сервер
	server := httptest.NewServer(http.NewServeMux())
	defer server.Close()

	// Регистрируем handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/stats", statsHandler)
	mux.HandleFunc("/api/echo", echoHandler)

	testServer := httptest.NewServer(mux)
	defer testServer.Close()

	// Тестируем health эндпоинт
	resp, err := http.Get(testServer.URL + "/health")
	if err != nil {
		t.Fatalf("Ошибка запроса к серверу: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusOK, resp.StatusCode)
	}

	// Тестируем stats эндпоинт
	resp, err = http.Get(testServer.URL + "/api/stats")
	if err != nil {
		t.Fatalf("Ошибка запроса к серверу: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusOK, resp.StatusCode)
	}

	// Тестируем echo эндпоинт
	echoReq := EchoRequest{Message: "integration test"}
	jsonBody, _ := json.Marshal(echoReq)
	resp, err = http.Post(testServer.URL+"/api/echo", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatalf("Ошибка запроса к серверу: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusOK, resp.StatusCode)
	}
}
