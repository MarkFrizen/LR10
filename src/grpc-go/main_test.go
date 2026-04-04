package main

import (
	"context"
	"testing"
	"time"

	pb "github.com/frozenm/lr10/grpc-go/proto"
)

// setupTestServer создаёт тестовое хранилище и сервер
func setupTestServer() (*dataServer, *ItemStore) {
	store := NewItemStore()
	server := &dataServer{
		store:   store,
		started: time.Now(),
	}
	return server, store
}

// --- Тесты преобразования в proto ---

func TestDataItemToProto(t *testing.T) {
	now := time.Now()
	item := &DataItem{
		ID:          1,
		Name:        "Тест",
		Description: "Тестовое описание",
		Value:       100.5,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	proto := toProto(item)

	if proto.Id != item.ID {
		t.Errorf("toProto().Id = %d, ожидалось %d", proto.Id, item.ID)
	}
	if proto.Name != item.Name {
		t.Errorf("toProto().Name = %s, ожидалось %s", proto.Name, item.Name)
	}
	if proto.Description != item.Description {
		t.Errorf("toProto().Description = %s, ожидалось %s", proto.Description, item.Description)
	}
	if proto.Value != item.Value {
		t.Errorf("toProto().Value = %f, ожидалось %f", proto.Value, item.Value)
	}
}

// --- Тесты проверки здоровья ---

func TestHealthCheck(t *testing.T) {
	server, store := setupTestServer()

	// Добавляем тестовые данные
	store.items[1] = &DataItem{ID: 1, Name: "Тест"}
	store.items[2] = &DataItem{ID: 2, Name: "Тест2"}
	store.nextID = 3

	resp, err := server.HealthCheck(context.Background(), &pb.HealthRequest{})
	if err != nil {
		t.Fatalf("HealthCheck() ошибка = %v", err)
	}

	if resp.Status != "работает" {
		t.Errorf("HealthCheck().Status = %s, ожидалось 'работает'", resp.Status)
	}
	if resp.Version != serviceVersion {
		t.Errorf("HealthCheck().Version = %s, ожидалось %s", resp.Version, serviceVersion)
	}
	if resp.TotalItems != 2 {
		t.Errorf("HealthCheck().TotalItems = %d, ожидалось 2", resp.TotalItems)
	}
	if resp.Timestamp == "" {
		t.Error("HealthCheck().Timestamp не должен быть пустым")
	}
}

// --- Тесты создания элемента ---

func TestCreateData(t *testing.T) {
	server, _ := setupTestServer()

	resp, err := server.CreateData(context.Background(), &pb.CreateRequest{
		Name:        "Новый элемент",
		Description: "Описание нового элемента",
		Value:       50.25,
	})
	if err != nil {
		t.Fatalf("CreateData() ошибка = %v", err)
	}

	if resp.Item.Name != "Новый элемент" {
		t.Errorf("CreateData().Item.Name = %s, ожидалось 'Новый элемент'", resp.Item.Name)
	}
	if resp.Item.Description != "Описание нового элемента" {
		t.Errorf("CreateData().Item.Description = %s, ожидалось 'Описание нового элемента'", resp.Item.Description)
	}
	if resp.Item.Value != 50.25 {
		t.Errorf("CreateData().Item.Value = %f, ожидалось 50.25", resp.Item.Value)
	}
	if resp.Item.Id != 1 {
		t.Errorf("CreateData().Item.Id = %d, ожидалось 1", resp.Item.Id)
	}
	if resp.Message == "" {
		t.Error("CreateData().Message не должен быть пустым")
	}
}

func TestCreateDataEmptyName(t *testing.T) {
	server, _ := setupTestServer()

	_, err := server.CreateData(context.Background(), &pb.CreateRequest{
		Name:        "",
		Description: "Описание",
		Value:       10.0,
	})
	if err == nil {
		t.Error("CreateData() должна вернуть ошибку при пустом имени")
	}
}

func TestCreateDataWhitespaceOnlyName(t *testing.T) {
	server, _ := setupTestServer()

	_, err := server.CreateData(context.Background(), &pb.CreateRequest{
		Name:        "   ",
		Description: "Описание",
		Value:       10.0,
	})
	if err == nil {
		t.Error("CreateData() должна вернуть ошибку при имени из одних пробелов")
	}
}

// --- Тесты получения элемента ---

func TestGetData(t *testing.T) {
	server, _ := setupTestServer()

	// Создаём элемент
	createResp, _ := server.CreateData(context.Background(), &pb.CreateRequest{
		Name:  "Тестовый элемент",
		Value: 100.0,
	})

	// Получаем элемент
	resp, err := server.GetData(context.Background(), &pb.GetRequest{
		Id: createResp.Item.Id,
	})
	if err != nil {
		t.Fatalf("GetData() ошибка = %v", err)
	}

	if resp.Item.Id != createResp.Item.Id {
		t.Errorf("GetData().Item.Id = %d, ожидалось %d", resp.Item.Id, createResp.Item.Id)
	}
	if resp.Item.Name != createResp.Item.Name {
		t.Errorf("GetData().Item.Name = %s, ожидалось %s", resp.Item.Name, createResp.Item.Name)
	}
}

func TestGetDataNotFound(t *testing.T) {
	server, _ := setupTestServer()

	_, err := server.GetData(context.Background(), &pb.GetRequest{
		Id: 999,
	})
	if err == nil {
		t.Error("GetData() должна вернуть ошибку для несуществующего элемента")
	}
}

// --- Тесты списка элементов ---

func TestListData(t *testing.T) {
	server, _ := setupTestServer()

	// Создаём несколько элементов
	for i := 0; i < 5; i++ {
		_, err := server.CreateData(context.Background(), &pb.CreateRequest{
			Name:  "Элемент",
			Value: float64(i),
		})
		if err != nil {
			t.Fatalf("CreateData() ошибка = %v", err)
		}
	}

	// Получаем все элементы
	resp, err := server.ListData(context.Background(), &pb.ListRequest{
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("ListData() ошибка = %v", err)
	}

	if resp.Total != 5 {
		t.Errorf("ListData().Total = %d, ожидалось 5", resp.Total)
	}
	if len(resp.Items) != 5 {
		t.Errorf("ListData() вернуло %d элементов, ожидалось 5", len(resp.Items))
	}
	if resp.Page != 1 {
		t.Errorf("ListData().Page = %d, ожидалось 1", resp.Page)
	}
}

func TestListDataPagination(t *testing.T) {
	server, _ := setupTestServer()

	// Создаём 10 элементов
	for i := 0; i < 10; i++ {
		_, err := server.CreateData(context.Background(), &pb.CreateRequest{
			Name:  "Элемент",
			Value: float64(i),
		})
		if err != nil {
			t.Fatalf("CreateData() ошибка = %v", err)
		}
	}

	// Получаем первую страницу
	resp, err := server.ListData(context.Background(), &pb.ListRequest{
		Page:    1,
		PerPage: 3,
	})
	if err != nil {
		t.Fatalf("ListData() ошибка = %v", err)
	}

	if resp.Total != 10 {
		t.Errorf("ListData().Total = %d, ожидалось 10", resp.Total)
	}
	if len(resp.Items) != 3 {
		t.Errorf("ListData() вернуло %d элементов, ожидалось 3", len(resp.Items))
	}
}

// --- Тесты обновления элемента ---

func TestUpdateData(t *testing.T) {
	server, _ := setupTestServer()

	// Создаём элемент
	createResp, _ := server.CreateData(context.Background(), &pb.CreateRequest{
		Name:        "Оригинал",
		Description: "Оригинальное описание",
		Value:       100.0,
	})

	// Обновляем элемент
	updateResp, err := server.UpdateData(context.Background(), &pb.UpdateRequest{
		Id:          createResp.Item.Id,
		Name:        "Обновлённый",
		Description: "Обновлённое описание",
		Value:       200.0,
		UpdateValue: true,
	})
	if err != nil {
		t.Fatalf("UpdateData() ошибка = %v", err)
	}

	if updateResp.Item.Name != "Обновлённый" {
		t.Errorf("UpdateData().Item.Name = %s, ожидалось 'Обновлённый'", updateResp.Item.Name)
	}
	if updateResp.Item.Description != "Обновлённое описание" {
		t.Errorf("UpdateData().Item.Description = %s, ожидалось 'Обновлённое описание'", updateResp.Item.Description)
	}
	if updateResp.Item.Value != 200.0 {
		t.Errorf("UpdateData().Item.Value = %f, ожидалось 200.0", updateResp.Item.Value)
	}
}

func TestUpdateDataPartial(t *testing.T) {
	server, store := setupTestServer()

	// Создаём элемент
	id := store.Create(&DataItem{
		Name:        "Оригинал",
		Description: "Оригинальное описание",
		Value:       100.0,
	})

	// Обновляем только имя
	updateResp, err := server.UpdateData(context.Background(), &pb.UpdateRequest{
		Id:          id,
		Name:        "Только имя",
		UpdateValue: false,
	})
	if err != nil {
		t.Fatalf("UpdateData() ошибка = %v", err)
	}

	if updateResp.Item.Name != "Только имя" {
		t.Errorf("UpdateData().Item.Name = %s, ожидалось 'Только имя'", updateResp.Item.Name)
	}
	// Описание и значение не должны измениться
	if updateResp.Item.Description != "Оригинальное описание" {
		t.Errorf("UpdateData().Item.Description = %s, ожидалось 'Оригинальное описание'", updateResp.Item.Description)
	}
	if updateResp.Item.Value != 100.0 {
		t.Errorf("UpdateData().Item.Value = %f, ожидалось 100.0", updateResp.Item.Value)
	}
}

func TestUpdateDataEmptyNameDoesNotOverwrite(t *testing.T) {
	server, store := setupTestServer()

	// Создаём элемент
	id := store.Create(&DataItem{
		Name:  "Оригинал",
		Value: 100.0,
	})

	// Пытаемся обновить с пустым именем — имя не должно измениться
	_, err := server.UpdateData(context.Background(), &pb.UpdateRequest{
		Id:          id,
		Name:        "",
		UpdateValue: false,
	})
	if err != nil {
		t.Fatalf("UpdateData() ошибка = %v", err)
	}

	item, _ := store.Get(id)
	if item.Name != "Оригинал" {
		t.Errorf("Имя изменилось на '%s', ожидалось 'Оригинал'", item.Name)
	}
}

func TestUpdateDataWhitespaceNameDoesNotOverwrite(t *testing.T) {
	server, store := setupTestServer()

	// Создаём элемент
	id := store.Create(&DataItem{
		Name:  "Оригинал",
		Value: 100.0,
	})

	// Пытаемся обновить с именем из одних пробелов — имя не должно измениться
	_, err := server.UpdateData(context.Background(), &pb.UpdateRequest{
		Id:          id,
		Name:        "   ",
		UpdateValue: false,
	})
	if err != nil {
		t.Fatalf("UpdateData() ошибка = %v", err)
	}

	item, _ := store.Get(id)
	if item.Name != "Оригинал" {
		t.Errorf("Имя изменилось на '%s', ожидалось 'Оригинал'", item.Name)
	}
}

func TestUpdateDataNotFound(t *testing.T) {
	server, _ := setupTestServer()

	_, err := server.UpdateData(context.Background(), &pb.UpdateRequest{
		Id:    999,
		Name:  "Обновлённый",
		Value: 100.0,
	})
	if err == nil {
		t.Error("UpdateData() должна вернуть ошибку для несуществующего элемента")
	}
}

// --- Тесты удаления элемента ---

func TestDeleteData(t *testing.T) {
	server, store := setupTestServer()

	// Создаём элемент
	id := store.Create(&DataItem{
		Name:  "На удаление",
		Value: 100.0,
	})

	// Удаляем элемент
	deleteResp, err := server.DeleteData(context.Background(), &pb.DeleteRequest{
		Id: id,
	})
	if err != nil {
		t.Fatalf("DeleteData() ошибка = %v", err)
	}

	if !deleteResp.Success {
		t.Error("DeleteData().Success должно быть true")
	}

	// Проверяем, что элемент действительно удалён
	_, err = server.GetData(context.Background(), &pb.GetRequest{
		Id: id,
	})
	if err == nil {
		t.Error("GetData() должна вернуть ошибку после удаления")
	}
}

func TestDeleteDataNotFound(t *testing.T) {
	server, _ := setupTestServer()

	_, err := server.DeleteData(context.Background(), &pb.DeleteRequest{
		Id: 999,
	})
	if err == nil {
		t.Error("DeleteData() должна вернуть ошибку для несуществующего элемента")
	}
}

// --- Тесты инициализации демонстрационных данных ---

func TestInitSampleData(t *testing.T) {
	store := NewItemStore()
	store.InitSampleData()

	if store.Count() != 3 {
		t.Errorf("InitSampleData() создало %d элементов, ожидалось 3", store.Count())
	}
	if store.nextID != 4 {
		t.Errorf("nextID = %d, ожидалось 4", store.nextID)
	}

	// Проверяем, что все элементы имеют корректные данные
	for id, item := range store.items {
		if item.ID != id {
			t.Errorf("Несоответствие ID элемента: %d != %d", item.ID, id)
		}
		if item.Name == "" {
			t.Errorf("Элемент %d имеет пустое имя", id)
		}
	}
}
