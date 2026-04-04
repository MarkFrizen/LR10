package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// --- Helper ---

func newTestApp() *App {
	app := NewApp()
	app.SetupHTTP()
	return app
}

// --- Health Handler Tests ---

func TestHealthHandler(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	app.healthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected application/json, got %s", contentType)
	}

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	if response.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %s", response.Status)
	}
	if response.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", response.Version)
	}
	if response.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
}

func TestHealthHandlerMethodNotAllowed(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()

	app.healthHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// --- Stats Handler Tests ---

func TestStatsHandler(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()

	app.statsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, w.Code)
	}

	var response Stats
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	if response.RequestCount < 1 {
		t.Errorf("expected RequestCount >= 1, got %d", response.RequestCount)
	}
	if response.Uptime == "" {
		t.Error("expected non-empty uptime")
	}
	if response.StartTime == "" {
		t.Error("expected non-empty start_time")
	}
}

func TestStatsHandlerMethodNotAllowed(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodPost, "/api/stats", nil)
	w := httptest.NewRecorder()

	app.statsHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// --- Echo Handler Tests ---

func TestEchoHandler(t *testing.T) {
	app := newTestApp()

	requestBody := EchoRequest{
		Message: "Hello from test!",
		Data:    map[string]interface{}{"key": "value", "number": 42},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("JSON marshal error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/echo", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.echoHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, w.Code)
	}

	var response EchoResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	if response.Original.Message != "Hello from test!" {
		t.Errorf("expected message 'Hello from test!', got %s", response.Original.Message)
	}

	expectedProcessed := "Received: Hello from test! (processed by Go API)"
	if response.Processed != expectedProcessed {
		t.Errorf("expected processed '%s', got %s", expectedProcessed, response.Processed)
	}
}

func TestEchoHandlerInvalidJSON(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest(http.MethodPost, "/api/echo", bytes.NewBuffer([]byte(`{"message": "invalid`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.echoHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestEchoHandlerMethodNotAllowed(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/echo", nil)
	w := httptest.NewRecorder()

	app.echoHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// --- Request Counter Tests ---

func TestRequestCounterIncrement(t *testing.T) {
	rc := NewRequestCounter()

	for i := 0; i < 5; i++ {
		rc.Increment()
	}

	stats := rc.Get()
	if stats.RequestCount != 5 {
		t.Errorf("expected count 5, got %d", stats.RequestCount)
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
		t.Errorf("expected count 100, got %d", stats.RequestCount)
	}
}

// --- JSON Format Tests ---

func TestHealthResponseJSONFormat(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	app.healthHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	for _, field := range []string{"status", "timestamp", "version"} {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("missing field: %s", field)
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
		t.Fatalf("JSON decode error: %v", err)
	}

	for _, field := range []string{"request_count", "uptime", "start_time"} {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("missing field: %s", field)
		}
	}
}

func TestEchoResponseJSONFormat(t *testing.T) {
	app := newTestApp()

	jsonBody, _ := json.Marshal(EchoRequest{Message: "test"})
	req := httptest.NewRequest(http.MethodPost, "/api/echo", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.echoHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	for _, field := range []string{"original", "processed", "timestamp"} {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("missing field: %s", field)
		}
	}
}

// --- Posts Handler Tests ---

func TestGetPostsHandler(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, w.Code)
	}

	var response GetPostsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	if response.Total == 0 {
		t.Error("expected total > 0")
	}
	if response.Page != 1 {
		t.Errorf("expected page 1, got %d", response.Page)
	}
	if len(response.Posts) == 0 {
		t.Error("expected non-empty posts list")
	}
}

func TestGetPostsHandlerPagination(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/posts?page=1&per_page=2", nil)
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, w.Code)
	}

	var response GetPostsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	if len(response.Posts) > 2 {
		t.Errorf("expected at most 2 posts, got %d", len(response.Posts))
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
		t.Fatalf("JSON decode error: %v", err)
	}

	for i := 1; i < len(response.Posts); i++ {
		if response.Posts[i-1].ID < response.Posts[i].ID {
			t.Errorf("posts not sorted by descending ID: [%d] = %d, [%d] = %d",
				i-1, response.Posts[i-1].ID, i, response.Posts[i].ID)
		}
	}
}

