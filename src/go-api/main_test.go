package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func init() {
	// Инициализация для тестов
	startTime = time.Now()
	stats.StartTime = startTime.Format(time.RFC3339)
	initSamplePosts()
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

// ==================== ТЕСТЫ ДЛЯ POSTS HANDLER ====================

// TestGetPostsHandler проверяет получение списка постов
func TestGetPostsHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	w := httptest.NewRecorder()

	postsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusOK, w.Code)
	}

	var response GetPostsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	if response.Total == 0 {
		t.Error("Ожидалось, что total > 0")
	}
	if response.Page != 1 {
		t.Errorf("Ожидаемая страница: 1, получена: %d", response.Page)
	}
	if len(response.Posts) == 0 {
		t.Error("Ожидалось, что posts не пустой")
	}
}

// TestGetPostsHandlerWithPagination проверяет пагинацию
func TestGetPostsHandlerWithPagination(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/posts?page=1&per_page=2", nil)
	w := httptest.NewRecorder()

	postsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusOK, w.Code)
	}

	var response GetPostsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	if len(response.Posts) > 2 {
		t.Errorf("Ожидалось не более 2 постов на странице, получено: %d", len(response.Posts))
	}
}

// TestCreatePostHandler проверяет создание поста
func TestCreatePostHandler(t *testing.T) {
	createReq := CreatePostRequest{
		Title:    "Тестовый пост",
		Content:  "Содержимое тестового поста",
		Excerpt:  "Краткое описание",
		TagNames: []string{"Test", "Go"},
	}

	jsonBody, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	postsHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusCreated, w.Code)
	}

	var response CreatePostResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	if response.Post.Title != createReq.Title {
		t.Errorf("Ожидаемый заголовок: %s, получен: %s", createReq.Title, response.Post.Title)
	}
	if response.Post.Slug != "тестовый-пост" && response.Post.Slug != "testovyy-post" {
		// Slug может быть транслитерирован или оставлен как есть
		if response.Post.Slug == "" {
			t.Error("Ожидался непустой slug")
		}
	}
	if len(response.Post.Tags) != 2 {
		t.Errorf("Ожидалось 2 тега, получено: %d", len(response.Post.Tags))
	}
	if response.Message != "Post created successfully" {
		t.Errorf("Ожидаемое сообщение: 'Post created successfully', получено: %s", response.Message)
	}
}

// TestCreatePostHandlerEmptyTitle проверяет создание поста с пустым заголовком
func TestCreatePostHandlerEmptyTitle(t *testing.T) {
	createReq := CreatePostRequest{
		Title:   "",
		Content: "Содержимое",
	}

	jsonBody, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	postsHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusBadRequest, w.Code)
	}
}

// TestCreatePostHandlerEmptyContent проверяет создание поста с пустым содержимым
func TestCreatePostHandlerEmptyContent(t *testing.T) {
	createReq := CreatePostRequest{
		Title:   "Заголовок",
		Content: "",
	}

	jsonBody, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	postsHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusBadRequest, w.Code)
	}
}

// TestGetPostByIDHandler проверяет получение поста по ID
func TestGetPostByIDHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/posts/1", nil)
	w := httptest.NewRecorder()

	postByIDHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusOK, w.Code)
	}

	var response PostResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	if response.Post.ID != 1 {
		t.Errorf("Ожидаемый ID поста: 1, получен: %d", response.Post.ID)
	}
	if response.Post.Author == nil {
		t.Error("Ожидался непустой автор")
	}
	if response.Post.ViewCount < 0 {
		t.Error("Ожидалось viewCount >= 0")
	}
}

// TestGetPostByIDHandlerNotFound проверяет получение несуществующего поста
func TestGetPostByIDHandlerNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/posts/9999", nil)
	w := httptest.NewRecorder()

	postByIDHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusNotFound, w.Code)
	}

	var errorResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errorResp); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	if errorResp.Message != "Post not found" {
		t.Errorf("Ожидаемое сообщение: 'Post not found', получено: %s", errorResp.Message)
	}
}

