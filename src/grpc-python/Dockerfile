FROM python:3.11-slim

WORKDIR /app

# Устанавливаем зависимости
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Копируем proto-файл и генерируем код
COPY proto/*.proto /app/proto/
RUN python -m grpc_tools.protoc \
    -I/app \
    --python_out=. \
    --grpc_python_out=. \
    proto/data.proto

# Копируем клиент
COPY client.py .

# Делаем файл исполняемым
RUN chmod +x client.py

# Запускаем клиент
CMD ["python", "client.py"]