func TestCreatePostHandler(t *testing.T) {
	app := newTestApp()

	createReq := CreatePostRequest{
		Title:    "Test Post",
		Content:  "Test content",
		Excerpt:  "Test excerpt",
		TagNames: []string{"Test", "Go"},
	}

	jsonBody, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected %d, got %d", http.StatusCreated, w.Code)
	}

	var response CreatePostResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	if response.Post.Title != createReq.Title {
		t.Errorf("expected title '%s', got '%s'", createReq.Title, response.Post.Title)
	}
	if len(response.Post.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(response.Post.Tags))
	}
	if response.Message != "Post created successfully" {
		t.Errorf("expected message 'Post created successfully', got '%s'", response.Message)
	}
}

func TestCreatePostHandlerEmptyTitle(t *testing.T) {
	app := newTestApp()

	jsonBody, _ := json.Marshal(CreatePostRequest{Title: "", Content: "content"})
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestCreatePostHandlerEmptyContent(t *testing.T) {
	app := newTestApp()

	jsonBody, _ := json.Marshal(CreatePostRequest{Title: "Title", Content: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// --- Post By ID Handler Tests ---

func TestGetPostByIDHandler(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/posts/1", nil)
	w := httptest.NewRecorder()

	app.postByIDHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, w.Code)
	}

	var response PostResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	if response.Post.ID != 1 {
		t.Errorf("expected ID 1, got %d", response.Post.ID)
	}
	if response.Post.Author == nil {
		t.Error("expected non-nil author")
	}
}

func TestGetPostByIDHandlerNotFound(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/posts/9999", nil)
	w := httptest.NewRecorder()

	app.postByIDHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, w.Code)
	}

	var errorResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errorResp); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	if errorResp.Message != "Post not found" {
		t.Errorf("expected message 'Post not found', got '%s'", errorResp.Message)
	}
}

// --- Update Post Handler Tests ---

func TestUpdatePostHandler(t *testing.T) {
	app := newTestApp()

	// Создаём тестовый пост
	store := app.GetStore()
	store.Create(Post{
		Title:   "Original Title",
		Content: "Original content",
		Author:  &Author{ID: 1, Username: "test", Email: "test@test.com"},
	})
	// Получаем ID созданного поста
	allPosts, _ := store.GetAll(1, 100)
	var testPostID int64
	for _, p := range allPosts {
		if p.Title == "Original Title" {
			testPostID = p.ID
			break
		}
	}

	updateReq := UpdatePostRequest{
		Title:    "Updated Title",
		Excerpt:  "New excerpt",
		TagNames: []string{"Updated"},
	}

	jsonBody, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPut, "/api/posts/"+formatID(testPostID), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postByIDHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, w.Code)
	}

	var response PostResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	if response.Post.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got '%s'", response.Post.Title)
	}
	if response.Post.Excerpt != "New excerpt" {
		t.Errorf("expected excerpt 'New excerpt', got '%s'", response.Post.Excerpt)
	}
}

func TestUpdatePostHandlerNotFound(t *testing.T) {
	app := newTestApp()

	jsonBody, _ := json.Marshal(UpdatePostRequest{Title: "Updated"})
	req := httptest.NewRequest(http.MethodPut, "/api/posts/9999", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postByIDHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, w.Code)
	}
}

// --- Delete Post Handler Tests ---

