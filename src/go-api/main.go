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

// --- Константы ---

const (
	// Настройки пагинации
	minPerPage     = 1
	maxPerPage     = 100
	defaultPerPage = 10
	defaultPage    = 1

	// Таймауты HTTP-сервера
	httpReadTimeout  = 15 * time.Second
	httpWriteTimeout = 15 * time.Second
	httpIdleTimeout  = 60 * time.Second

	// Таймаут graceful-завершения
	shutdownTimeout = 30 * time.Second

	// Порт по умолчанию
	defaultPort = ":8080"

	// Версия API
	apiVersion = "1.0.0"

	// ID автора по умолчанию
	defaultAuthorID = int64(1)
)

// --- Модели данных ---

// Stats хранит статистику запросов
type Stats struct {
	RequestCount int64  `json:"request_count"`
	Uptime       string `json:"uptime"`
	StartTime    string `json:"start_time"`
}

// EchoRequest — структура запроса для эндпоинта «эхо»
type EchoRequest struct {
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// EchoResponse — структура ответа для эндпоинта «эхо»
type EchoResponse struct {
	Original  EchoRequest `json:"original"`
	Processed string      `json:"processed"`
	Timestamp string      `json:"timestamp"`
}

// HealthResponse — структура ответа для эндпоинта проверки здоровья
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
}

// Author — автор поста в блоге
type Author struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Bio       string `json:"bio,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// Tag — тег поста
type Tag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// Comment — комментарий к посту
type Comment struct {
	ID        int64     `json:"id"`
	PostID    int64     `json:"post_id"`
	AuthorID  int64     `json:"author_id"`
	Author    *Author   `json:"author,omitempty"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// Post — пост в блоге
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

// CreatePostRequest — запрос на создание поста
type CreatePostRequest struct {
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Excerpt  string   `json:"excerpt,omitempty"`
	AuthorID int64    `json:"author_id,omitempty"`
	TagNames []string `json:"tag_names,omitempty"`
}

// UpdatePostRequest — запрос на обновление поста
type UpdatePostRequest struct {
	Title    string   `json:"title,omitempty"`
	Content  string   `json:"content,omitempty"`
	Excerpt  string   `json:"excerpt,omitempty"`
	TagNames []string `json:"tag_names,omitempty"`
}

