package main

import (
	"context"
	"testing"
	"time"

	pb "github.com/frozenm/lr10/grpc-go/proto"
)

// setupTestServer создаёт тестовый сервер
func setupTestServer() *dataServer {
	return &dataServer{
		items:   make(map[int64]*DataItem),
		nextID:  1,
		started: time.Now(),
	}
}

// TestDataItemToProto тестирует функцию toProto
func TestDataItemToProto(t *testing.T) {
	now := time.Now()
	item := &DataItem{
		ID:          1,
		Name:        "Test",
		Description: "Test Description",
		Value:       100.5,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	proto := toProto(item)

	if proto.Id != item.ID {
		t.Errorf("toProto().Id = %d, want %d", proto.Id, item.ID)
	}
	if proto.Name != item.Name {
		t.Errorf("toProto().Name = %s, want %s", proto.Name, item.Name)
	}
	if proto.Description != item.Description {
		t.Errorf("toProto().Description = %s, want %s", proto.Description, item.Description)
	}
	if proto.Value != item.Value {
		t.Errorf("toProto().Value = %f, want %f", proto.Value, item.Value)
	}
}

// TestHealthCheck тестирует проверку работоспособности
func TestHealthCheck(t *testing.T) {
	server := setupTestServer()

	// Добавляем тестовые данные
	server.items[1] = &DataItem{ID: 1, Name: "Test"}
	server.items[2] = &DataItem{ID: 2, Name: "Test2"}
	server.nextID = 3

	resp, err := server.HealthCheck(context.Background(), &pb.HealthRequest{})
	if err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("HealthCheck().Status = %s, want healthy", resp.Status)
	}
	if resp.Version != "1.0.0" {
		t.Errorf("HealthCheck().Version = %s, want 1.0.0", resp.Version)
	}
	if resp.TotalItems != 2 {
		t.Errorf("HealthCheck().TotalItems = %d, want 2", resp.TotalItems)
	}
	if resp.Timestamp == "" {
		t.Error("HealthCheck().Timestamp should not be empty")
	}
}

// TestCreateData тестирует создание элемента
func TestCreateData(t *testing.T) {
	server := setupTestServer()

	resp, err := server.CreateData(context.Background(), &pb.CreateRequest{
		Name:        "New Item",
		Description: "New item description",
		Value:       50.25,
	})
	if err != nil {
		t.Fatalf("CreateData() error = %v", err)
	}

	if resp.Item.Name != "New Item" {
		t.Errorf("CreateData().Item.Name = %s, want New Item", resp.Item.Name)
	}
	if resp.Item.Description != "New item description" {
		t.Errorf("CreateData().Item.Description = %s, want New item description", resp.Item.Description)
	}
	if resp.Item.Value != 50.25 {
		t.Errorf("CreateData().Item.Value = %f, want 50.25", resp.Item.Value)
	}
	if resp.Item.Id != 1 {
		t.Errorf("CreateData().Item.Id = %d, want 1", resp.Item.Id)
	}
	if resp.Message == "" {
		t.Error("CreateData().Message should not be empty")
	}
	if server.nextID != 2 {
		t.Errorf("nextID = %d, want 2", server.nextID)
	}
}

// TestCreateDataEmptyName тестирует создание элемента с пустым именем
func TestCreateDataEmptyName(t *testing.T) {
	server := setupTestServer()

	_, err := server.CreateData(context.Background(), &pb.CreateRequest{
		Name:        "",
		Description: "Description",
		Value:       10.0,
	})
	if err == nil {
		t.Error("CreateData() should return error for empty name")
	}
}

// TestGetData тестирует получение элемента
func TestGetData(t *testing.T) {
	server := setupTestServer()

	// Создаём элемент
	createResp, _ := server.CreateData(context.Background(), &pb.CreateRequest{
		Name:  "Test Item",
		Value: 100.0,
	})

	// Получаем элемент
	resp, err := server.GetData(context.Background(), &pb.GetRequest{
		Id: createResp.Item.Id,
	})
	if err != nil {
		t.Fatalf("GetData() error = %v", err)
	}

	if resp.Item.Id != createResp.Item.Id {
		t.Errorf("GetData().Item.Id = %d, want %d", resp.Item.Id, createResp.Item.Id)
	}
	if resp.Item.Name != createResp.Item.Name {
		t.Errorf("GetData().Item.Name = %s, want %s", resp.Item.Name, createResp.Item.Name)
	}
}

// TestGetDataNotFound тестирует получение несуществующего элемента
func TestGetDataNotFound(t *testing.T) {
	server := setupTestServer()

	_, err := server.GetData(context.Background(), &pb.GetRequest{
		Id: 999,
	})
	if err == nil {
		t.Error("GetData() should return error for non-existent item")
	}
}