func TestDeletePostHandler(t *testing.T) {
	app := newTestApp()

	store := app.GetStore()
	id := store.Create(Post{
		Title:   "To Delete",
		Content: "Will be deleted",
		Author:  &Author{ID: 1, Username: "test", Email: "test@test.com"},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/posts/"+formatID(id), nil)
	w := httptest.NewRecorder()

	app.postByIDHandler(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected %d, got %d", http.StatusNoContent, w.Code)
	}

	_, exists := store.GetByID(id)
	if exists {
		t.Error("expected post to be deleted")
	}
}

func TestDeletePostHandlerNotFound(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodDelete, "/api/posts/9999", nil)
	w := httptest.NewRecorder()

	app.postByIDHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestPostByIDHandlerInvalidID(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/posts/invalid", nil)
	w := httptest.NewRecorder()

	app.postByIDHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestPostsHandlerMethodNotAllowed(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodPatch, "/api/posts", nil)
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// --- Post Response JSON Format Tests ---

func TestPostComplexJSONStructure(t *testing.T) {
	app := newTestApp()

	createReq := CreatePostRequest{
		Title:    "Complex Post",
		Content:  "Content with complex structure",
		TagNames: []string{"Tag1", "Tag2", "Tag3", "Tag4", "Tag5"},
	}

	jsonBody, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	for _, field := range []string{"post", "message", "timestamp"} {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("missing field: %s", field)
		}
	}

	postData, ok := rawJSON["post"].(map[string]interface{})
	if !ok {
		t.Fatal("field 'post' is not an object")
	}

	for _, field := range []string{"id", "title", "slug", "content", "author", "tags", "view_count", "created_at"} {
		if _, exists := postData[field]; !exists {
			t.Errorf("missing field in post: %s", field)
		}
	}

	tagsData, ok := postData["tags"].([]interface{})
	if !ok {
		t.Fatal("field 'tags' is not an array")
	}
	if len(tagsData) != 5 {
		t.Errorf("expected 5 tags, got %d", len(tagsData))
	}
}

func TestGetPostsResponseJSONFormat(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	for _, field := range []string{"posts", "total", "page", "per_page", "timestamp"} {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("missing field: %s", field)
		}
	}
}

func TestCreatePostResponseJSONFormat(t *testing.T) {
	app := newTestApp()

	jsonBody, _ := json.Marshal(CreatePostRequest{Title: "Format Test", Content: "Content"})
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	var rawJSON map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&rawJSON); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	for _, field := range []string{"post", "message", "timestamp"} {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("missing field: %s", field)
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
		t.Fatalf("JSON decode error: %v", err)
	}

	for _, field := range []string{"post", "timestamp"} {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("missing field: %s", field)
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
		t.Fatalf("JSON decode error: %v", err)
	}

	for _, field := range []string{"error", "message", "timestamp"} {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("missing field: %s", field)
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
		t.Fatalf("JSON decode error: %v", err)
	}

	if response.Post.Author == nil {
		t.Fatal("Author must be set")
	}
	if response.Post.Author.Username == "" {
		t.Error("Author username must not be empty")
	}
	if response.Post.Author.Email == "" {
		t.Error("Author email must not be empty")
	}
}

func TestTagsArrayStructure(t *testing.T) {
	app := newTestApp()

	jsonBody, _ := json.Marshal(CreatePostRequest{
		Title:    "Tag Test",
		Content:  "Content",
		TagNames: []string{"Go", "API", "Testing"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postsHandler(w, req)

	var response CreatePostResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	if len(response.Post.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(response.Post.Tags))
	}

	for _, tag := range response.Post.Tags {
		if tag.Name == "" {
			t.Error("Tag name must not be empty")
		}
		if tag.Slug == "" {
			t.Error("Tag slug must not be empty")
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
		t.Fatalf("JSON decode error: %v", err)
	}
	firstCount := resp1.Post.ViewCount

	// Второй запрос
	req2 := httptest.NewRequest(http.MethodGet, "/api/posts/1", nil)
	w2 := httptest.NewRecorder()
	app.postByIDHandler(w2, req2)

	var resp2 PostResponse
	if err := json.NewDecoder(w2.Body).Decode(&resp2); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	if resp2.Post.ViewCount != firstCount+1 {
		t.Errorf("expected view count %d, got %d", firstCount+1, resp2.Post.ViewCount)
	}
}

func TestUpdatePostPartialUpdate(t *testing.T) {
	app := newTestApp()

	store := app.GetStore()
	id := store.Create(Post{
		Title:   "Original",
		Content: "Original content",
		Excerpt: "Original excerpt",
		Author:  &Author{ID: 1, Username: "test", Email: "test@test.com"},
	})

	// Обновляем только заголовок
	updateReq := UpdatePostRequest{Title: "Updated Title"}
	jsonBody, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPut, "/api/posts/"+formatID(id), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.postByIDHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, w.Code)
	}

	var response PostResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	if response.Post.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got '%s'", response.Post.Title)
	}
	// Content должен остаться прежним
	if response.Post.Content != "Original content" {
		t.Errorf("expected content 'Original content', got '%s'", response.Post.Content)
	}
}

// --- Integration Test ---

func TestIntegrationAllEndpoints(t *testing.T) {
	app := newTestApp()
	mux := app.SetupHTTP()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Health
	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health: expected 200, got %d", resp.StatusCode)
	}

	// Stats
	resp, err = http.Get(server.URL + "/api/stats")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("stats: expected 200, got %d", resp.StatusCode)
	}

	// Echo
	echoReq := EchoRequest{Message: "integration test"}
	jsonBody, _ := json.Marshal(echoReq)
	resp, err = http.Post(server.URL+"/api/echo", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("echo: expected 200, got %d", resp.StatusCode)
	}

	// Posts
	resp, err = http.Get(server.URL + "/api/posts")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("posts: expected 200, got %d", resp.StatusCode)
	}
}

// Helper: форматирование ID в строку для URL
func formatID(id int64) string {
	return strconv.FormatInt(id, 10)
}