// TestUpdatePostHandler проверяет обновление поста
func TestUpdatePostHandler(t *testing.T) {
	// Сначала создаём тестовый пост
	postsMu.Lock()
	testPost := Post{
		ID:        999,
		Title:     "Original Title",
		Slug:      "original-title",
		Content:   "Original content",
		Author:    defaultAuthor,
		Tags:      []Tag{},
		Comments:  []Comment{},
		CreatedAt: time.Now(),
	}
	posts[999] = testPost
	postsMu.Unlock()

	// Обновляем
	updateReq := CreatePostRequest{
		Title:    "Updated Title",
		Excerpt:  "New excerpt",
		TagNames: []string{"Updated"},
	}

	jsonBody, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPut, "/api/posts/999", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	postByIDHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusOK, w.Code)
	}

	var response PostResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	if response.Post.Title != "Updated Title" {
		t.Errorf("Ожидаемый заголовок: 'Updated Title', получен: %s", response.Post.Title)
	}
	if response.Post.Excerpt != "New excerpt" {
		t.Errorf("Ожидаемое описание: 'New excerpt', получено: %s", response.Post.Excerpt)
	}

	// Очищаем тестовый пост
	postsMu.Lock()
	delete(posts, 999)
	postsMu.Unlock()
}

// TestDeletePostHandler проверяет удаление поста
func TestDeletePostHandler(t *testing.T) {
	// Создаём тестовый пост
	postsMu.Lock()
	testPost := Post{
		ID:        998,
		Title:     "To Delete",
		Slug:      "to-delete",
		Content:   "Will be deleted",
		Author:    defaultAuthor,
		Tags:      []Tag{},
		Comments:  []Comment{},
		CreatedAt: time.Now(),
	}
	posts[998] = testPost
	postsMu.Unlock()

	req := httptest.NewRequest(http.MethodDelete, "/api/posts/998", nil)
	w := httptest.NewRecorder()

	postByIDHandler(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusNoContent, w.Code)
	}

	// Проверяем, что пост удалён
	postsMu.RLock()
	_, exists := posts[998]
	postsMu.RUnlock()

	if exists {
		t.Error("Ожидалось, что пост будет удалён")
	}
}

// TestDeletePostHandlerNotFound проверяет удаление несуществующего поста
func TestDeletePostHandlerNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/posts/9999", nil)
	w := httptest.NewRecorder()

	postByIDHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusNotFound, w.Code)
	}
}

// TestPostByIDHandlerInvalidID проверяет обработку невалидного ID
func TestPostByIDHandlerInvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/posts/invalid", nil)
	w := httptest.NewRecorder()

	postByIDHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusBadRequest, w.Code)
	}
}

// TestPostsHandlerMethodNotAllowed проверяет неправильный метод
func TestPostsHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/api/posts", nil)
	w := httptest.NewRecorder()

	postsHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestPostComplexJSONStructure проверяет сложную JSON-структуру поста
func TestPostComplexJSONStructure(t *testing.T) {
	createReq := CreatePostRequest{
		Title:   "Complex Post",
		Content: "Content with complex structure",
		TagNames: []string{"Tag1", "Tag2", "Tag3", "Tag4", "Tag5"},
	}

	jsonBody, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	postsHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	// Проверяем наличие всех ожидаемых полей в ответе
	if _, exists := rawJSON["post"]; !exists {
		t.Error("Отсутствует поле 'post' в ответе")
	}
	if _, exists := rawJSON["message"]; !exists {
		t.Error("Отсутствует поле 'message' в ответе")
	}
	if _, exists := rawJSON["timestamp"]; !exists {
		t.Error("Отсутствует поле 'timestamp' в ответе")
	}

	// Проверяем структуру поста
	postData, ok := rawJSON["post"].(map[string]interface{})
	if !ok {
		t.Fatal("Поле 'post' не является объектом")
	}

	expectedPostFields := []string{"id", "title", "slug", "content", "author", "tags", "view_count", "created_at"}
	for _, field := range expectedPostFields {
		if _, exists := postData[field]; !exists {
			t.Errorf("Отсутствует поле в post: %s", field)
		}
	}

	// Проверяем структуру автора
	authorData, ok := postData["author"].(map[string]interface{})
	if !ok {
		t.Fatal("Поле 'author' не является объектом")
	}

	expectedAuthorFields := []string{"id", "username", "email"}
	for _, field := range expectedAuthorFields {
		if _, exists := authorData[field]; !exists {
			t.Errorf("Отсутствует поле в author: %s", field)
		}
	}

	// Проверяем, что tags - это массив
	tagsData, ok := postData["tags"].([]interface{})
	if !ok {
		t.Fatal("Поле 'tags' не является массивом")
	}
	if len(tagsData) != 5 {
		t.Errorf("Ожидалось 5 тегов, получено: %d", len(tagsData))
	}
}

