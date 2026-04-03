FROM golang:1.24-alpine AS builder

WORKDIR /app

# Установка зависимостей
RUN apk add --no-cache git protoc protobuf-dev

# Копируем go.mod и загружаем зависимости
COPY go.mod ./
RUN go mod download

# Устанавливаем плагины для генерации proto
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Добавляем Go bin в PATH
ENV PATH="/go/bin:${PATH}"

# Копируем proto-файл и генерируем код
COPY proto/*.proto /app/proto/
RUN protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/data.proto

# Обновляем go.sum для сгенерированного proto-кода
RUN go mod tidy

# Копируем исходный код
COPY *.go ./

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Финальный образ
FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Копируем бинарник из builder
COPY --from=builder /app/main .

EXPOSE 50051

CMD ["./main"]
