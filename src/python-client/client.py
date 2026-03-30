#!/usr/bin/env python3
"""
Python-клиент для подключения к Go API.
Демонстрирует работу с 3 эндпоинтами: /health, /api/stats, /api/echo
"""

import requests
import json
from typing import Any

BASE_URL = "http://localhost:8080"


def check_health() -> dict[str, Any]:
    """Проверка статуса сервера через /health эндпоинт."""
    response = requests.get(f"{BASE_URL}/health", timeout=5)
    response.raise_for_status()
    return response.json()


def get_stats() -> dict[str, Any]:
    """Получение статистики запросов через /api/stats эндпоинт."""
    response = requests.get(f"{BASE_URL}/api/stats", timeout=5)
    response.raise_for_status()
    return response.json()


def send_echo_message(message: str, data: dict[str, Any] | None = None) -> dict[str, Any]:
    """Отправка сообщения через /api/echo эндпоинт."""
    payload = {"message": message}
    if data:
        payload["data"] = data
    
    response = requests.post(
        f"{BASE_URL}/api/echo",
        json=payload,
        headers={"Content-Type": "application/json"},
        timeout=5
    )
    response.raise_for_status()
    return response.json()


def main():
    """Основная функция демонстрации работы с API."""
    print("=" * 50)
    print("Python-клиент: подключение к Go API")
    print("=" * 50)
    
    # 1. Проверка health эндпоинта
    print("\n[1] Проверка статуса сервера (/health)...")
    try:
        health = check_health()
        print(f"    Статус: {health['status']}")
        print(f"    Версия: {health['version']}")
        print(f"    Время: {health['timestamp']}")
    except requests.exceptions.ConnectionError:
        print("    ОШИБКА: Не удалось подключиться к серверу!")
        print("    Убедитесь, что Go API запущен: go run main.go")
        return
    except Exception as e:
        print(f"    ОШИБКА: {e}")
        return
    
    # 2. Получение статистики
    print("\n[2] Получение статистики (/api/stats)...")
    try:
        stats = get_stats()
        print(f"    Количество запросов: {stats['request_count']}")
        print(f"    Время работы: {stats['uptime']}")
        print(f"    Время запуска: {stats['start_time']}")
    except Exception as e:
        print(f"    ОШИБКА: {e}")
    
    # 3. Отправка сообщения через echo
    print("\n[3] Отправка сообщения через echo (/api/echo)...")
    try:
        echo_data = {"user": "python_client", "action": "test"}
        result = send_echo_message("Hello from Python!", echo_data)
        print(f"    Оригинальное сообщение: {result['original']['message']}")
        print(f"    Обработано: {result['processed']}")
        print(f"    Время ответа: {result['timestamp']}")
        
        # Дополнительные данные
        if result['original'].get('data'):
            print(f"    Данные: {json.dumps(result['original']['data'], ensure_ascii=False)}")
    except Exception as e:
        print(f"    ОШИБКА: {e}")
    
    # 4. Повторная проверка статистики (счётчик должен увеличиться)
    print("\n[4] Повторная проверка статистики...")
    try:
        stats = get_stats()
        print(f"    Количество запросов: {stats['request_count']}")
        print("    (счётчик увеличился на 2 после /api/stats и /api/echo)")
    except Exception as e:
        print(f"    ОШИБКА: {e}")
    
    print("\n" + "=" * 50)
    print("Работа с API завершена успешно!")
    print("=" * 50)


if __name__ == "__main__":
    main()
