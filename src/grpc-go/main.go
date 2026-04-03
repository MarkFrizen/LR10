package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	pb "github.com/frozenm/lr10/grpc-go/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// DataItem представляет элемент данных в хранилище
type DataItem struct {
	ID          int64
	Name        string
	Description string
	Value       float64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// dataServer реализует сервис DataService
type dataServer struct {
	pb.UnimplementedDataServiceServer
	mu      sync.RWMutex
	items   map[int64]*DataItem
	nextID  int64
	started time.Time
}

// GetData возвращает элемент по ID
func (s *dataServer) GetData(ctx context.Context, req *pb.GetRequest) (*pb.DataResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, exists := s.items[req.Id]
	if !exists {
		return nil, fmt.Errorf("item with ID %d not found", req.Id)
	}

	return &pb.DataResponse{
		Item:      toProto(item),
		Message:   "Item retrieved successfully",
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

// CreateData создаёт новый элемент
func (s *dataServer) CreateData(ctx context.Context, req *pb.CreateRequest) (*pb.DataResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	id := s.nextID
	s.nextID++

	now := time.Now()
	item := &DataItem{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Value:       req.Value,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.items[id] = item

	return &pb.DataResponse{
		Item:      toProto(item),
		Message:   "Item created successfully",
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

// ListData возвращает список элементов с пагинацией
func (s *dataServer) ListData(ctx context.Context, req *pb.ListRequest) (*pb.ListResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	page := req.Page
	perPage := req.PerPage

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	allItems := make([]*pb.DataItem, 0, len(s.items))
	for _, item := range s.items {
		allItems = append(allItems, toProto(item))
	}

	total := int32(len(allItems))
	start := (page - 1) * perPage
	end := start + perPage

	var paginatedItems []*pb.DataItem
	if start < int32(len(allItems)) {
		if end > int32(len(allItems)) {
			end = int32(len(allItems))
		}
		paginatedItems = allItems[start:end]
	} else {
		paginatedItems = []*pb.DataItem{}
	}

	return &pb.ListResponse{
		Items:     paginatedItems,
		Total:     total,
		Page:      page,
		PerPage:   perPage,
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

// UpdateData обновляет элемент
func (s *dataServer) UpdateData(ctx context.Context, req *pb.UpdateRequest) (*pb.DataResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.items[req.Id]
	if !exists {
		return nil, fmt.Errorf("item with ID %d not found", req.Id)
	}

	if req.Name != "" {
		item.Name = req.Name
	}
	if req.Description != "" {
		item.Description = req.Description
	}
	item.Value = req.Value
	item.UpdatedAt = time.Now()

	return &pb.DataResponse{
		Item:      toProto(item),
		Message:   "Item updated successfully",
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

// DeleteData удаляет элемент
func (s *dataServer) DeleteData(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.items[req.Id]
	if !exists {
		return nil, fmt.Errorf("item with ID %d not found", req.Id)
	}

	delete(s.items, req.Id)

	return &pb.DeleteResponse{
		Success:   true,
		Message:   "Item deleted successfully",
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

// HealthCheck проверяет работоспособность сервиса
func (s *dataServer) HealthCheck(ctx context.Context, req *pb.HealthRequest) (*pb.HealthResponse, error) {
	s.mu.RLock()
	totalItems := int32(len(s.items))
	s.mu.RUnlock()

	return &pb.HealthResponse{
		Status:     "healthy",
		Version:    "1.0.0",
		Timestamp:  time.Now().Format(time.RFC3339),
		TotalItems: totalItems,
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

// initSampleData инициализирует тестовые данные
func initSampleData(s *dataServer) {
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

func main() {
	port := ":50051"

	// Создаём слукователь
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Не удалось запустить слукователь: %v", err)
	}

	// Создаём gRPC-сервер
	grpcServer := grpc.NewServer()

	// Создаём сервер данных
	server := &dataServer{
		items:   make(map[int64]*DataItem),
		nextID:  1,
		started: time.Now(),
	}

	// Инициализируем тестовые данные
	initSampleData(server)

	// Регистрируем сервис
	pb.RegisterDataServiceServer(grpcServer, server)

	// Включаем reflection для отладки
	reflection.Register(grpcServer)

	// Запускаем сервер в горутине
	go func() {
		fmt.Printf("gRPC-сервер запущен на порту %s\n", port)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Ошибка сервера: %v", err)
		}
	}()

	// Обработка сигналов для graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	fmt.Printf("\nПолучен сигнал %v. Завершение работы...\n", sig)

	// Graceful shutdown
	grpcServer.GracefulStop()
	fmt.Println("gRPC-сервер успешно завершил работу")
}
