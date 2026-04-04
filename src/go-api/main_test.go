package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// --- Вспомогательные функции ---

func newTestApp() *App {
	app := NewApp()
	app.SetupHTTP()
	return app
}

// --- Тесты эндпоинта здоровья ---

func TestHealthHandler(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	app.healthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ожидалось %d, получено %d", http.StatusOK, w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("ожидалось application/json, получено %s", contentType)
	}

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	if response.Status != "работает" {
		t.Errorf("ожидался status 'работает', получено %s", response.Status)
	}
	if response.Version != apiVersion {
		t.Errorf("ожидалась версия '%s', получено %s", apiVersion, response.Version)
	}
	if response.Timestamp == "" {
		t.Error("ожидался непустой timestamp")
	}
}

func TestHealthHandlerMethodNotAllowed(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()

	app.healthHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("ожидалось %d, получено %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// --- Тесты эндпоинта статистики ---

func TestStatsHandler(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()

	app.statsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ожидалось %d, получено %d", http.StatusOK, w.Code)
	}

	var response Stats
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	if response.RequestCount < 1 {
		t.Errorf("ожидалось RequestCount >= 1, получено %d", response.RequestCount)
	}
	if response.Uptime == "" {
		t.Error("ожидался непустой uptime")
	}
	if response.StartTime == "" {
		t.Error("ожидался непустой start_time")
	}
}

func TestStatsHandlerMethodNotAllowed(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodPost, "/api/stats", nil)
	w := httptest.NewRecorder()

	app.statsHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("ожидалось %d, получено %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// --- Тесты эндпоинта «эхо» ---

func TestEchoHandler(t *testing.T) {
	app := newTestApp()

	requestBody := EchoRequest{
		Message: "Привет из теста!",
		Data:    map[string]interface{}{"ключ": "значение", "число": 42},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Ошибка маршалинга JSON: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/echo", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.echoHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ожидалось %d, получено %d", http.StatusOK, w.Code)
	}

	var response EchoResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	if response.Original.Message != "Привет из теста!" {
		t.Errorf("ожидалось message 'Привет из теста!', получено %s", response.Original.Message)
	}

	expected := "Получено: Привет из теста! (обработано REST API)"
	if response.Processed != expected {
		t.Errorf("ожидалось processed '%s', получено %s", expected, response.Processed)
	}
}

func TestEchoHandlerInvalidJSON(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest(http.MethodPost, "/api/echo", bytes.NewBuffer([]byte(`{"message": "некорректный`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.echoHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("ожидалось %d, получено %d", http.StatusBadRequest, w.Code)
	}
}

func TestEchoHandlerMethodNotAllowed(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/echo", nil)
	w := httptest.NewRecorder()

	app.echoHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("ожидалось %d, получено %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// --- Тесты счётчика запросов ---

func TestRequestCounterIncrement(t *testing.T) {
	rc := NewRequestCounter()

	for i := 0; i < 5; i++ {
		rc.Increment()
	}

	stats := rc.Get()
	if stats.RequestCount != 5 {
		t.Errorf("ожидалось 5, получено %d", stats.RequestCount)
	}
}

func TestConcurrentRequests(t *testing.T) {
	rc := NewRequestCounter()

	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			rc.Increment()
			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	stats := rc.Get()
	if stats.RequestCount != 100 {
		t.Errorf("ожидалось 100, получено %d", stats.RequestCount)
	}
}

// --- Тесты формата JSON ---

func TestHealthResponseJSONFormat(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	app.healthHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	for _, field := range []string{"status", "timestamp", "version"} {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("отсутствует поле: %s", field)
		}
	}
}

func TestStatsResponseJSONFormat(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()

	app.statsHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	for _, field := range []string{"request_count", "uptime", "start_time"} {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("отсутствует поле: %s", field)
		}
	}
}

func TestEchoResponseJSONFormat(t *testing.T) {
	app := newTestApp()

	jsonBody, _ := json.Marshal(EchoRequest{Message: "тест"})
	req := httptest.NewRequest(http.MethodPost, "/api/echo", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.echoHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	for _, field := range []string{"original", "processed", "timestamp"} {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("отсутствует поле: %s", field)
		}
	}
}

// --- Тесты обработчиков постов ---

func TestGetPostsHandler(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ожидалось %d, получено %d", http.StatusOK, w.Code)
	}

	var response GetPostsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	if response.Total == 0 {
		t.Error("ожидалось total > 0")
	}
	if response.Page != 1 {
		t.Errorf("ожидалась страница 1, получено %d", response.Page)
	}
	if len(response.Posts) == 0 {
		t.Error("ожидался непустой список постов")
	}
}

func TestGetPostsHandlerPagination(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/posts?page=1&per_page=2", nil)
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ожидалось %d, получено %d", http.StatusOK, w.Code)
	}

	var response GetPostsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	if len(response.Posts) > 2 {
		t.Errorf("ожидалось не более 2 постов, получено %d", len(response.Posts))
	}
}

// Проверка, что посты отсортированы по ID (новые первыми)
func TestGetPostsHandlerSorting(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/posts?per_page=10", nil)
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	var response GetPostsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	for i := 1; i < len(response.Posts); i++ {
		if response.Posts[i-1].ID < response.Posts[i].ID {
			t.Errorf("посты не отсортированы по убыванию ID: [%d] = %d, [%d] = %d",
				i-1, response.Posts[i-1].ID, i, response.Posts[i].ID)
		}
	}
}

func TestCreatePostHandler(t *testing.T) {
	app := newTestApp()

	createReq := CreatePostRequest{
		Title:    "Тестовый пост",
		Content:  "Тестовое содержание",
		Excerpt:  "Тестовое описание",
		TagNames: []string{"Тест", "Go"},
	}

	jsonBody, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("ожидалось %d, получено %d", http.StatusCreated, w.Code)
	}

	var response CreatePostResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	if response.Post.Title != createReq.Title {
		t.Errorf("ожидался заголовок '%s', получено '%s'", createReq.Title, response.Post.Title)
	}
	if len(response.Post.Tags) != 2 {
		t.Errorf("ожидалось 2 тега, получено %d", len(response.Post.Tags))
	}
	if response.Message != "Пост успешно создан" {
		t.Errorf("ожидалось сообщение 'Пост успешно создан', получено '%s'", response.Message)
	}
}

