#!/usr/bin/env python3
"""
Unit-тесты для Python gRPC-клиента.
Использует mocking для тестирования без реального сервера.
"""

import unittest
from unittest.mock import MagicMock, patch, PropertyMock
import grpc
import signal
import sys
import os

# Добавляем путь к модулю
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

# Импортируем модуль клиента
import client


class TestGracefulShutdown(unittest.TestCase):
    """Тесты для обработки graceful shutdown."""

    def setUp(self):
        """Сбрасывает флаг shutdown перед каждым тестом."""
        client.reset_shutdown_flag()

    def test_is_shutdown_requested_initial(self):
        """Проверяет, что изначально флаг shutdown=False."""
        self.assertFalse(client.is_shutdown_requested())

    def test_reset_shutdown_flag(self):
        """Проверяет сброс флага shutdown."""
        client._shutdown_requested = True
        client.reset_shutdown_flag()
        self.assertFalse(client.is_shutdown_requested())

    def test_check_shutdown_raises_error(self):
        """Проверяет, что _check_shutdown выбрасывает исключение."""
        client._shutdown_requested = True
        with self.assertRaises(client.GracefulShutdownError):
            client._check_shutdown()

    def test_check_shutdown_ok_when_not_requested(self):
        """Проверяет, что _check_shutdown не выбрасывает исключение."""
        client._shutdown_requested = False
        try:
            client._check_shutdown()
        except client.GracefulShutdownError:
            self.fail("_check_shutdown() raised GracefulShutdownError unexpectedly")


class TestConnectionManagement(unittest.TestCase):
    """Тесты для управления подключением."""

    def setUp(self):
        """Сбрасывает глобальные переменные."""
        client._channel = None
        client._stub = None

    def tearDown(self):
        """Закрывает канал после теста."""
        client.close_channel()

    @patch('client.grpc.insecure_channel')
    def test_get_channel_creates_channel(self, mock_channel):
        """Проверяет, что get_channel создаёт канал."""
        mock_channel.return_value = MagicMock()
        channel = client.get_channel()
        mock_channel.assert_called_once_with(client.GRPC_SERVER)
        self.assertIsNotNone(channel)

    @patch('client.grpc.insecure_channel')
    def test_get_channel_returns_existing(self, mock_channel):
        """Проверяет, что get_channel возвращает существующий канал."""
        mock_channel.return_value = MagicMock()
        channel1 = client.get_channel()
        channel2 = client.get_channel()
        self.assertIs(channel1, channel2)
        mock_channel.assert_called_once()

    @patch('client.data_pb2_grpc.DataServiceStub')
    @patch('client.grpc.insecure_channel')
    def test_get_stub_creates_stub(self, mock_channel, mock_stub_class):
        """Проверяет, что get_stub создаёт stub."""
        mock_channel.return_value = MagicMock()
        mock_stub = MagicMock()
        mock_stub_class.return_value = mock_stub
        
        stub = client.get_stub()
        
        mock_stub_class.assert_called_once()
        self.assertIsNotNone(stub)

    def test_close_channel_resets_state(self):
        """Проверяет, что close_channel сбрасывает состояние."""
        mock_channel = MagicMock()
        client._channel = mock_channel
        client._stub = MagicMock()
        
        client.close_channel()
        
        self.assertIsNone(client._channel)
        self.assertIsNone(client._stub)
        mock_channel.close.assert_called_once()

    def test_context_manager_closes_channel(self):
        """Проверяет, что контекстный менеджер закрывает канал."""
        with patch('client.grpc.insecure_channel') as mock_channel:
            mock_channel.return_value = MagicMock()
            with patch('client.data_pb2_grpc.DataServiceStub') as mock_stub:
                mock_stub.return_value = MagicMock()
                
                with client.grpc_connection() as stub:
                    self.assertIsNotNone(stub)
                
                # После выхода из контекста канал должен быть закрыт
                self.assertIsNone(client._channel)


