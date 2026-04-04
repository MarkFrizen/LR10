#!/usr/bin/env python3
"""
Python gRPC-клиент для подключения к Go gRPC-серверу.
Демонстрирует работу с сервисом управления данными.
"""

import grpc
import logging
import signal
import os
from contextlib import contextmanager

# Импортируем сгенерированные proto-файлы
import proto.data_pb2 as data_pb2
import proto.data_pb2_grpc as data_pb2_grpc

# Конфигурация
GRPC_SERVER = os.getenv("GRPC_SERVER", "localhost:50051")

# Глобальный флаг для graceful shutdown
_shutdown_requested = False
_channel = None
_stub = None

# Настройка логирования
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class GracefulShutdownError(Exception):
    """Исключение, выбрасываемое при запросе завершения работы."""
    pass


def _signal_handler(signum, frame):
    """Обработчик сигналов для graceful shutdown."""
    global _shutdown_requested
    _shutdown_requested = True
    logger.info(f"Получен сигнал {signum}. Завершение работы...")


def setup_signal_handlers():
    """Устанавливает обработчики сигналов для graceful shutdown."""
    signal.signal(signal.SIGINT, _signal_handler)
    signal.signal(signal.SIGTERM, _signal_handler)


def is_shutdown_requested() -> bool:
    """Проверяет, был ли запрошен shutdown."""
    return _shutdown_requested


def reset_shutdown_flag():
    """Сбрасывает флаг shutdown (для тестов)."""
    global _shutdown_requested
    _shutdown_requested = False


def get_channel() -> grpc.Channel:
    """Возвращает или создаёт канал для подключения."""
    global _channel
    if _channel is None:
        _channel = grpc.insecure_channel(GRPC_SERVER)
    return _channel


def get_stub() -> data_pb2_grpc.DataServiceStub:
    """Возвращает или создаёт stub для подключения."""
    global _stub
    if _stub is None:
        _stub = data_pb2_grpc.DataServiceStub(get_channel())
    return _stub


def close_channel():
    """Закрывает канал и освобождает ресурсы."""
    global _channel, _stub
    if _channel is not None:
        _channel.close()
        _channel = None
        _stub = None

@contextmanager
def grpc_connection():
    """Контекстный менеджер для подключения с автоматическим закрытием."""
    channel = get_channel()
    stub = get_stub()
    try:
        yield stub
    finally:
        close_channel()

def _check_shutdown():
    """Проверяет флаг shutdown и выбрасывает исключение если нужно."""
    if _shutdown_requested:
        raise GracefulShutdownError("Завершение работы запрошено")

# === Функции для работы с DataService ===

def health_check() -> data_pb2.HealthResponse:
    """Проверка работоспособности сервера."""
    _check_shutdown()
    stub = get_stub()
    return stub.HealthCheck(data_pb2.HealthRequest())

def create_data(name: str, description: str = "", value: float = 0.0) -> data_pb2.DataResponse:
    """Создание нового элемента данных."""
    _check_shutdown()
    stub = get_stub()
    request = data_pb2.CreateRequest(
        name=name,
        description=description,
        value=value
    )
    return stub.CreateData(request)

def get_data(item_id: int) -> data_pb2.DataResponse:
    """Получение элемента по ID."""
    _check_shutdown()
    stub = get_stub()
    request = data_pb2.GetRequest(id=item_id)
    return stub.GetData(request)

def list_data(page: int = 1, per_page: int = 10) -> data_pb2.ListResponse:
    """Получение списка элементов с пагинацией."""
    _check_shutdown()
    stub = get_stub()
    request = data_pb2.ListRequest(
        page=page,
        per_page=per_page
    )
    return stub.ListData(request)


def update_data(item_id: int, name: str = "", description: str = "", value: float = 0.0) -> data_pb2.DataResponse:
    """Обновление элемента."""
    _check_shutdown()
    stub = get_stub()
    request = data_pb2.UpdateRequest(
        id=item_id,
        name=name,
        description=description,
        value=value,
        update_value=(value != 0.0)
    )
    return stub.UpdateData(request)


def delete_data(item_id: int) -> data_pb2.DeleteResponse:
    """Удаление элемента."""
    _check_shutdown()
    stub = get_stub()
    request = data_pb2.DeleteRequest(id=item_id)
    return stub.DeleteData(request)


def print_data_item(item: data_pb2.DataItem, prefix: str = "    "):
    """Выводит информацию об элементе данных."""
    print(f"{prefix}ID: {item.id}")
    print(f"{prefix}Имя: {item.name}")
    print(f"{prefix}Описание: {item.description}")
    print(f"{prefix}Значение: {item.value}")
    print(f"{prefix}Создан: {item.created_at}")
    print(f"{prefix}Обновлён: {item.updated_at}")


def main():
    """Основная функция демонстрации работы с gRPC-сервером."""
    # Устанавливаем обработчики сигналов
    setup_signal_handlers()

    try:
        _run_main()
    except grpc.RpcError as e:
        logger.error(f"gRPC ошибка: {e.code()} - {e.details()}")
    except GracefulShutdownError:
        logger.info("Работа прервана пользователем")
    except Exception as e:
        logger.error(f"Неожиданная ошибка: {e}")
    finally:
        # Закрываем канал
        close_channel()