func TestCreatePostHandlerEmptyTitle(t *testing.T) {
	app := newTestApp()

	jsonBody, _ := json.Marshal(CreatePostRequest{Title: "", Content: "содержание"})
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("ожидалось %d, получено %d", http.StatusBadRequest, w.Code)
	}
}

func TestCreatePostHandlerEmptyContent(t *testing.T) {
	app := newTestApp()

	jsonBody, _ := json.Marshal(CreatePostRequest{Title: "Заголовок", Content: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("ожидалось %d, получено %d", http.StatusBadRequest, w.Code)
	}
}

// --- Тесты получения поста по ID ---

func TestGetPostByIDHandler(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/posts/1", nil)
	w := httptest.NewRecorder()

	app.postByIDHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ожидалось %d, получено %d", http.StatusOK, w.Code)
	}

	var response PostResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	if response.Post.ID != 1 {
		t.Errorf("ожидался ID 1, получено %d", response.Post.ID)
	}
	if response.Post.Author == nil {
		t.Error("ожидался непустой author")
	}
}

func TestGetPostByIDHandlerNotFound(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/posts/9999", nil)
	w := httptest.NewRecorder()

	app.postByIDHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("ожидалось %d, получено %d", http.StatusNotFound, w.Code)
	}

	var errorResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errorResp); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	if errorResp.Message != "Пост не найден" {
		t.Errorf("ожидалось сообщение 'Пост не найден', получено '%s'", errorResp.Message)
	}
}

// --- Тесты обновления поста ---

func TestUpdatePostHandler(t *testing.T) {
	app := newTestApp()

	// Создаём тестовый пост
	store := app.GetStore()
	store.Create(Post{
		Title:   "Оригинальный заголовок",
		Content: "Оригинальное содержание",
		Author:  &Author{ID: 1, Username: "тест", Email: "test@test.com"},
	})
	// Получаем ID созданного поста
	allPosts, _ := store.GetAll(1, 100)
	var testPostID int64
	for _, p := range allPosts {
		if p.Title == "Оригинальный заголовок" {
			testPostID = p.ID
			break
		}
	}

	updateReq := UpdatePostRequest{
		Title:    "Обновлённый заголовок",
		Excerpt:  "Новое описание",
		TagNames: []string{"Обновлено"},
	}

	jsonBody, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPut, "/api/posts/"+formatID(testPostID), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postByIDHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ожидалось %d, получено %d", http.StatusOK, w.Code)
	}

	var response PostResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	if response.Post.Title != "Обновлённый заголовок" {
		t.Errorf("ожидался заголовок 'Обновлённый заголовок', получено '%s'", response.Post.Title)
	}
	if response.Post.Excerpt != "Новое описание" {
		t.Errorf("ожидалось описание 'Новое описание', получено '%s'", response.Post.Excerpt)
	}
}