// CreatePostResponse — ответ при создании поста
type CreatePostResponse struct {
	Post      Post   `json:"post"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// GetPostsResponse — ответ при получении списка постов
type GetPostsResponse struct {
	Posts     []Post `json:"posts"`
	Total     int    `json:"total"`
	Page      int    `json:"page"`
	PerPage   int    `json:"per_page"`
	Timestamp string `json:"timestamp"`
}

// PostResponse — ответ при получении одного поста
type PostResponse struct {
	Post      Post   `json:"post"`
	Timestamp string `json:"timestamp"`
}

// ErrorResponse — структура ошибки
type ErrorResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// --- Справочник авторов ---

// AuthorStore — хранилище авторов (справочник)
type AuthorStore struct {
	authors map[int64]*Author
}

// NewAuthorStore создаёт справочник авторов с предзаполненными данными
func NewAuthorStore() *AuthorStore {
	store := &AuthorStore{
		authors: make(map[int64]*Author),
	}
	// Предзаполненный автор по умолчанию
	store.authors[defaultAuthorID] = &Author{
		ID:        defaultAuthorID,
		Username:  "john_blogger",
		Email:     "john@example.com",
		Bio:       "Разработчик и технический писатель",
		AvatarURL: "https://example.com/avatars/john.jpg",
	}
	return store
}

// GetByID возвращает автора по идентификатору
func (s *AuthorStore) GetByID(id int64) (*Author, bool) {
	author, exists := s.authors[id]
	return author, exists
}

// --- Хранилище постов ---

// PostStore — потокобезопасное хранилище постов
type PostStore struct {
	mu     sync.RWMutex
	posts  map[int64]Post
	nextID int64
}

// NewPostStore создаёт новое пустое хранилище
func NewPostStore() *PostStore {
	return &PostStore{
		posts:  make(map[int64]Post),
		nextID: 1,
	}
}

// GetAll возвращает посты с пагинацией, отсортированные по ID (новые первыми)
func (s *PostStore) GetAll(page, perPage int) ([]Post, int) {
	s.mu.RLock()
	allPosts := make([]Post, 0, len(s.posts))
	for _, post := range s.posts {
		allPosts = append(allPosts, post)
	}
	s.mu.RUnlock()

	// Сортировка по убыванию ID (новые первыми)
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

// GetByID возвращает пост по идентификатору
func (s *PostStore) GetByID(id int64) (Post, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	post, exists := s.posts[id]
	return post, exists
}

// Create добавляет новый пост и возвращает его идентификатор
func (s *PostStore) Create(post Post) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextID
	post.ID = id
	s.nextID++
	s.posts[id] = post
	return id
}

// Update обновляет существующий пост по идентификатору
func (s *PostStore) Update(id int64, req UpdatePostRequest) (Post, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	post, exists := s.posts[id]
	if !exists {
		return Post{}, false
	}
	if req.Title != "" {
		post.Title = req.Title
		post.Slug = makeSlug(req.Title)
	}
	if req.Content != "" {
		post.Content = req.Content
	}
	if req.Excerpt != "" {
		post.Excerpt = req.Excerpt
	}
	if len(req.TagNames) > 0 {
		post.Tags = makeTagsFromNames(req.TagNames)
	}
	post.UpdatedAt = time.Now()
	s.posts[id] = post
	return post, true
}

// Delete удаляет пост по идентификатору
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

// IncrementViewCount увеличивает счётчик просмотров поста
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

// InitSamplePosts заполняет хранилище демонстрационными данными
func (s *PostStore) InitSamplePosts(authorStore *AuthorStore) {
	now := time.Now()
	author, _ := authorStore.GetByID(1)

	samples := []Post{
		{
			Title:       "Введение в Go API",
			Slug:        "introduction-to-go-api",
			Content:     "Go (Golang) — это мощный язык программирования для создания высокопроизводительных API...",
			Excerpt:     "Изучаем основы создания API на Go",
			Author:      author,
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
			Author:      author,
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
			Author:      author,
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

// makeSlug создаёт человекопонятный URL из текста
func makeSlug(text string) string {
	return strings.ToLower(strings.ReplaceAll(text, " ", "-"))
}

// makeTagsFromNames создаёт теги из списка названий
func makeTagsFromNames(names []string) []Tag {
	tags := make([]Tag, 0, len(names))
	for i, name := range names {
		tags = append(tags, Tag{
			ID:   int64(i + 1),
			Name: name,
			Slug: makeSlug(name),
		})
	}
	return tags
}

// writeJSON записывает ответ в формате JSON с указанным кодом состояния
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// writeError записывает ошибку в формате JSON
func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, ErrorResponse{
		Error:     http.StatusText(statusCode),
		Message:   message,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// --- RequestCounter — потокобезопасный счётчик запросов ---

// RequestCounter хранит счётчик запросов и время запуска сервера
type RequestCounter struct {
	mu        sync.RWMutex
	count     int64
	startTime time.Time
}

// NewRequestCounter создаёт новый счётчик запросов
func NewRequestCounter() *RequestCounter {
	return &RequestCounter{
		startTime: time.Now(),
	}
}

// Increment увеличивает счётчик запросов
func (rc *RequestCounter) Increment() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.count++
}

// Get возвращает текущую статистику запросов
func (rc *RequestCounter) Get() Stats {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return Stats{
		RequestCount: rc.count,
		Uptime:       time.Since(rc.startTime).String(),
		StartTime:    rc.startTime.Format(time.RFC3339),
	}
}

// --- App — приложение с внедрением зависимостей ---

// App содержит все зависимости приложения
type App struct {
	authorStore *AuthorStore
	store       *PostStore
	counter     *RequestCounter
	server      *http.Server
	running     atomic.Bool
}

// NewApp создаёт новое приложение с инициализированными зависимостями
func NewApp() *App {
	app := &App{
		authorStore: NewAuthorStore(),
		store:       NewPostStore(),
		counter:     NewRequestCounter(),
	}
	app.store.InitSamplePosts(app.authorStore)
	return app
}

// GetStore возвращает хранилище постов (для тестов)
func (a *App) GetStore() *PostStore {
	return a.store
}

// GetCounter возвращает счётчик запросов (для тестов)
func (a *App) GetCounter() *RequestCounter {
	return a.counter
}

// GetServer возвращает HTTP-сервер (для тестов)
func (a *App) GetServer() *http.Server {
	return a.server
}

// IsRunning проверяет, запущен ли сервер
func (a *App) IsRunning() bool {
	return a.running.Load()
}

// SetupHTTP регистрирует все обработчики HTTP-запросов
func (a *App) SetupHTTP() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", a.healthHandler)
	mux.HandleFunc("/api/stats", a.statsHandler)
	mux.HandleFunc("/api/echo", a.echoHandler)
	mux.HandleFunc("/api/posts", a.postsHandler)
	mux.HandleFunc("/api/posts/", a.postByIDHandler)
	return mux
}

// Start запускает HTTP-сервер с graceful-завершением
func (a *App) Start(port string) error {
	mux := a.SetupHTTP()

	a.server = &http.Server{
		Addr:         port,
		Handler:      mux,
		ReadTimeout:  httpReadTimeout,
		WriteTimeout: httpWriteTimeout,
		IdleTimeout:  httpIdleTimeout,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		fmt.Printf("REST API запущен на порту %s\n", port)
		a.running.Store(true)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка сервера: %v", err)
		}
	}()

	sig := <-quit
	fmt.Printf("\nПолучен сигнал %v. Завершение работы...\n", sig)

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		log.Fatalf("Ошибка при завершении работы: %v", err)
	}

	a.running.Store(false)
	fmt.Println("Сервер успешно завершил работу")
	return nil
}

// --- Обработчики HTTP ---

// healthHandler — эндпоинт проверки здоровья сервиса
func (a *App) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	a.counter.Increment()

	writeJSON(w, http.StatusOK, HealthResponse{
		Status:    "работает",
		Timestamp: time.Now().Format(time.RFC3339),
		Version:   apiVersion,
	})
}

// statsHandler — эндпоинт статистики запросов
func (a *App) statsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	a.counter.Increment()
	writeJSON(w, http.StatusOK, a.counter.Get())
}

// echoHandler — эндпоинт «эхо» (возвращает полученное сообщение)
func (a *App) echoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	a.counter.Increment()

	var req EchoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Некорректный JSON")
		return
	}

	writeJSON(w, http.StatusOK, EchoResponse{
		Original:  req,
		Processed: fmt.Sprintf("Получено: %s (обработано REST API)", req.Message),
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// postsHandler — маршрутизатор эндпоинта /api/posts (список и создание)
func (a *App) postsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.getPostsHandler(w, r)
	case http.MethodPost:
		a.createPostHandler(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
	}
}

// getPostsHandler — получение списка постов с пагинацией
func (a *App) getPostsHandler(w http.ResponseWriter, r *http.Request) {
	a.counter.Increment()

	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	perPage, _ := strconv.Atoi(query.Get("per_page"))

	if page < defaultPage {
		page = defaultPage
	}
	if perPage < minPerPage || perPage > maxPerPage {
		perPage = defaultPerPage
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

// createPostHandler — создание нового поста
func (a *App) createPostHandler(w http.ResponseWriter, r *http.Request) {
	a.counter.Increment()

	var req CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Некорректный JSON")
		return
	}

	if req.Title == "" || req.Content == "" {
		writeError(w, http.StatusBadRequest, "Заголовок и содержание обязательны")
		return
	}

	// Определяем автора: если не указан — используем автора по умолчанию
	authorID := req.AuthorID
	if authorID == 0 {
		authorID = defaultAuthorID
	}

	author, found := a.authorStore.GetByID(authorID)
	if !found {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Автор с ID %d не найден", authorID))
		return
	}

	now := time.Now()

	post := Post{
		Title:     req.Title,
		Slug:      makeSlug(req.Title),
		Content:   req.Content,
		Excerpt:   req.Excerpt,
		Author:    author,
		Tags:      makeTagsFromNames(req.TagNames),
		Comments:  []Comment{},
		CreatedAt: now,
	}

	id := a.store.Create(post)
	post.ID = id

	writeJSON(w, http.StatusCreated, CreatePostResponse{
		Post:      post,
		Message:   "Пост успешно создан",
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// postByIDHandler — маршрутизатор эндпоинта /api/posts/{id}
func (a *App) postByIDHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/posts/")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Некорректный ID поста")
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
		writeError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
	}
}

// getPostByIDHandler — получение одного поста по ID
func (a *App) getPostByIDHandler(w http.ResponseWriter, r *http.Request, id int64) {
	a.counter.Increment()

	post, found := a.store.IncrementViewCount(id)
	if !found {
		writeError(w, http.StatusNotFound, "Пост не найден")
		return
	}

	writeJSON(w, http.StatusOK, PostResponse{
		Post:      post,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// updatePostHandler — обновление поста по ID
func (a *App) updatePostHandler(w http.ResponseWriter, r *http.Request, id int64) {
	a.counter.Increment()

	if _, exists := a.store.GetByID(id); !exists {
		writeError(w, http.StatusNotFound, "Пост не найден")
		return
	}

	var req UpdatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Некорректный JSON")
		return
	}

	updatedPost, ok := a.store.Update(id, req)
	if !ok {
		writeError(w, http.StatusNotFound, "Пост не найден")
		return
	}

	writeJSON(w, http.StatusOK, PostResponse{
		Post:      updatedPost,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// deletePostHandler — удаление поста по ID
func (a *App) deletePostHandler(w http.ResponseWriter, r *http.Request, id int64) {
	a.counter.Increment()

	if !a.store.Delete(id) {
		writeError(w, http.StatusNotFound, "Пост не найден")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

func main() {
	app := NewApp()
	app.Start(defaultPort)
}
