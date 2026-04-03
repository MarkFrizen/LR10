#!/bin/bash
# Скрипт для генерации gRPC кода из proto-файла для Go

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "=== Установка protoc и плагинов для Go ==="

# Проверяем, что Go установлен
if ! command -v go &> /dev/null; then
    export PATH=$PATH:/home/ubuntu/go/go1.26.1/bin
fi

echo "Go версия: $(go version)"

# Устанавливаем protoc-gen-go и protoc-gen-go-grpc
echo "Установка protoc-gen-go..."
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Проверяем, что protoc установлен
if ! command -v protoc &> /dev/null; then
    echo "protoc не найден. Установка..."
    sudo apt-get update
    sudo apt-get install -y protobuf-compiler
fi

echo "protoc версия: $(protoc --version)"

# Генерируем код из proto-файла
echo "Генерация кода из proto/data.proto..."
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/data.proto

echo "=== Генерация завершена успешно ==="
echo "Файлы сгенерированы в proto/:"
ls -la proto/*.pb.go 2>/dev/null || echo "pb.go файлы не найдены"
ls -la proto/*_grpc.pb.go 2>/dev/null || echo "_grpc.pb.go файлы не найдены"