func TestUpdatePostHandlerNotFound(t *testing.T) {
	app := newTestApp()

	jsonBody, _ := json.Marshal(UpdatePostRequest{Title: "Обновлённый"})
	req := httptest.NewRequest(http.MethodPut, "/api/posts/9999", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postByIDHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("ожидалось %d, получено %d", http.StatusNotFound, w.Code)
	}
}

// --- Тесты удаления поста ---

func TestDeletePostHandler(t *testing.T) {
	app := newTestApp()

	store := app.GetStore()
	id := store.Create(Post{
		Title:   "На удаление",
		Content: "Будет удалён",
		Author:  &Author{ID: 1, Username: "тест", Email: "test@test.com"},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/posts/"+formatID(id), nil)
	w := httptest.NewRecorder()

	app.postByIDHandler(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("ожидалось %d, получено %d", http.StatusNoContent, w.Code)
	}

	_, exists := store.GetByID(id)
	if exists {
		t.Error("ожидалось, что пост будет удалён")
	}
}

func TestDeletePostHandlerNotFound(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodDelete, "/api/posts/9999", nil)
	w := httptest.NewRecorder()

	app.postByIDHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("ожидалось %d, получено %d", http.StatusNotFound, w.Code)
	}
}

func TestPostByIDHandlerInvalidID(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/posts/invalid", nil)
	w := httptest.NewRecorder()

	app.postByIDHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("ожидалось %d, получено %d", http.StatusBadRequest, w.Code)
	}
}

func TestPostsHandlerMethodNotAllowed(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodPatch, "/api/posts", nil)
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("ожидалось %d, получено %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// --- Тесты формата ответа поста ---

func TestPostComplexJSONStructure(t *testing.T) {
	app := newTestApp()

	createReq := CreatePostRequest{
		Title:    "Сложный пост",
		Content:  "Содержание сложной структуры",
		TagNames: []string{"Тег1", "Тег2", "Тег3", "Тег4", "Тег5"},
	}

	jsonBody, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	for _, field := range []string{"post", "message", "timestamp"} {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("отсутствует поле: %s", field)
		}
	}

	postData, ok := rawJSON["post"].(map[string]interface{})
	if !ok {
		t.Fatal("поле 'post' не является объектом")
	}

	for _, field := range []string{"id", "title", "slug", "content", "author", "tags", "view_count", "created_at"} {
		if _, exists := postData[field]; !exists {
			t.Errorf("отсутствует поле в post: %s", field)
		}
	}

	tagsData, ok := postData["tags"].([]interface{})
	if !ok {
		t.Fatal("поле 'tags' не является массивом")
	}
	if len(tagsData) != 5 {
		t.Errorf("ожидалось 5 тегов, получено %d", len(tagsData))
	}
}

func TestGetPostsResponseJSONFormat(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	for _, field := range []string{"posts", "total", "page", "per_page", "timestamp"} {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("отсутствует поле: %s", field)
		}
	}
}

func TestCreatePostResponseJSONFormat(t *testing.T) {
	app := newTestApp()

	jsonBody, _ := json.Marshal(CreatePostRequest{Title: "Тест формата", Content: "Содержание"})
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	for _, field := range []string{"post", "message", "timestamp"} {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("отсутствует поле: %s", field)
		}
	}
}

func TestPostResponseJSONFormat(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/posts/1", nil)
	w := httptest.NewRecorder()

	app.postByIDHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	for _, field := range []string{"post", "timestamp"} {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("отсутствует поле: %s", field)
		}
	}
}

func TestErrorResponseJSONFormat(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/posts/99999", nil)
	w := httptest.NewRecorder()

	app.postByIDHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	for _, field := range []string{"error", "message", "timestamp"} {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("отсутствует поле: %s", field)
		}
	}
}

func TestAuthorNestedStructure(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/posts/1", nil)
	w := httptest.NewRecorder()

	app.postByIDHandler(w, req)

	var response PostResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	if response.Post.Author == nil {
		t.Fatal("Author должен быть установлен")
	}
	if response.Post.Author.Username == "" {
		t.Error("Username автора не должен быть пустым")
	}
	if response.Post.Author.Email == "" {
		t.Error("Email автора не должен быть пустым")
	}
}

