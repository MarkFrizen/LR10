#Лабораторная работа №10
#Студент: Фризен Марк Владимирович 
#Группа: 221331 
#Вариант 5
#Задания варианта:
#5. Передавать сложные структуры данных (JSON) между сервисами. 
#7. Реализовать graceful shutdown в обоих сервисах
#1. Создать простое API на Go (Gin) с 2-3 эндпоинтами.
#5.  Развернуть оба сервиса в Docker Compose с общей сетью
#1. Реализовать gRPC-сервер на Go и gRPC-клиент на Python.



## Описание проекта

Проект содержит 4 сервисаa:

1. **REST API на Go** (`src/go-api`) - HTTP REST API для управления постами блога
2. **REST Client на Python** (`src/python-client`) - Python-клиент для Go REST API
3. **gRPC Server на Go** (`src/grpc-go`) - gRPC-сервер как источник данных
4. **gRPC Client на Python** (`src/grpc-python`) - Python gRPC-клиент для Go gRPC-сервера

## Структура проекта

```
src/
├── go-api/                 # REST API на Go (порт 8080)
│   ├── main.go
│   ├── main_test.go
│   ├── Dockerfile
│   └── go.mod
├── python-client/          # REST Client на Python
│   ├── client.py
│   ├── client_test.py
│   ├── requirements.txt
│   └── Dockerfile
├── grpc-go/                # gRPC Server на Go (порт 50051)
│   ├── main.go
│   ├── main_test.go
│   ├── Dockerfile
│   ├── go.mod
│   └── proto/
│       └── data.proto
└── grpc-python/            # gRPC Client на Python
    ├── client.py
    ├── client_test.py
    ├── requirements.txt
    ├── Dockerfile
    └── proto/
        └── data.proto
```

## gRPC Сервис (Go)

### Описание

Go gRPC-сервер является **источником данных** для Python-клиента. Реализует сервис `DataService` с методами:

- **GetData** - получить элемент по ID
- **CreateData** - создать новый элемент
- **ListData** - получить список элементов с пагинацией
- **UpdateData** - обновить элемент
- **DeleteData** - удалить элемент
- **HealthCheck** - проверка работоспособности

### Запуск gRPC-сервера

**Настройка (один раз):**
```bash
cd src/grpc-go
# Установка protoc и плагинов
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
# Генерация кода из proto (требуется protoc)
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/data.proto
# Установка зависимостей
go mod tidy
```

**Локально:**
```bash
cd src/grpc-go
go run main.go
```

**Через Docker:**
```bash
docker-compose up grpc-go
```

### Запуск тестов для Go
```bash
cd src/grpc-go
go test -v
```

## gRPC Клиент (Python)

### Описание

Python gRPC-клиент подключается к Go gRPC-серверу и демонстрирует все операции с данными.

### Запуск gRPC-клиента

**Настройка (один раз):**
```bash
cd src/grpc-python
# Установка зависимостей
pip install -r requirements.txt
# Генерация кода из proto
python3 -m grpc_tools.protoc -I. --python_out=. --grpc_python_out=. proto/data.proto
```

**Локально:**
```bash
cd src/grpc-python
python3 client.py
```

**Через Docker:**
```bash
docker-compose up grpc-python
```

### Запуск тестов для Python
```bash
cd src/grpc-python
python -m unittest client_test.py -v
```

## Docker Compose

Запуск всех сервисов:
```bash
docker-compose up
```

Запуск только gRPC сервисов:
```bash
docker-compose up grpc-go grpc-python
```

Запуск только REST сервисов:
```bash
docker-compose up go-api python-client
```

## Порты

- **8080** - REST API (Go)
- **50051** - gRPC Server (Go)

## Переменные окружения

- `GRPC_SERVER` - адрес gRPC-сервера (по умолчанию `localhost:50051`)
- `BASE_URL` - адрес REST API (по умолчанию `http://localhost:8080`)