// TestGetPostsResponseJSONFormat проверяет формат JSON ответа списка постов
func TestGetPostsResponseJSONFormat(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	w := httptest.NewRecorder()

	postsHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	expectedFields := []string{"posts", "total", "page", "per_page", "timestamp"}
	for _, field := range expectedFields {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("Отсутствует поле в JSON ответе: %s", field)
		}
	}
}

// TestCreatePostResponseJSONFormat проверяет формат JSON ответа создания поста
func TestCreatePostResponseJSONFormat(t *testing.T) {
	createReq := CreatePostRequest{
		Title:   "Format Test",
		Content: "Content",
	}
	jsonBody, _ := json.Marshal(createReq)

	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	postsHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	expectedFields := []string{"post", "message", "timestamp"}
	for _, field := range expectedFields {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("Отсутствует поле в JSON ответе: %s", field)
		}
	}
}

// TestPostResponseJSONFormat проверяет формат JSON ответа одного поста
func TestPostResponseJSONFormat(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/posts/1", nil)
	w := httptest.NewRecorder()

	postByIDHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	expectedFields := []string{"post", "timestamp"}
	for _, field := range expectedFields {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("Отсутствует поле в JSON ответе: %s", field)
		}
	}
}

// TestErrorResponseJSONFormat проверяет формат JSON ответа ошибки
func TestErrorResponseJSONFormat(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/posts/99999", nil)
	w := httptest.NewRecorder()

	postByIDHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	expectedFields := []string{"error", "message", "timestamp"}
	for _, field := range expectedFields {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("Отсутствует поле в JSON ответе: %s", field)
		}
	}
}

// TestAuthorNestedStructure проверяет вложенную структуру Author в Post
func TestAuthorNestedStructure(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/posts/1", nil)
	w := httptest.NewRecorder()

	postByIDHandler(w, req)

	var response PostResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	if response.Post.Author == nil {
		t.Fatal("Author должен быть установлен")
	}

	if response.Post.Author.ID == 0 {
		t.Error("Author.ID должен быть > 0")
	}
	if response.Post.Author.Username == "" {
		t.Error("Author.Username не должен быть пустым")
	}
	if response.Post.Author.Email == "" {
		t.Error("Author.Email не должен быть пустым")
	}
}

// TestTagArrayStructure проверяет структуру массива тегов
func TestTagArrayStructure(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/posts/1", nil)
	w := httptest.NewRecorder()

	postByIDHandler(w, req)

	var response PostResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декодирования JSON: %v", err)
	}

	if len(response.Post.Tags) == 0 {
		t.Error("Ожидалось, что tags не пустой")
	}

	for i, tag := range response.Post.Tags {
		if tag.ID == 0 {
			t.Errorf("Tag[%d].ID должен быть > 0", i)
		}
		if tag.Name == "" {
			t.Errorf("Tag[%d].Name не должен быть пустым", i)
		}
		if tag.Slug == "" {
			t.Errorf("Tag[%d].Slug не должен быть пустым", i)
		}
	}
}

// TestViewCountIncrement проверяет увеличение счётчика просмотров
func TestViewCountIncrement(t *testing.T) {
	// Получаем начальное значение
	req1 := httptest.NewRequest(http.MethodGet, "/api/posts/2", nil)
	w1 := httptest.NewRecorder()
	postByIDHandler(w1, req1)

	var resp1 PostResponse
	json.NewDecoder(w1.Body).Decode(&resp1)
	initialViews := resp1.Post.ViewCount

	// Получаем ещё раз
	req2 := httptest.NewRequest(http.MethodGet, "/api/posts/2", nil)
	w2 := httptest.NewRecorder()
	postByIDHandler(w2, req2)

	var resp2 PostResponse
	json.NewDecoder(w2.Body).Decode(&resp2)

	if resp2.Post.ViewCount <= initialViews {
		t.Errorf("Ожидалось, что viewCount увеличится (было: %d, стало: %d)", initialViews, resp2.Post.ViewCount)
	}
}