// TestListData тестирует получение списка элементов
func TestListData(t *testing.T) {
	server := setupTestServer()

	// Создаём несколько элементов
	for i := 0; i < 5; i++ {
		_, err := server.CreateData(context.Background(), &pb.CreateRequest{
			Name:  "Item",
			Value: float64(i),
		})
		if err != nil {
			t.Fatalf("CreateData() error = %v", err)
		}
	}

	// Получаем все элементы
	resp, err := server.ListData(context.Background(), &pb.ListRequest{
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("ListData() error = %v", err)
	}

	if resp.Total != 5 {
		t.Errorf("ListData().Total = %d, want 5", resp.Total)
	}
	if len(resp.Items) != 5 {
		t.Errorf("ListData() returned %d items, want 5", len(resp.Items))
	}
	if resp.Page != 1 {
		t.Errorf("ListData().Page = %d, want 1", resp.Page)
	}
}

// TestListDataPagination тестирует пагинацию
func TestListDataPagination(t *testing.T) {
	server := setupTestServer()

	// Создаём 10 элементов
	for i := 0; i < 10; i++ {
		_, err := server.CreateData(context.Background(), &pb.CreateRequest{
			Name:  "Item",
			Value: float64(i),
		})
		if err != nil {
			t.Fatalf("CreateData() error = %v", err)
		}
	}

	// Получаем первую страницу
	resp, err := server.ListData(context.Background(), &pb.ListRequest{
		Page:    1,
		PerPage: 3,
	})
	if err != nil {
		t.Fatalf("ListData() error = %v", err)
	}

	if resp.Total != 10 {
		t.Errorf("ListData().Total = %d, want 10", resp.Total)
	}
	if len(resp.Items) != 3 {
		t.Errorf("ListData() returned %d items, want 3", len(resp.Items))
	}
}

// TestUpdateData тестирует обновление элемента
func TestUpdateData(t *testing.T) {
	server := setupTestServer()

	// Создаём элемент
	createResp, _ := server.CreateData(context.Background(), &pb.CreateRequest{
		Name:        "Original",
		Description: "Original description",
		Value:       100.0,
	})

	// Обновляем элемент
	updateResp, err := server.UpdateData(context.Background(), &pb.UpdateRequest{
		Id:           createResp.Item.Id,
		Name:         "Updated",
		Description:  "Updated description",
		Value:        200.0,
		UpdateValue:  true,
	})
	if err != nil {
		t.Fatalf("UpdateData() error = %v", err)
	}

	if updateResp.Item.Name != "Updated" {
		t.Errorf("UpdateData().Item.Name = %s, want Updated", updateResp.Item.Name)
	}
	if updateResp.Item.Description != "Updated description" {
		t.Errorf("UpdateData().Item.Description = %s, want Updated description", updateResp.Item.Description)
	}
	if updateResp.Item.Value != 200.0 {
		t.Errorf("UpdateData().Item.Value = %f, want 200.0", updateResp.Item.Value)
	}
}

// TestUpdateDataNotFound тестирует обновление несуществующего элемента
func TestUpdateDataNotFound(t *testing.T) {
	server := setupTestServer()

	_, err := server.UpdateData(context.Background(), &pb.UpdateRequest{
		Id:    999,
		Name:  "Updated",
		Value: 100.0,
	})
	if err == nil {
		t.Error("UpdateData() should return error for non-existent item")
	}
}

// TestDeleteData тестирует удаление элемента
func TestDeleteData(t *testing.T) {
	server := setupTestServer()

	// Создаём элемент
	createResp, _ := server.CreateData(context.Background(), &pb.CreateRequest{
		Name:  "To Delete",
		Value: 100.0,
	})

	// Удаляем элемент
	deleteResp, err := server.DeleteData(context.Background(), &pb.DeleteRequest{
		Id: createResp.Item.Id,
	})
	if err != nil {
		t.Fatalf("DeleteData() error = %v", err)
	}

	if !deleteResp.Success {
		t.Error("DeleteData().Success should be true")
	}

	// Проверяем, что элемент действительно удалён
	_, err = server.GetData(context.Background(), &pb.GetRequest{
		Id: createResp.Item.Id,
	})
	if err == nil {
		t.Error("GetData() should return error after deletion")
	}
}

// TestDeleteDataNotFound тестирует удаление несуществующего элемента
func TestDeleteDataNotFound(t *testing.T) {
	server := setupTestServer()

	_, err := server.DeleteData(context.Background(), &pb.DeleteRequest{
		Id: 999,
	})
	if err == nil {
		t.Error("DeleteData() should return error for non-existent item")
	}
}

// TestInitSampleData тестирует инициализацию тестовых данных
func TestInitSampleData(t *testing.T) {
	server := setupTestServer()
	initSampleData(server)

	if len(server.items) != 3 {
		t.Errorf("initSampleData() created %d items, want 3", len(server.items))
	}
	if server.nextID != 4 {
		t.Errorf("nextID = %d, want 4", server.nextID)
	}

	// Проверяем, что все элементы имеют корректные данные
	for id, item := range server.items {
		if item.ID != id {
			t.Errorf("Item ID mismatch: %d != %d", item.ID, id)
		}
		if item.Name == "" {
			t.Errorf("Item %d has empty name", id)
		}
	}
}