def _run_main():
    """Основная логика демонстрации работы с gRPC-сервером."""
    print("=" * 60)
    print("Python gRPC-клиент: подключение к Go gRPC-серверу")
    print("=" * 60)

    # 1. Проверка health
    print("\n[1] Проверка работоспособности сервера...")
    try:
        health = health_check()
        print(f"    Статус: {health.status}")
        print(f"    Версия: {health.version}")
        print(f"    Элементов на сервере: {health.total_items}")
        print(f"    Время: {health.timestamp}")
    except grpc.RpcError as e:
        logger.error(f"Не удалось подключиться к серверу: {e.code()}")
        print("    Убедитесь, что gRPC-сервер запущен на порту 50051")
        return

    # 2. Получение списка всех элементов
    print("\n[2] Получение списка элементов...")
    try:
        list_response = list_data(page=1, per_page=10)
        print(f"    Всего элементов: {list_response.total}")
        print(f"    Страница: {list_response.page}")
        print(f"    Элементов на странице: {len(list_response.items)}")

        for item in list_response.items[:3]:
            print(f"\n    📦 Элемент:")
            print_data_item(item)
    except grpc.RpcError as e:
        logger.error(f"Ошибка при получении списка: {e.code()}")

    # 3. Получение конкретного элемента
    print("\n[3] Получение элемента #1...")
    try:
        response = get_data(1)
        print(f"    Сообщение: {response.message}")
        print_data_item(response.item)
    except grpc.RpcError as e:
        logger.error(f"Ошибка при получении элемента: {e.code()}")

    # 4. Создание нового элемента
    print("\n[4] Создание нового элемента...")
    try:
        response = create_data(
            name="Тестовый элемент из Python",
            description="Этот элемент был создан через Python gRPC-клиент",
            value=42.5
        )
        print(f"    ✅ Элемент создан")
        print(f"    Сообщение: {response.message}")
        print_data_item(response.item)

        # Сохраняем ID для дальнейших тестов
        test_item_id = response.item.id
    except grpc.RpcError as e:
        logger.error(f"Ошибка при создании элемента: {e.code()}")
        test_item_id = None

    # 5. Обновление элемента
    if test_item_id:
        print(f"\n[5] Обновление элемента #{test_item_id}...")
        try:
            response = update_data(
                item_id=test_item_id,
                name="Обновлённый элемент",
                description="Описание было изменено",
                value=99.9
            )
            print(f"    ✅ Элемент обновлён")
            print(f"    Сообщение: {response.message}")
            print_data_item(response.item)
        except grpc.RpcError as e:
            logger.error(f"Ошибка при обновлении элемента: {e.code()}")

    # 6. Создание ещё одного элемента для демонстрации пагинации
    print("\n[6] Создание дополнительного элемента...")
    try:
        response = create_data(
            name="Ещё один элемент",
            description="Для демонстрации пагинации",
            value=15.75
        )
        print(f"    ✅ Элемент создан с ID: {response.item.id}")
    except grpc.RpcError as e:
        logger.error(f"Ошибка при создании элемента: {e.code()}")

    # 7. Получение обновлённого списка
    print("\n[7] Получение обновлённого списка элементов...")
    try:
        list_response = list_data(page=1, per_page=5)
        print(f"    Всего элементов: {list_response.total}")
        print(f"    Получено элементов: {len(list_response.items)}")
    except grpc.RpcError as e:
        logger.error(f"Ошибка при получении списка: {e.code()}")

    # 8. Удаление элемента
    if test_item_id:
        print(f"\n[8] Удаление элемента #{test_item_id}...")
        try:
            response = delete_data(test_item_id)
            if response.success:
                print(f"    ✅ Элемент успешно удалён")
                print(f"    Сообщение: {response.message}")
        except grpc.RpcError as e:
            logger.error(f"Ошибка при удалении элемента: {e.code()}")

    # 9. Проверка обработки ошибок - получение несуществующего элемента
    print("\n[9] Проверка обработки ошибок (элемент #999)...")
    try:
        get_data(999)
        print("    ⚠️ Ошибка: элемент должен быть не найден")
    except grpc.RpcError as e:
        if e.code() == grpc.StatusCode.NOT_FOUND:
            print(f"    ✅ Ошибка корректно обработана: NOT_FOUND")
        else:
            logger.error(f"Неожиданная ошибка: {e.code()}")

    # 10. Финальная проверка health
    print("\n[10] Финальная проверка работоспособности...")
    try:
        health = health_check()
        print(f"    Статус: {health.status}")
        print(f"    Элементов на сервере: {health.total_items}")
    except grpc.RpcError as e:
        logger.error(f"Ошибка при проверке health: {e.code()}")

    print("\n" + "=" * 60)
    print("Работа с gRPC-сервером завершена успешно!")
    print("=" * 60)


if __name__ == "__main__":
    main()