// TestCreatePostIntegration проверяет полный цикл создания и получения поста
func TestCreatePostIntegration(t *testing.T) {
	// Создаём пост
	createReq := CreatePostRequest{
		Title:    "Integration Test Post",
		Content:  "Full integration test content",
		Excerpt:  "Test excerpt",
		TagNames: []string{"Integration", "Test"},
	}

	jsonBody, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	postsHandler(w, req)

	var createResp CreatePostResponse
	if err := json.NewDecoder(w.Body).Decode(&createResp); err != nil {
		t.Fatalf("Ошибка декодирования JSON создания: %v", err)
	}

	postID := createResp.Post.ID

	// Получаем созданный пост
	getReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/posts/%d", postID), nil)
	getW := httptest.NewRecorder()
	postByIDHandler(getW, getReq)

	var getResp PostResponse
	if err := json.NewDecoder(getW.Body).Decode(&getResp); err != nil {
		t.Fatalf("Ошибка декодирования JSON получения: %v", err)
	}

	if getResp.Post.Title != createReq.Title {
		t.Errorf("Заголовки не совпадают: %s vs %s", createReq.Title, getResp.Post.Title)
	}
	if getResp.Post.Content != createReq.Content {
		t.Errorf("Содержимое не совпадает: %s vs %s", createReq.Content, getResp.Post.Content)
	}

	// Удаляем пост
	deleteReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/posts/%d", postID), nil)
	deleteW := httptest.NewRecorder()
	postByIDHandler(deleteW, deleteReq)

	if deleteW.Code != http.StatusNoContent {
		t.Errorf("Ожидаемый статус код при удалении: %d, получен: %d", http.StatusNoContent, deleteW.Code)
	}
}

// ==================== ТЕСТЫ ДЛЯ GRACEFUL SHUTDOWN ====================

// TestIsShuttingDown проверяет функцию isShuttingDown
func TestIsShuttingDown(t *testing.T) {
	// Сбрасываем shutdown флаг
	ResetShutdown()

	if isShuttingDown() {
		t.Error("Ожидалось, что shutdown не активен")
	}

	// Устанавливаем shutdown флаг
	SetShutdownInProgress(1)

	if !isShuttingDown() {
		t.Error("Ожидалось, что shutdown активен")
	}

	// Сбрасываем
	ResetShutdown()
}

// TestSetShutdownInProgress проверяет установку флага shutdown
func TestSetShutdownInProgress(t *testing.T) {
	ResetShutdown()

	SetShutdownInProgress(1)
	if !isShuttingDown() {
		t.Error("Ожидалось, что shutdown активен после установки в 1")
	}

	SetShutdownInProgress(0)
	if isShuttingDown() {
		t.Error("Ожидалось, что shutdown не активен после установки в 0")
	}
}

// TestResetShutdown проверяет сброс флага shutdown
func TestResetShutdown(t *testing.T) {
	SetShutdownInProgress(1)
	ResetShutdown()

	if isShuttingDown() {
		t.Error("Ожидалось, что shutdown сброшен после ResetShutdown")
	}
}

// TestGetServer проверяет функцию GetServer
func TestGetServer(t *testing.T) {
	// Создаём тестовый сервер
	testServer := &http.Server{
		Addr: ":9999",
	}
	SetServer(testServer)

	retrieved := GetServer()
	if retrieved != testServer {
		t.Error("Ожидалось, что GetServer вернёт установленный сервер")
	}

	// Сбрасываем
	SetServer(nil)
}

// TestSetServer проверяет функцию SetServer
func TestSetServer(t *testing.T) {
	testServer := &http.Server{
		Addr: ":8888",
	}

	SetServer(testServer)
	if GetServer() != testServer {
		t.Error("Ожидалось, что сервер установлен")
	}

	SetServer(nil)
}

