package main

import (
	"context"
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

	"github.com/gin-gonic/gin"
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
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Slug        string    `json:"slug"`
	Content     string    `json:"content"`
	Excerpt     string    `json:"excerpt,omitempty"`
	Author      *Author   `json:"author"`
	Tags        []Tag     `json:"tags,omitempty"`
	Comments    []Comment `json:"comments,omitempty"`
	ViewCount   int64     `json:"view_count"`
	PublishedAt time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
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

// --- Интерфейсы репозиториев (DIP + ISP) ---

// AuthorRepository — контракт хранилища авторов
type AuthorRepository interface {
	GetByID(id int64) (*Author, bool)
}

// PostRepository — контракт хранилища постов
type PostRepository interface {
	GetAll(page, perPage int) ([]Post, int)
	GetByID(id int64) (Post, bool)
	Create(post Post) int64
	Update(id int64, req UpdatePostRequest) (Post, bool)
	Delete(id int64) bool
	IncrementViewCount(id int64) (Post, bool)
}

// --- Справочник авторов ---

// AuthorStore — хранилище авторов (справочник), реализует AuthorRepository
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

// PostStore — потокобезопасное хранилище постов, реализует PostRepository
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
func (s *PostStore) InitSamplePosts(authorRepo AuthorRepository) {
	now := time.Now()
	author, _ := authorRepo.GetByID(1)

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

// App содержит зависимости, маршрутизатор и сервер
type App struct {
	authorRepo AuthorRepository
	postRepo   PostRepository
	counter    *RequestCounter
	engine     *gin.Engine
	server     *http.Server
	running    atomic.Bool
}

// AppOptions — параметры для создания приложения (DI)
type AppOptions struct {
	AuthorRepo AuthorRepository
	PostRepo   PostRepository
	Counter    *RequestCounter
}

// NewApp создаёт приложение с инъекцией зависимостей
func NewApp(opts AppOptions) *App {
	app := &App{
		authorRepo: opts.AuthorRepo,
		postRepo:   opts.PostRepo,
		counter:    opts.Counter,
	}
	return app
}

// IsRunning проверяет, запущен ли сервер
func (a *App) IsRunning() bool {
	return a.running.Load()
}

// SetupHTTP регистрирует все обработчики HTTP-запросов через Gin
func (a *App) SetupHTTP() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	a.engine = engine

	// Middleware для подсчёта запросов
	engine.Use(func(c *gin.Context) {
		a.counter.Increment()
		c.Next()
	})

	// Health check
	engine.GET("/health", a.healthHandler)

	// API routes
	engine.GET("/api/stats", a.statsHandler)
	engine.POST("/api/echo", a.echoHandler)

	// Posts routes
	engine.GET("/api/posts", a.getPostsHandler)
	engine.POST("/api/posts", a.createPostHandler)
	engine.GET("/api/posts/:id", a.getPostByIDHandler)
	engine.PUT("/api/posts/:id", a.updatePostHandler)
	engine.DELETE("/api/posts/:id", a.deletePostHandler)

	return engine
}

// Start запускает HTTP-сервер с graceful-завершением
func (a *App) Start(port string) error {
	a.SetupHTTP()

	a.server = &http.Server{
		Addr:         port,
		Handler:      a.engine,
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

// --- Обработчики HTTP (Gin) ---

// healthHandler — эндпоинт проверки здоровья сервиса
func (a *App) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:    "работает",
		Timestamp: time.Now().Format(time.RFC3339),
		Version:   apiVersion,
	})
}

// statsHandler — эндпоинт статистики запросов
func (a *App) statsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, a.counter.Get())
}

// echoHandler — эндпоинт «эхо» (возвращает полученное сообщение)
func (a *App) echoHandler(c *gin.Context) {
	var req EchoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:     http.StatusText(http.StatusBadRequest),
			Message:   "Некорректный JSON",
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	c.JSON(http.StatusOK, EchoResponse{
		Original:  req,
		Processed: fmt.Sprintf("Получено: %s (обработано REST API)", req.Message),
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// getPostsHandler — получение списка постов с пагинацией
func (a *App) getPostsHandler(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "10"))

	if page < defaultPage {
		page = defaultPage
	}
	if perPage < minPerPage || perPage > maxPerPage {
		perPage = defaultPerPage
	}

	paginatedPosts, total := a.postRepo.GetAll(page, perPage)

	c.JSON(http.StatusOK, GetPostsResponse{
		Posts:     paginatedPosts,
		Total:     total,
		Page:      page,
		PerPage:   perPage,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// createPostHandler — создание нового поста
func (a *App) createPostHandler(c *gin.Context) {
	var req CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:     http.StatusText(http.StatusBadRequest),
			Message:   "Некорректный JSON",
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	if req.Title == "" || req.Content == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:     http.StatusText(http.StatusBadRequest),
			Message:   "Заголовок и содержание обязательны",
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	// Определяем автора: если не указан — используем автора по умолчанию
	authorID := req.AuthorID
	if authorID == 0 {
		authorID = defaultAuthorID
	}

	author, found := a.authorRepo.GetByID(authorID)
	if !found {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:     http.StatusText(http.StatusBadRequest),
			Message:   fmt.Sprintf("Автор с ID %d не найден", authorID),
			Timestamp: time.Now().Format(time.RFC3339),
		})
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

	id := a.postRepo.Create(post)
	post.ID = id

	c.JSON(http.StatusCreated, CreatePostResponse{
		Post:      post,
		Message:   "Пост успешно создан",
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// getPostByIDHandler — получение одного поста по ID
func (a *App) getPostByIDHandler(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:     http.StatusText(http.StatusBadRequest),
			Message:   "Некорректный ID поста",
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	post, found := a.postRepo.IncrementViewCount(id)
	if !found {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:     http.StatusText(http.StatusNotFound),
			Message:   "Пост не найден",
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	c.JSON(http.StatusOK, PostResponse{
		Post:      post,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// updatePostHandler — обновление поста по ID
func (a *App) updatePostHandler(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:     http.StatusText(http.StatusBadRequest),
			Message:   "Некорректный ID поста",
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	if _, exists := a.postRepo.GetByID(id); !exists {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:     http.StatusText(http.StatusNotFound),
			Message:   "Пост не найден",
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	var req UpdatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:     http.StatusText(http.StatusBadRequest),
			Message:   "Некорректный JSON",
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	updatedPost, ok := a.postRepo.Update(id, req)
	if !ok {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:     http.StatusText(http.StatusNotFound),
			Message:   "Пост не найден",
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	c.JSON(http.StatusOK, PostResponse{
		Post:      updatedPost,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// deletePostHandler — удаление поста по ID
func (a *App) deletePostHandler(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:     http.StatusText(http.StatusBadRequest),
			Message:   "Некорректный ID поста",
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	if !a.postRepo.Delete(id) {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:     http.StatusText(http.StatusNotFound),
			Message:   "Пост не найден",
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	c.Status(http.StatusNoContent)
}

func main() {
	// Создание и инъекция зависимостей (Dependency Injection)
	authorRepo := NewAuthorStore()
	postRepo := NewPostStore()
	postRepo.InitSamplePosts(authorRepo)
	counter := NewRequestCounter()

	app := NewApp(AppOptions{
		AuthorRepo: authorRepo,
		PostRepo:   postRepo,
		Counter:    counter,
	})
	app.Start(defaultPort)
}
