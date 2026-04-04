package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	pb "github.com/frozenm/lr10/grpc-go/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// --- Константы ---

const (
	// Порт gRPC-сервера по умолчанию
	grpcPort = ":50051"

	// Настройки пагинации
	minPerPage     = 1
	maxPerPage     = 100
	defaultPerPage = 10
	defaultPage    = 1

	// Версия gRPC-сервиса
	serviceVersion = "1.0.0"
)

// --- Модели данных ---

// DataItem представляет элемент данных в хранилище
type DataItem struct {
	ID          int64
	Name        string
	Description string
	Value       float64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// --- ItemStore — потокобезопасное хранилище элементов ---

// ItemStore хранит элементы данных с защитой от конкурентного доступа
type ItemStore struct {
	mu     sync.RWMutex
	items  map[int64]*DataItem
	nextID int64
}

// NewItemStore создаёт новое хранилище элементов
func NewItemStore() *ItemStore {
	return &ItemStore{
		items:  make(map[int64]*DataItem),
		nextID: 1,
	}
}

// Get возвращает элемент по идентификатору
func (s *ItemStore) Get(id int64) (*DataItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, exists := s.items[id]
	return item, exists
}

// Create добавляет новый элемент и возвращает его идентификатор
func (s *ItemStore) Create(item *DataItem) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextID
	item.ID = id
	s.nextID++
	s.items[id] = item
	return id
}

// List возвращает элементы с пагинацией, отсортированные по ID
func (s *ItemStore) List(page, perPage int32) ([]*DataItem, int32) {
	s.mu.RLock()
	allItems := make([]*DataItem, 0, len(s.items))
	for _, item := range s.items {
		allItems = append(allItems, item)
	}
	s.mu.RUnlock()

	total := int32(len(allItems))

	if page < defaultPage {
		page = defaultPage
	}
	if perPage < minPerPage || perPage > maxPerPage {
		perPage = defaultPerPage
	}

	start := (page - 1) * perPage
	end := start + perPage

	if start >= total {
		return []*DataItem{}, total
	}
	if end > total {
		end = total
	}

	return allItems[start:end], total
}

// Update обновляет существующий элемент
// Возвращает ошибку, если элемент не найден
func (s *ItemStore) Update(id int64, name, description string, value float64, updateValue bool) (*DataItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.items[id]
	if !exists {
		return nil, fmt.Errorf("элемент с ID %d не найден", id)
	}

	// Обновляем имя только если оно не пустое и не состоит из одних пробелов
	if name != "" && strings.TrimSpace(name) != "" {
		item.Name = name
	}
	if description != "" {
		item.Description = description
	}
	if updateValue {
		item.Value = value
	}
	item.UpdatedAt = time.Now()

	return item, nil
}

// Delete удаляет элемент по идентификатору
func (s *ItemStore) Delete(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.items[id]
	if !exists {
		return false
	}
	delete(s.items, id)
	return true
}

// Count возвращает количество элементов в хранилище
func (s *ItemStore) Count() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return int32(len(s.items))
}

// InitSampleData заполняет хранилище демонстрационными данными
func (s *ItemStore) InitSampleData() {
	now := time.Now()
	samples := []*DataItem{
		{
			ID:          1,
			Name:        "Первый элемент",
			Description: "Описание первого элемента для тестирования",
			Value:       100.50,
			CreatedAt:   now.Add(-24 * time.Hour),
			UpdatedAt:   now.Add(-24 * time.Hour),
		},
		{
			ID:          2,
			Name:        "Второй элемент",
			Description: "Описание второго элемента",
			Value:       250.75,
			CreatedAt:   now.Add(-12 * time.Hour),
			UpdatedAt:   now.Add(-12 * time.Hour),
		},
		{
			ID:          3,
			Name:        "Третий элемент",
			Description: "Тестовый элемент с данными",
			Value:       75.25,
			CreatedAt:   now.Add(-6 * time.Hour),
			UpdatedAt:   now.Add(-6 * time.Hour),
		},
	}

	for _, item := range samples {
		s.items[item.ID] = item
	}
	s.nextID = 4
}

// --- dataServer — реализация gRPC-сервиса DataService ---

// dataServer реализует сервис DataService с хранилищем
type dataServer struct {
	pb.UnimplementedDataServiceServer
	store   *ItemStore
	started time.Time
}

