#!/usr/bin/env python3
"""
Python-клиент для подключения к Go API.
Демонстрирует работу с эндпоинтами блога: /api/posts, /api/posts/{id}
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


# === Блог: Работа с постами ===

def get_posts(page: int = 1, per_page: int = 10) -> dict[str, Any]:
    """Получение списка постов с пагинацией."""
    response = requests.get(
        f"{BASE_URL}/api/posts",
        params={"page": page, "per_page": per_page},
        timeout=5
    )
    response.raise_for_status()
    return response.json()


def get_post(post_id: int) -> dict[str, Any]:
    """Получение поста по ID."""
    response = requests.get(f"{BASE_URL}/api/posts/{post_id}", timeout=5)
    response.raise_for_status()
    return response.json()


def create_post(title: str, content: str, excerpt: str = "", tag_names: list[str] | None = None) -> dict[str, Any]:
    """Создание нового поста."""
    payload = {
        "title": title,
        "content": content,
        "excerpt": excerpt,
        "tag_names": tag_names or []
    }
    response = requests.post(
        f"{BASE_URL}/api/posts",
        json=payload,
        headers={"Content-Type": "application/json"},
        timeout=5
    )
    response.raise_for_status()
    return response.json()


def update_post(post_id: int, title: str = "", content: str = "", excerpt: str = "", tag_names: list[str] | None = None) -> dict[str, Any]:
    """Обновление поста."""
    payload = {}
    if title:
        payload["title"] = title
    if content:
        payload["content"] = content
    if excerpt:
        payload["excerpt"] = excerpt
    if tag_names is not None:
        payload["tag_names"] = tag_names

    response = requests.put(
        f"{BASE_URL}/api/posts/{post_id}",
        json=payload,
        headers={"Content-Type": "application/json"},
        timeout=5
    )
    response.raise_for_status()
    return response.json()


def delete_post(post_id: int) -> bool:
    """Удаление поста. Возвращает True при успехе."""
    response = requests.delete(f"{BASE_URL}/api/posts/{post_id}", timeout=5)
    response.raise_for_status()
    return response.status_code == 204


def main():
    """Основная функция демонстрации работы с API."""
    print("=" * 50)
    print("Python-клиент: подключение к Go API (Блог)")
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

    # 3. Получение списка постов
    print("\n[3] Получение списка постов (/api/posts)...")
    try:
        posts_response = get_posts(page=1, per_page=5)
        print(f"    Всего постов: {posts_response['total']}")
        print(f"    Страница: {posts_response['page']}")
        print(f"    Найдено постов на странице: {len(posts_response['posts'])}")
        
        for post in posts_response['posts'][:3]:
            print(f"\n    📝 Пост #{post['id']}: {post['title']}")
            print(f"       Slug: {post['slug']}")
            print(f"       Автор: {post['author']['username']}")
            print(f"       Просмотры: {post['view_count']}")
            if post.get('tags'):
                tags = ", ".join(tag['name'] for tag in post['tags'])
                print(f"       Теги: {tags}")
    except Exception as e:
        print(f"    ОШИБКА: {e}")

    # 4. Получение конкретного поста (счётчик просмотров увеличится)
    print("\n[4] Получение поста #1 (/api/posts/1)...")
    try:
        post = get_post(1)
        print(f"    Заголовок: {post['post']['title']}")
        print(f"    Содержание (первые 100 симв.): {post['post']['content'][:100]}...")
        print(f"    Просмотров: {post['post']['view_count']}")
        print(f"    Создан: {post['post']['created_at']}")
    except Exception as e:
        print(f"    ОШИБКА: {e}")

    # 5. Создание нового поста
    print("\n[5] Создание нового поста (/api/posts POST)...")
    try:
        new_post_data = create_post(
            title="Python для начинающих: полное руководство",
            content="Python — это интерпретируемый язык программирования общего назначения...",
            excerpt="Изучаем Python с нуля: синтаксис, структуры данных, ООП",
            tag_names=["Python", "Beginner", "Tutorial"]
        )
        new_post = new_post_data['post']
        print(f"    ✅ Пост создан с ID: {new_post['id']}")
        print(f"    Заголовок: {new_post['title']}")
        print(f"    Slug: {new_post['slug']}")
        print(f"    Теги: {', '.join(tag['name'] for tag in new_post['tags'])}")
        print(f"    Сообщение: {new_post_data['message']}")
        
        # Сохраняем ID для дальнейших тестов
        test_post_id = new_post['id']
    except Exception as e:
        print(f"    ОШИБКА: {e}")
        test_post_id = None

    # 6. Обновление поста
    if test_post_id:
        print(f"\n[6] Обновление поста #{test_post_id} (/api/posts/{test_post_id} PUT)...")
        try:
            updated = update_post(
                post_id=test_post_id,
                excerpt="Обновлённое описание для тестового поста",
                tag_names=["Python", "Updated", "Hot"]
            )
            print(f"    ✅ Пост обновлён")
            print(f"    Новое описание: {updated['post']['excerpt']}")
            print(f"    Новые теги: {', '.join(tag['name'] for tag in updated['post']['tags'])}")
        except Exception as e:
            print(f"    ОШИБКА: {e}")

    # 7. Удаление поста
    if test_post_id:
        print(f"\n[7] Удаление поста #{test_post_id} (/api/posts/{test_post_id} DELETE)...")
        try:
            result = delete_post(test_post_id)
            if result:
                print(f"    ✅ Пост успешно удалён")
            else:
                print(f"    ⚠️ Пост не был удалён")
        except Exception as e:
            print(f"    ОШИБКА: {e}")

    # 8. Проверка несуществующего поста (обработка ошибок)
    print("\n[8] Проверка обработки ошибок (пост #999)...")
    try:
        get_post(999)
    except requests.exceptions.HTTPError as e:
        print(f"    ✅ Ошибка корректно обработана: {e.response.status_code}")
        error_data = e.response.json()
        print(f"    Сообщение ошибки: {error_data.get('message', 'Unknown error')}")
    except Exception as e:
        print(f"    ОШИБКА: {e}")

    # 9. Финальная статистика
    print("\n[9] Финальная статистика запросов...")
    try:
        stats = get_stats()
        print(f"    Количество запросов: {stats['request_count']}")
        print(f"    Время работы: {stats['uptime']}")
    except Exception as e:
        print(f"    ОШИБКА: {e}")

    print("\n" + "=" * 50)
    print("Работа с Blog API завершена успешно!")
    print("=" * 50)


if __name__ == "__main__":
    main()