class TestGRPCClientFunctions(unittest.TestCase):
    """Тесты для функций gRPC-клиента."""

    def setUp(self):
        """Настраивает моки для каждого теста."""
        client.reset_shutdown_flag()
        client._channel = None
        client._stub = None
        
        # Создаём моки
        self.mock_channel = MagicMock()
        self.mock_stub = MagicMock()
        
        # Патчим создание канала и stub
        self.channel_patcher = patch('client.grpc.insecure_channel', return_value=self.mock_channel)
        self.stub_patcher = patch('client.data_pb2_grpc.DataServiceStub', return_value=self.mock_stub)
        
        self.channel_patcher.start()
        self.stub_patcher.start()

    def tearDown(self):
        """Останавливает патчи и закрывает канал."""
        self.channel_patcher.stop()
        self.stub_patcher.stop()
        client.close_channel()

    def test_health_check(self):
        """Тестирует функцию health_check."""
        mock_response = MagicMock()
        mock_response.status = "healthy"
        mock_response.version = "1.0.0"
        mock_response.total_items = 5
        self.mock_stub.HealthCheck.return_value = mock_response

        response = client.health_check()
        
        self.mock_stub.HealthCheck.assert_called_once()
        self.assertEqual(response.status, "healthy")
        self.assertEqual(response.version, "1.0.0")
        self.assertEqual(response.total_items, 5)

    def test_create_data(self):
        """Тестирует функцию create_data."""
        mock_item = MagicMock()
        mock_item.id = 1
        mock_item.name = "Test Item"
        mock_item.description = "Test Description"
        mock_item.value = 100.5
        
        mock_response = MagicMock()
        mock_response.item = mock_item
        mock_response.message = "Item created successfully"
        self.mock_stub.CreateData.return_value = mock_response

        response = client.create_data(
            name="Test Item",
            description="Test Description",
            value=100.5
        )
        
        self.mock_stub.CreateData.assert_called_once()
        request = self.mock_stub.CreateData.call_args[0][0]
        self.assertEqual(request.name, "Test Item")
        self.assertEqual(request.description, "Test Description")
        self.assertEqual(request.value, 100.5)
        self.assertEqual(response.message, "Item created successfully")

    def test_get_data(self):
        """Тестирует функцию get_data."""
        mock_item = MagicMock()
        mock_item.id = 1
        mock_response = MagicMock()
        mock_response.item = mock_item
        self.mock_stub.GetData.return_value = mock_response

        response = client.get_data(1)
        
        self.mock_stub.GetData.assert_called_once()
        request = self.mock_stub.GetData.call_args[0][0]
        self.assertEqual(request.id, 1)

    def test_list_data(self):
        """Тестирует функцию list_data."""
        mock_items = [MagicMock(), MagicMock(), MagicMock()]
        mock_response = MagicMock()
        mock_response.items = mock_items
        mock_response.total = 10
        mock_response.page = 1
        mock_response.per_page = 5
        self.mock_stub.ListData.return_value = mock_response

        response = client.list_data(page=1, per_page=5)
        
        self.mock_stub.ListData.assert_called_once()
        request = self.mock_stub.ListData.call_args[0][0]
        self.assertEqual(request.page, 1)
        self.assertEqual(request.per_page, 5)
        self.assertEqual(response.total, 10)
        self.assertEqual(len(response.items), 3)

    def test_update_data(self):
        """Тестирует функцию update_data."""
        mock_item = MagicMock()
        mock_item.id = 1
        mock_item.name = "Updated"
        mock_response = MagicMock()
        mock_response.item = mock_item
        mock_response.message = "Updated successfully"
        self.mock_stub.UpdateData.return_value = mock_response

        response = client.update_data(
            item_id=1,
            name="Updated",
            value=200.0
        )

        self.mock_stub.UpdateData.assert_called_once()
        request = self.mock_stub.UpdateData.call_args[0][0]
        self.assertEqual(request.id, 1)
        self.assertEqual(request.name, "Updated")
        self.assertEqual(request.value, 200.0)
        self.assertTrue(request.update_value)

    def test_delete_data(self):
        """Тестирует функцию delete_data."""
        mock_response = MagicMock()
        mock_response.success = True
        mock_response.message = "Deleted successfully"
        self.mock_stub.DeleteData.return_value = mock_response

        response = client.delete_data(1)
        
        self.mock_stub.DeleteData.assert_called_once()
        request = self.mock_stub.DeleteData.call_args[0][0]
        self.assertEqual(request.id, 1)
        self.assertTrue(response.success)

    def test_grpc_error_handling(self):
        """Тестирует обработку gRPC ошибок."""
        self.mock_stub.GetData.side_effect = grpc.RpcError()
        self.mock_stub.GetData.side_effect.code = lambda: grpc.StatusCode.NOT_FOUND
        self.mock_stub.GetData.side_effect.details = lambda: "Item not found"

        with self.assertRaises(grpc.RpcError):
            client.get_data(999)


class TestSignalHandlers(unittest.TestCase):
    """Тесты для обработки сигналов."""

    def setUp(self):
        """Сбрасывает флаг shutdown."""
        client.reset_shutdown_flag()

    def test_signal_handler_sets_flag(self):
        """Проверяет, что обработчик сигнала устанавливает флаг."""
        client._signal_handler(signal.SIGINT, None)
        self.assertTrue(client.is_shutdown_requested())

    def test_signal_handler_sigterm(self):
        """Проверяет обработку SIGTERM."""
        client._signal_handler(signal.SIGTERM, None)
        self.assertTrue(client.is_shutdown_requested())


if __name__ == '__main__':
    unittest.main()