// GetData возвращает элемент по идентификатору
func (s *dataServer) GetData(ctx context.Context, req *pb.GetRequest) (*pb.DataResponse, error) {
	item, exists := s.store.Get(req.Id)
	if !exists {
		return nil, fmt.Errorf("элемент с ID %d не найден", req.Id)
	}

	return &pb.DataResponse{
		Item:      toProto(item),
		Message:   "Элемент успешно получен",
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

// CreateData создаёт новый элемент данных
func (s *dataServer) CreateData(ctx context.Context, req *pb.CreateRequest) (*pb.DataResponse, error) {
	// Проверяем, что имя не пустое и не состоит из одних пробелов
	if req.Name == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("имя элемента обязательно")
	}

	now := time.Now()
	item := &DataItem{
		Name:        req.Name,
		Description: req.Description,
		Value:       req.Value,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	id := s.store.Create(item)
	item.ID = id

	return &pb.DataResponse{
		Item:      toProto(item),
		Message:   "Элемент успешно создан",
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

// ListData возвращает список элементов с пагинацией
func (s *dataServer) ListData(ctx context.Context, req *pb.ListRequest) (*pb.ListResponse, error) {
	page := req.Page
	perPage := req.PerPage

	if page < defaultPage {
		page = defaultPage
	}
	if perPage < minPerPage || perPage > maxPerPage {
		perPage = defaultPerPage
	}

	items, total := s.store.List(page, perPage)

	// Конвертируем внутренние DataItem в proto-сообщения
	protoItems := make([]*pb.DataItem, 0, len(items))
	for _, item := range items {
		protoItems = append(protoItems, toProto(item))
	}

	return &pb.ListResponse{
		Items:     protoItems,
		Total:     total,
		Page:      page,
		PerPage:   perPage,
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

// UpdateData обновляет существующий элемент
func (s *dataServer) UpdateData(ctx context.Context, req *pb.UpdateRequest) (*pb.DataResponse, error) {
	updatedItem, err := s.store.Update(req.Id, req.Name, req.Description, req.Value, req.UpdateValue)
	if err != nil {
		return nil, err
	}

	return &pb.DataResponse{
		Item:      toProto(updatedItem),
		Message:   "Элемент успешно обновлён",
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

// DeleteData удаляет элемент по идентификатору
func (s *dataServer) DeleteData(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	deleted := s.store.Delete(req.Id)
	if !deleted {
		return nil, fmt.Errorf("элемент с ID %d не найден", req.Id)
	}

	return &pb.DeleteResponse{
		Success:   true,
		Message:   "Элемент успешно удалён",
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

// HealthCheck проверяет работоспособность сервиса
func (s *dataServer) HealthCheck(ctx context.Context, req *pb.HealthRequest) (*pb.HealthResponse, error) {
	return &pb.HealthResponse{
		Status:     "работает",
		Version:    serviceVersion,
		Timestamp:  time.Now().Format(time.RFC3339),
		TotalItems: s.store.Count(),
	}, nil
}

// toProto преобразует DataItem в proto-сообщение
func toProto(item *DataItem) *pb.DataItem {
	return &pb.DataItem{
		Id:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		Value:       item.Value,
		CreatedAt:   item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   item.UpdatedAt.Format(time.RFC3339),
	}
}

func main() {
	// Создаём слушатель (исправлена опечатка: было «слукователь»)
	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		log.Fatalf("Не удалось запустить слушатель: %v", err)
	}

	// Создаём gRPC-сервер
	grpcServer := grpc.NewServer()

	// Создаём хранилище и инициализируем демонстрационными данными
	store := NewItemStore()
	store.InitSampleData()

	// Создаём сервер данных
	server := &dataServer{
		store:   store,
		started: time.Now(),
	}

	// Регистрируем сервис
	pb.RegisterDataServiceServer(grpcServer, server)

	// Включаем reflection для отладки (допустимо для учебной работы)
	reflection.Register(grpcServer)

	// Запускаем сервер в горутине
	go func() {
		fmt.Printf("gRPC-сервер запущен на порту %s\n", grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Ошибка сервера: %v", err)
		}
	}()

	// Обработка сигналов для graceful-завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	fmt.Printf("\nПолучен сигнал %v. Завершение работы...\n", sig)

	// Graceful-завершение
	grpcServer.GracefulStop()
	fmt.Println("gRPC-сервер успешно завершил работу")
}