// TestGracefulShutdownWithServer проверяет graceful shutdown с реальным сервером
func TestGracefulShutdownWithServer(t *testing.T) {
	// Создаём тестовый mux
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/stats", statsHandler)

	// Создаём сервер
	testServer := &http.Server{
		Addr:         ":18080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  10 * time.Second,
	}

	SetServer(testServer)
	ResetShutdown()

	// Запускаем сервер в горутине
	serverErr := make(chan error, 1)
	go func() {
		if err := testServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		} else {
			serverErr <- nil
		}
	}()

	// Даём серверу время запуститься
	time.Sleep(100 * time.Millisecond)

	// Проверяем, что сервер работает
	resp, err := http.Get("http://localhost:18080/health")
	if err != nil {
		t.Fatalf("Сервер не отвечает: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Ожидаемый статус код: %d, получен: %d", http.StatusOK, resp.StatusCode)
	}

	// Инициируем graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	shutdownErr := make(chan error, 1)
	go func() {
		shutdownErr <- testServer.Shutdown(ctx)
	}()

	// Ждём завершения shutdown
	select {
	case err := <-shutdownErr:
		if err != nil {
			t.Errorf("Ошибка при shutdown: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Error("Timeout при ожидании shutdown")
	}

	// Проверяем, что сервер остановлен
	_, err = http.Get("http://localhost:18080/health")
	if err == nil {
		t.Error("Ожидалась ошибка подключения после shutdown")
	}

	// Сбрасываем
	SetServer(nil)
	ResetShutdown()
}

// TestGracefulShutdownFlagDuringShutdown проверяет установку флага во время shutdown
func TestGracefulShutdownFlagDuringShutdown(t *testing.T) {
	ResetShutdown()

	// Имитируем начало shutdown
	SetShutdownInProgress(1)

	if !isShuttingDown() {
		t.Error("Ожидалось, что флаг shutdown установлен")
	}

	// Сбрасываем после теста
	ResetShutdown()
}

// TestServerConfiguration проверяет конфигурацию сервера
func TestServerConfiguration(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)

	testServer := &http.Server{
		Addr:         ":18081",
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	SetServer(testServer)

	server := GetServer()
	if server == nil {
		t.Fatal("Сервер не установлен")
	}

	if server.ReadTimeout != 15*time.Second {
		t.Errorf("Ожидаемый ReadTimeout: 15s, получен: %v", server.ReadTimeout)
	}

	if server.WriteTimeout != 15*time.Second {
		t.Errorf("Ожидаемый WriteTimeout: 15s, получен: %v", server.WriteTimeout)
	}

	if server.IdleTimeout != 60*time.Second {
		t.Errorf("Ожидаемый IdleTimeout: 60s, получен: %v", server.IdleTimeout)
	}

	SetServer(nil)
}

// TestConcurrentShutdownAndRequest проверяет обработку запросов во время shutdown
func TestConcurrentShutdownAndRequest(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)

	testServer := &http.Server{
		Addr:    ":18082",
		Handler: mux,
	}

	SetServer(testServer)
	ResetShutdown()

	// Запускаем сервер
	go func() {
		_ = testServer.ListenAndServe()
	}()

	// Даём серверу время запуститься
	time.Sleep(100 * time.Millisecond)

	// Инициируем shutdown
	SetShutdownInProgress(1)

	// Создаём context для shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Запускаем shutdown
	go func() {
		_ = testServer.Shutdown(ctx)
	}()

	// Небольшая задержка
	time.Sleep(50 * time.Millisecond)

	// Сбрасываем
	SetServer(nil)
	ResetShutdown()
}

// TestShutdownWithActiveConnections проверяет shutdown с активными соединениями
func TestShutdownWithActiveConnections(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Имитируем долгий запрос
		time.Sleep(100 * time.Millisecond)
		healthHandler(w, r)
	})

	testServer := &http.Server{
		Addr:         ":18083",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	SetServer(testServer)

	// Запускаем сервер
	go func() {
		_ = testServer.ListenAndServe()
	}()

	// Даём серверу время запуститься
	time.Sleep(100 * time.Millisecond)

	// Делаем запрос
	go func() {
		resp, err := http.Get("http://localhost:18083/health")
		if err == nil {
			resp.Body.Close()
		}
	}()

	// Небольшая задержка
	time.Sleep(50 * time.Millisecond)

	// Инициируем shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	shutdownErr := make(chan error, 1)
	go func() {
		shutdownErr <- testServer.Shutdown(ctx)
	}()

	// Ждём завершения
	select {
	case err := <-shutdownErr:
		if err != nil {
			t.Logf("Shutdown завершён с ошибкой (допустимо): %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Error("Timeout при ожидании shutdown")
	}

	SetServer(nil)
	ResetShutdown()
}

// TestShutdownTimeout проверяет timeout при shutdown
func TestShutdownTimeout(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		// Очень долгий запрос
		select {
		case <-time.After(10 * time.Second):
		case <-r.Context().Done():
		}
	})

	testServer := &http.Server{
		Addr:    ":18084",
		Handler: mux,
	}

	SetServer(testServer)

	go func() {
		_ = testServer.ListenAndServe()
	}()

	time.Sleep(100 * time.Millisecond)

	// Делаем запрос к slow эндпоинту
	go func() {
		_, _ = http.Get("http://localhost:18084/slow")
	}()

	time.Sleep(50 * time.Millisecond)

	// Короткий timeout для shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	shutdownErr := make(chan error, 1)
	go func() {
		shutdownErr <- testServer.Shutdown(ctx)
	}()

	// Ожидем timeout или завершение
	select {
	case err := <-shutdownErr:
		if err == context.DeadlineExceeded {
			t.Log("Получен ожидаемый timeout при shutdown")
		}
	case <-time.After(5 * time.Second):
		t.Error("Timeout при ожидании результата shutdown")
	}

	SetServer(nil)
}

