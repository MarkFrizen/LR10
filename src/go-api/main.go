package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
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

// Author представляет автора поста в блоге
type Author struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Bio       string `json:"bio,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// Tag представляет тег поста
type Tag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// Comment представляет комментарий к посту
type Comment struct {
	ID        int64     `json:"id"`
	PostID    int64     `json:"post_id"`
	AuthorID  int64     `json:"author_id"`
	Author    *Author   `json:"author,omitempty"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// Post представляет пост в блоге
type Post struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	Content     string     `json:"content"`
	Excerpt     string     `json:"excerpt,omitempty"`
	Author      *Author    `json:"author"`
	Tags        []Tag      `json:"tags,omitempty"`
	Comments    []Comment  `json:"comments,omitempty"`
	ViewCount   int64      `json:"view_count"`
	PublishedAt time.Time  `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at,omitempty"`
}

// CreatePostRequest запрос на создание поста
type CreatePostRequest struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Excerpt string   `json:"excerpt,omitempty"`
	TagNames []string `json:"tag_names,omitempty"`
}

// CreatePostResponse ответ на создание поста
type CreatePostResponse struct {
	Post      Post     `json:"post"`
	Message   string   `json:"message"`
	Timestamp string   `json:"timestamp"`
}

// GetPostsResponse ответ на получение списка постов
type GetPostsResponse struct {
	Posts      []Post   `json:"posts"`
	Total      int      `json:"total"`
	Page       int      `json:"page"`
	PerPage    int      `json:"per_page"`
	Timestamp  string   `json:"timestamp"`
}

// PostResponse ответ на получение одного поста
type PostResponse struct {
	Post      Post     `json:"post"`
	Timestamp string   `json:"timestamp"`
}

// ErrorResponse структура ошибки
type ErrorResponse struct {
	Error     string   `json:"error"`
	Message   string   `json:"message"`
	Timestamp string   `json:"timestamp"`
}

var (
	stats     = Stats{RequestCount: 0}
	statsMu   sync.RWMutex
	startTime time.Time

	// Хранилище постов (in-memory)
	posts   = make(map[int64]Post)
	postsMu sync.RWMutex
	nextID  int64 = 1

	// Примеры данных для демонстрации
	defaultAuthor = &Author{
		ID:        1,
		Username:  "john_blogger",
		Email:     "john@example.com",
		Bio:       "Разработчик и технический писатель",
		AvatarURL: "https://example.com/avatars/john.jpg",
	}
	defaultTags = []Tag{
		{ID: 1, Name: "Go", Slug: "go"},
		{ID: 2, Name: "Backend", Slug: "backend"},
		{ID: 3, Name: "API", Slug: "api"},
	}
)