func TestTagsArrayStructure(t *testing.T) {
	app := newTestApp()

	jsonBody, _ := json.Marshal(CreatePostRequest{
		Title:    "Тест тегов",
		Content:  "Содержание",
		TagNames: []string{"Go", "API", "Тестирование"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	var response CreatePostResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	if len(response.Post.Tags) != 3 {
		t.Errorf("ожидалось 3 тега, получено %d", len(response.Post.Tags))
	}

	for _, tag := range response.Post.Tags {
		if tag.Name == "" {
			t.Error("Название тега не должно быть пустым")
		}
		if tag.Slug == "" {
			t.Error("Slug тега не должен быть пустым")
		}
	}
}

func TestViewCountIncrement(t *testing.T) {
	app := newTestApp()

	// Первый запрос
	req1 := httptest.NewRequest(http.MethodGet, "/api/posts/1", nil)
	w1 := httptest.NewRecorder()
	app.postByIDHandler(w1, req1)

	var resp1 PostResponse
	if err := json.NewDecoder(w1.Body).Decode(&resp1); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}
	firstCount := resp1.Post.ViewCount

	// Второй запрос
	req2 := httptest.NewRequest(http.MethodGet, "/api/posts/1", nil)
	w2 := httptest.NewRecorder()
	app.postByIDHandler(w2, req2)

	var resp2 PostResponse
	if err := json.NewDecoder(w2.Body).Decode(&resp2); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	if resp2.Post.ViewCount != firstCount+1 {
		t.Errorf("ожидался счётчик %d, получено %d", firstCount+1, resp2.Post.ViewCount)
	}
}

func TestUpdatePostPartialUpdate(t *testing.T) {
	app := newTestApp()

	store := app.GetStore()
	id := store.Create(Post{
		Title:   "Оригинал",
		Content: "Оригинальное содержание",
		Excerpt: "Оригинальное описание",
		Author:  &Author{ID: 1, Username: "тест", Email: "test@test.com"},
	})

	// Обновляем только заголовок
	updateReq := UpdatePostRequest{Title: "Обновлённый заголовок"}
	jsonBody, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPut, "/api/posts/"+formatID(id), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postByIDHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ожидалось %d, получено %d", http.StatusOK, w.Code)
	}

	var response PostResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	if response.Post.Title != "Обновлённый заголовок" {
		t.Errorf("ожидался заголовок 'Обновлённый заголовок', получено '%s'", response.Post.Title)
	}
	// Содержание должно остаться прежним
	if response.Post.Content != "Оригинальное содержание" {
		t.Errorf("ожидалось содержание 'Оригинальное содержание', получено '%s'", response.Post.Content)
	}
}

// --- Тесты создания поста с указанием автора ---

func TestCreatePostWithAuthorID(t *testing.T) {
	app := newTestApp()

	// Создаём пост с указанием ID автора
	jsonBody, _ := json.Marshal(CreatePostRequest{
		Title:    "Пост с автором",
		Content:  "Содержание",
		AuthorID: 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("ожидалось %d, получено %d", http.StatusCreated, w.Code)
	}

	var response CreatePostResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	if response.Post.Author == nil {
		t.Fatal("Author должен быть установлен")
	}
	if response.Post.Author.ID != 1 {
		t.Errorf("ожидался Author.ID = 1, получено %d", response.Post.Author.ID)
	}
	if response.Post.Author.Username != "john_blogger" {
		t.Errorf("ожидался Username 'john_blogger', получено '%s'", response.Post.Author.Username)
	}
}

func TestCreatePostWithUnknownAuthor(t *testing.T) {
	app := newTestApp()

	// Создаём пост с несуществующим ID автора
	jsonBody, _ := json.Marshal(CreatePostRequest{
		Title:    "Пост с неизвестным автором",
		Content:  "Содержание",
		AuthorID: 999,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("ожидалось %d, получено %d", http.StatusBadRequest, w.Code)
	}

	var errorResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errorResp); err != nil {
		t.Fatalf("Ошибка декоодирования JSON: %v", err)
	}

	if errorResp.Message == "" {
		t.Error("ожидалось сообщение об ошибке")
	}
}

// --- Интеграционный тест ---

func TestIntegrationAllEndpoints(t *testing.T) {
	app := newTestApp()
	mux := app.SetupHTTP()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Здоровье
	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("ошибка запроса: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("здоровье: ожидалось 200, получено %d", resp.StatusCode)
	}

	// Статистика
	resp, err = http.Get(server.URL + "/api/stats")
	if err != nil {
		t.Fatalf("ошибка запроса: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("статистика: ожидалось 200, получено %d", resp.StatusCode)
	}

	// Эхо
	echoReq := EchoRequest{Message: "интеграционный тест"}
	jsonBody, _ := json.Marshal(echoReq)
	resp, err = http.Post(server.URL+"/api/echo", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatalf("ошибка запроса: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("эхо: ожидалось 200, получено %d", resp.StatusCode)
	}

	// Посты
	resp, err = http.Get(server.URL + "/api/posts")
	if err != nil {
		t.Fatalf("ошибка запроса: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("посты: ожидалось 200, получено %d", resp.StatusCode)
	}
}

// formatID — форматирование ID в строку для URL
func formatID(id int64) string {
	return strconv.FormatInt(id, 10)
}