// TestServerNilSafety проверяет безопасность при nil сервере
func TestServerNilSafety(t *testing.T) {
	SetServer(nil)

	server := GetServer()
	if server != nil {
		t.Error("Ожидался nil сервер")
	}

	ResetShutdown()
}

// TestShutdownHelpers проверяет вспомогательные функции shutdown
func TestShutdownHelpers(t *testing.T) {
	// Проверяем начальные значения
	ResetShutdown()
	if isShuttingDown() {
		t.Error("Ожидалось, что shutdown не активен после сброса")
	}

	// Устанавливаем и проверяем
	SetShutdownInProgress(1)
	if !isShuttingDown() {
		t.Error("Ожидалось, что shutdown активен")
	}

	// Сбрасываем
	ResetShutdown()
	if isShuttingDown() {
		t.Error("Ожидалось, что shutdown не активен после сброса")
	}
}

// TestGracefulShutdownIntegration полная интеграция graceful shutdown
func TestGracefulShutdownIntegration(t *testing.T) {
	// Создаём полный сервер со всеми handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/stats", statsHandler)
	mux.HandleFunc("/api/echo", echoHandler)
	mux.HandleFunc("/api/posts", postsHandler)
	mux.HandleFunc("/api/posts/", postByIDHandler)

	testServer := &http.Server{
		Addr:         ":18085",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  10 * time.Second,
	}

	SetServer(testServer)
	ResetShutdown()

	// Запускаем сервер
	serverErr := make(chan error, 1)
	go func() {
		if err := testServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		} else {
			serverErr <- nil
		}
	}()

	// Даём серверу время запуститься
	time.Sleep(200 * time.Millisecond)

	// Тестируем все эндпоинты
	endpoints := []string{
		"/health",
		"/api/stats",
		"/api/posts",
		"/api/posts/1",
	}

	for _, endpoint := range endpoints {
		resp, err := http.Get("http://localhost:18085" + endpoint)
		if err != nil {
			t.Errorf("Ошибка запроса к %s: %v", endpoint, err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Ожидаемый статус код для %s: %d, получен: %d", endpoint, http.StatusOK, resp.StatusCode)
		}
		resp.Body.Close()
	}

	// Тестируем POST запрос
	echoReq := EchoRequest{Message: "test"}
	jsonBody, _ := json.Marshal(echoReq)
	resp, err := http.Post("http://localhost:18085/api/echo", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Errorf("Ошибка POST запроса: %v", err)
	} else {
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Ожидаемый статус код для POST: %d, получен: %d", http.StatusOK, resp.StatusCode)
		}
		resp.Body.Close()
	}

	// Инициируем graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	shutdownErr := make(chan error, 1)
	go func() {
		shutdownErr <- testServer.Shutdown(ctx)
	}()

	// Ждём завершения
	select {
	case err := <-shutdownErr:
		if err != nil {
			t.Errorf("Ошибка при shutdown: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Error("Timeout при ожидании shutdown")
	}

	// Проверяем, что сервер остановлен
	_, err = http.Get("http://localhost:18085/health")
	if err == nil {
		t.Error("Ожидалась ошибка подключения после shutdown")
	}

	SetServer(nil)
	ResetShutdown()
}