func main() {
	startTime = time.Now()
	stats.StartTime = startTime.Format(time.RFC3339)

	// Инициализация тестовыми данными
	initSamplePosts()

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/api/stats", statsHandler)
	http.HandleFunc("/api/echo", echoHandler)
	http.HandleFunc("/api/posts", postsHandler)
	http.HandleFunc("/api/posts/", postByIDHandler)

	port := ":8080"
	fmt.Printf("Go API сервер запущен на порту %s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

// initSamplePosts создаёт тестовые посты для демонстрации
func initSamplePosts() {
	now := time.Now()
	samplePosts := []Post{
		{
			ID:        1,
			Title:     "Введение в Go API",
			Slug:      "introduction-to-go-api",
			Content:   "Go (Golang) — это мощный язык программирования для создания высокопроизводительных API...",
			Excerpt:   "Изучаем основы создания API на Go",
			Author:    defaultAuthor,
			Tags:      defaultTags,
			ViewCount: 150,
			PublishedAt: now.Add(-24 * time.Hour),
			CreatedAt: now.Add(-48 * time.Hour),
		},
		{
			ID:        2,
			Title:     "Работа с JSON в Go",
			Slug:      "working-with-json-in-go",
			Content:   "Обработка JSON — одна из самых частых задач при разработке веб-сервисов...",
			Excerpt:   "Полное руководство по encoding/json",
			Author:    defaultAuthor,
			Tags:      []Tag{{ID: 1, Name: "Go", Slug: "go"}, {ID: 4, Name: "JSON", Slug: "json"}},
			ViewCount: 89,
			PublishedAt: now.Add(-12 * time.Hour),
			CreatedAt: now.Add(-24 * time.Hour),
		},
		{
			ID:        3,
			Title:     "Микросервисы: лучшие практики",
			Slug:      "microservices-best-practices",
			Content:   "При проектировании микросервисной архитектуры важно следовать определённым принципам...",
			Excerpt:   "Советы по построению микросервисов",
			Author:    defaultAuthor,
			Tags:      []Tag{{ID: 2, Name: "Backend", Slug: "backend"}, {ID: 5, Name: "Architecture", Slug: "architecture"}},
			ViewCount: 234,
			PublishedAt: now.Add(-6 * time.Hour),
			CreatedAt: now.Add(-12 * time.Hour),
		},
	}

	for _, post := range samplePosts {
		posts[post.ID] = post
	}
	nextID = 4
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

// postsHandler обрабатывает GET /api/posts (список) и POST /api/posts (создание)
func postsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getPostsHandler(w, r)
	case http.MethodPost:
		createPostHandler(w, r)
	default:
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// getPostsHandler возвращает список всех постов
func getPostsHandler(w http.ResponseWriter, r *http.Request) {
	incrementRequestCount()

	// Получаем параметры пагинации
	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	perPage, _ := strconv.Atoi(query.Get("per_page"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	postsMu.RLock()
	allPosts := make([]Post, 0, len(posts))
	for _, post := range posts {
		allPosts = append(allPosts, post)
	}
	postsMu.RUnlock()

	// Сортировка по ID (новые первыми)
	for i, j := 0, len(allPosts)-1; i < j; i, j = i+1, j-1 {
		allPosts[i], allPosts[j] = allPosts[j], allPosts[i]
	}

	// Пагинация
	total := len(allPosts)
	start := (page - 1) * perPage
	end := start + perPage

	var paginatedPosts []Post
	if start < total {
		if end > total {
			end = total
		}
		paginatedPosts = allPosts[start:end]
	} else {
		paginatedPosts = []Post{}
	}

	response := GetPostsResponse{
		Posts:     paginatedPosts,
		Total:     total,
		Page:      page,
		PerPage:   perPage,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// createPostHandler создаёт новый пост
func createPostHandler(w http.ResponseWriter, r *http.Request) {
	incrementRequestCount()

	var req CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if req.Title == "" || req.Content == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Title and content are required")
		return
	}

	postsMu.Lock()
	id := nextID
	nextID++

	// Создаём теги из названий
	tags := make([]Tag, 0, len(req.TagNames))
	for i, name := range req.TagNames {
		tags = append(tags, Tag{
			ID:   int64(i + 1),
			Name: name,
			Slug: strings.ToLower(strings.ReplaceAll(name, " ", "-")),
		})
	}

	post := Post{
		ID:        id,
		Title:     req.Title,
		Slug:      strings.ToLower(strings.ReplaceAll(req.Title, " ", "-")),
		Content:   req.Content,
		Excerpt:   req.Excerpt,
		Author:    defaultAuthor,
		Tags:      tags,
		Comments:  []Comment{},
		ViewCount: 0,
		CreatedAt: time.Now(),
	}
	posts[id] = post
	postsMu.Unlock()

	response := CreatePostResponse{
		Post:      post,
		Message:   "Post created successfully",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// postByIDHandler обрабатывает GET, PUT, DELETE для /api/posts/{id}
func postByIDHandler(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из пути
	path := strings.TrimPrefix(r.URL.Path, "/api/posts/")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid post ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		getPostByIDHandler(w, r, id)
	case http.MethodPut:
		updatePostHandler(w, r, id)
	case http.MethodDelete:
		deletePostHandler(w, r, id)
	default:
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// getPostByIDHandler возвращает пост по ID
func getPostByIDHandler(w http.ResponseWriter, r *http.Request, id int64) {
	incrementRequestCount()

	postsMu.RLock()
	post, exists := posts[id]
	postsMu.RUnlock()

	if !exists {
		writeErrorResponse(w, http.StatusNotFound, "Post not found")
		return
	}

	// Увеличиваем счётчик просмотров
	postsMu.Lock()
	post.ViewCount++
	posts[id] = post
	postsMu.Unlock()

	response := PostResponse{
		Post:      post,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// updatePostHandler обновляет пост
func updatePostHandler(w http.ResponseWriter, r *http.Request, id int64) {
	incrementRequestCount()

	postsMu.RLock()
	post, exists := posts[id]
	postsMu.RUnlock()

	if !exists {
		writeErrorResponse(w, http.StatusNotFound, "Post not found")
		return
	}

	var req CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	postsMu.Lock()
	if req.Title != "" {
		post.Title = req.Title
		post.Slug = strings.ToLower(strings.ReplaceAll(req.Title, " ", "-"))
	}
	if req.Content != "" {
		post.Content = req.Content
	}
	if req.Excerpt != "" {
		post.Excerpt = req.Excerpt
	}
	if len(req.TagNames) > 0 {
		post.Tags = make([]Tag, 0, len(req.TagNames))
		for i, name := range req.TagNames {
			post.Tags = append(post.Tags, Tag{
				ID:   int64(i + 1),
				Name: name,
				Slug: strings.ToLower(strings.ReplaceAll(name, " ", "-")),
			})
		}
	}
	post.UpdatedAt = time.Now()
	posts[id] = post
	postsMu.Unlock()

	response := PostResponse{
		Post:      post,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// deletePostHandler удаляет пост
func deletePostHandler(w http.ResponseWriter, r *http.Request, id int64) {
	incrementRequestCount()

	postsMu.Lock()
	_, exists := posts[id]
	if !exists {
		postsMu.Unlock()
		writeErrorResponse(w, http.StatusNotFound, "Post not found")
		return
	}
	delete(posts, id)
	postsMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// writeErrorResponse записывает ответ с ошибкой
func writeErrorResponse(w http.ResponseWriter, status int, message string) {
	response := ErrorResponse{
		Error:     http.StatusText(status),
		Message:   message,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}
