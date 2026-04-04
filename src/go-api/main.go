package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// --- Модели данных ---

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
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Excerpt  string   `json:"excerpt,omitempty"`
	TagNames []string `json:"tag_names,omitempty"`
}

// UpdatePostRequest запрос на обновление поста
type UpdatePostRequest struct {
	Title    string   `json:"title,omitempty"`
	Content  string   `json:"content,omitempty"`
	Excerpt  string   `json:"excerpt,omitempty"`
	TagNames []string `json:"tag_names,omitempty"`
}

// CreatePostResponse ответ на создание поста
type CreatePostResponse struct {
	Post      Post   `json:"post"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// GetPostsResponse ответ на получение списка постов
type GetPostsResponse struct {
	Posts     []Post `json:"posts"`
	Total     int    `json:"total"`
	Page      int    `json:"page"`
	PerPage   int    `json:"per_page"`
	Timestamp string `json:"timestamp"`
}

// PostResponse ответ на получение одного поста
type PostResponse struct {
	Post      Post   `json:"post"`
	Timestamp string `json:"timestamp"`
}

// ErrorResponse структура ошибки
type ErrorResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// --- Хранилище постов ---

// PostStore — потокобезопасное хранилище постов
type PostStore struct {
	mu     sync.RWMutex
	posts  map[int64]Post
	nextID int64
}

// NewPostStore создаёт новое хранилище
func NewPostStore() *PostStore {
	return &PostStore{
		posts:  make(map[int64]Post),
		nextID: 1,
	}
}

// GetAll возвращает все посты с пагинацией, отсортированные по ID (новые первыми)
func (s *PostStore) GetAll(page, perPage int) ([]Post, int) {
	s.mu.RLock()
	allPosts := make([]Post, 0, len(s.posts))
	for _, post := range s.posts {
		allPosts = append(allPosts, post)
	}
	s.mu.RUnlock()

	// Сортировка по ID (новые первыми)
	sort.Slice(allPosts, func(i, j int) bool {
		return allPosts[i].ID > allPosts[j].ID
	})

	total := len(allPosts)
	start := (page - 1) * perPage
	end := start + perPage

	if start >= total {
		return []Post{}, total
	}
	if end > total {
		end = total
	}

	return allPosts[start:end], total
}

// GetByID возвращает пост по ID
func (s *PostStore) GetByID(id int64) (Post, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	post, exists := s.posts[id]
	return post, exists
}

// Create добавляет новый пост и возвращает его ID
func (s *PostStore) Create(post Post) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextID
	post.ID = id
	s.nextID++
	s.posts[id] = post
	return id
}

// Update обновляет пост по ID
func (s *PostStore) Update(id int64, update UpdatePostRequest) (Post, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	post, exists := s.posts[id]
	if !exists {
		return Post{}, false
	}
	if update.Title != "" {
		post.Title = update.Title
		post.Slug = strings.ToLower(strings.ReplaceAll(update.Title, " ", "-"))
	}
	if update.Content != "" {
		post.Content = update.Content
	}
	if update.Excerpt != "" {
		post.Excerpt = update.Excerpt
	}
	if len(update.TagNames) > 0 {
		post.Tags = makeTagsFromNames(update.TagNames)
	}
	post.UpdatedAt = time.Now()
	s.posts[id] = post
	return post, true
}

// Delete удаляет пост по ID
func (s *PostStore) Delete(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.posts[id]
	if !exists {
		return false
	}
	delete(s.posts, id)
	return true
}

// IncrementViewCount увеличивает счётчик просмотров
func (s *PostStore) IncrementViewCount(id int64) (Post, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	post, exists := s.posts[id]
	if !exists {
		return Post{}, false
	}
	post.ViewCount++
	s.posts[id] = post
	return post, true
}

// InitSamplePosts заполняет хранилище тестовыми данными
func (s *PostStore) InitSamplePosts() {
	now := time.Now()
	defaultAuthor := &Author{
		ID:        1,
		Username:  "john_blogger",
		Email:     "john@example.com",
		Bio:       "Разработчик и технический писатель",
		AvatarURL: "https://example.com/avatars/john.jpg",
	}

	samples := []Post{
		{
			Title:       "Введение в Go API",
			Slug:        "introduction-to-go-api",
			Content:     "Go (Golang) — это мощный язык программирования для создания высокопроизводительных API...",
			Excerpt:     "Изучаем основы создания API на Go",
			Author:      defaultAuthor,
			Tags:        []Tag{{ID: 1, Name: "Go", Slug: "go"}, {ID: 2, Name: "Backend", Slug: "backend"}, {ID: 3, Name: "API", Slug: "api"}},
			ViewCount:   150,
			PublishedAt: now.Add(-24 * time.Hour),
			CreatedAt:   now.Add(-48 * time.Hour),
		},
		{
			Title:       "Работа с JSON в Go",
			Slug:        "working-with-json-in-go",
			Content:     "Обработка JSON — одна из самых частых задач при разработке веб-сервисов...",
			Excerpt:     "Полное руководство по encoding/json",
			Author:      defaultAuthor,
			Tags:        []Tag{{ID: 1, Name: "Go", Slug: "go"}, {ID: 4, Name: "JSON", Slug: "json"}},
			ViewCount:   89,
			PublishedAt: now.Add(-12 * time.Hour),
			CreatedAt:   now.Add(-24 * time.Hour),
		},
		{
			Title:       "Микросервисы: лучшие практики",
			Slug:        "microservices-best-practices",
			Content:     "При проектировании микросервисной архитектуры важно следовать определённым принципам...",
			Excerpt:     "Советы по построению микросервисов",
			Author:      defaultAuthor,
			Tags:        []Tag{{ID: 2, Name: "Backend", Slug: "backend"}, {ID: 5, Name: "Architecture", Slug: "architecture"}},
			ViewCount:   234,
			PublishedAt: now.Add(-6 * time.Hour),
			CreatedAt:   now.Add(-12 * time.Hour),
		},
	}

	for _, post := range samples {
		s.Create(post)
	}
}

// --- Утилиты ---

func makeTagsFromNames(names []string) []Tag {
	tags := make([]Tag, 0, len(names))
	for i, name := range names {
		tags = append(tags, Tag{
			ID:   int64(i + 1),
			Name: name,
			Slug: strings.ToLower(strings.ReplaceAll(name, " ", "-")),
		})
	}
	return tags
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeErrorResponse(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{
		Error:     http.StatusText(status),
		Message:   message,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// --- RequestCounter — потокобезопасный счётчик запросов ---

// RequestCounter хранит счётчик запросов и время запуска
type RequestCounter struct {
	mu        sync.RWMutex
	count     int64
	startTime time.Time
}

// NewRequestCounter создаёт новый счётчик
func NewRequestCounter() *RequestCounter {
	return &RequestCounter{
		startTime: time.Now(),
	}
}

// Increment увеличивает счётчик
func (rc *RequestCounter) Increment() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.count++
}

// Get возвращает текущую статистику
func (rc *RequestCounter) Get() Stats {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return Stats{
		RequestCount: rc.count,
		Uptime:       time.Since(rc.startTime).String(),
		StartTime:    rc.startTime.Format(time.RFC3339),
	}
}

// --- App — приложение с инъекцией зависимостей ---

// App содержит все зависимости приложения
type App struct {
	store   *PostStore
	counter *RequestCounter
	server  *http.Server
	running atomic.Bool
}

// NewApp создаёт новое приложение
func NewApp() *App {
	app := &App{
		store:   NewPostStore(),
		counter: NewRequestCounter(),
	}
	app.store.InitSamplePosts()
	return app
}

// GetStore возвращает хранилище (для тестов)
func (a *App) GetStore() *PostStore {
	return a.store
}

// GetCounter возвращает счётчик (для тестов)
func (a *App) GetCounter() *RequestCounter {
	return a.counter
}

// GetServer возвращает HTTP-сервер (для тестов)
func (a *App) GetServer() *http.Server {
	return a.server
}

// IsRunning проверяет, работает ли сервер
func (a *App) IsRunning() bool {
	return a.running.Load()
}

// SetupHTTP регистрирует все обработчики
func (a *App) SetupHTTP() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", a.healthHandler)
	mux.HandleFunc("/api/stats", a.statsHandler)
	mux.HandleFunc("/api/echo", a.echoHandler)
	mux.HandleFunc("/api/posts", a.postsHandler)
	mux.HandleFunc("/api/posts/", a.postByIDHandler)
	return mux
}

// Start запускает HTTP-сервер
func (a *App) Start(port string) error {
	mux := a.SetupHTTP()

	a.server = &http.Server{
		Addr:         port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		fmt.Printf("Go API сервер запущен на порту %s\n", port)
		a.running.Store(true)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка сервера: %v", err)
		}
	}()

	sig := <-quit
	fmt.Printf("\nПолучен сигнал %v. Завершение работы...\n", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		log.Fatalf("Ошибка при завершении работы: %v", err)
	}

	a.running.Store(false)
	fmt.Println("Сервер успешно завершил работу")
	return nil
}

// --- Handlers ---

func (a *App) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	a.counter.Increment()

	writeJSON(w, http.StatusOK, HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().Format(time.RFC3339),
		Version:   "1.0.0",
	})
}

func (a *App) statsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	a.counter.Increment()
	writeJSON(w, http.StatusOK, a.counter.Get())
}

func (a *App) echoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	a.counter.Increment()

	var req EchoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	writeJSON(w, http.StatusOK, EchoResponse{
		Original:  req,
		Processed: fmt.Sprintf("Received: %s (processed by Go API)", req.Message),
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

func (a *App) postsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.getPostsHandler(w, r)
	case http.MethodPost:
		a.createPostHandler(w, r)
	default:
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (a *App) getPostsHandler(w http.ResponseWriter, r *http.Request) {
	a.counter.Increment()

	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	perPage, _ := strconv.Atoi(query.Get("per_page"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	paginatedPosts, total := a.store.GetAll(page, perPage)

	writeJSON(w, http.StatusOK, GetPostsResponse{
		Posts:     paginatedPosts,
		Total:     total,
		Page:      page,
		PerPage:   perPage,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

func (a *App) createPostHandler(w http.ResponseWriter, r *http.Request) {
	a.counter.Increment()

	var req CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if req.Title == "" || req.Content == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Title and content are required")
		return
	}

	now := time.Now()
	defaultAuthor := &Author{
		ID:        1,
		Username:  "john_blogger",
		Email:     "john@example.com",
		Bio:       "Разработчик и технический писатель",
		AvatarURL: "https://example.com/avatars/john.jpg",
	}

	post := Post{
		Title:     req.Title,
		Slug:      strings.ToLower(strings.ReplaceAll(req.Title, " ", "-")),
		Content:   req.Content,
		Excerpt:   req.Excerpt,
		Author:    defaultAuthor,
		Tags:      makeTagsFromNames(req.TagNames),
		Comments:  []Comment{},
		CreatedAt: now,
	}

	id := a.store.Create(post)
	post.ID = id

	writeJSON(w, http.StatusCreated, CreatePostResponse{
		Post:      post,
		Message:   "Post created successfully",
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

func (a *App) postByIDHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/posts/")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid post ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		a.getPostByIDHandler(w, r, id)
	case http.MethodPut:
		a.updatePostHandler(w, r, id)
	case http.MethodDelete:
		a.deletePostHandler(w, r, id)
	default:
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (a *App) getPostByIDHandler(w http.ResponseWriter, r *http.Request, id int64) {
	a.counter.Increment()

	post, updated := a.store.IncrementViewCount(id)
	if !updated {
		writeErrorResponse(w, http.StatusNotFound, "Post not found")
		return
	}

	writeJSON(w, http.StatusOK, PostResponse{
		Post:      post,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

func (a *App) updatePostHandler(w http.ResponseWriter, r *http.Request, id int64) {
	a.counter.Increment()

	if _, exists := a.store.GetByID(id); !exists {
		writeErrorResponse(w, http.StatusNotFound, "Post not found")
		return
	}

	var req UpdatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	updatedPost, ok := a.store.Update(id, req)
	if !ok {
		writeErrorResponse(w, http.StatusNotFound, "Post not found")
		return
	}

	writeJSON(w, http.StatusOK, PostResponse{
		Post:      updatedPost,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

func (a *App) deletePostHandler(w http.ResponseWriter, r *http.Request, id int64) {
	a.counter.Increment()

	if !a.store.Delete(id) {
		writeErrorResponse(w, http.StatusNotFound, "Post not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

func main() {
	app := NewApp()
	app.Start(":8080")
}
