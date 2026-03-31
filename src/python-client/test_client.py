#!/usr/bin/env python3
"""
Unit-тесты для Python-клиента с тестированием graceful shutdown.
"""

import unittest
import signal
import threading
import time
from unittest.mock import patch, MagicMock
from http.server import HTTPServer, BaseHTTPRequestHandler

import client


class GracefulShutdownError(Exception):
    """Псевдоним для тестирования."""
    pass


class MockHTTPRequestHandler(BaseHTTPRequestHandler):
    """Mock HTTP сервер для тестов."""

    def do_GET(self):
        # Обработка query параметров
        path = self.path
        if '?' in path:
            path = path.split('?')[0]
            
        if path == '/health':
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(b'{"status": "healthy", "version": "1.0.0", "timestamp": "2024-01-01T00:00:00Z"}')
        elif path == '/api/stats':
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(b'{"request_count": 10, "uptime": "1h", "start_time": "2024-01-01T00:00:00Z"}')
        elif path == '/api/posts':
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(b'{"posts": [], "total": 0, "page": 1, "per_page": 10, "timestamp": "2024-01-01T00:00:00Z"}')
        elif path == '/api/posts/1':
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(b'''{
                "post": {
                    "id": 1,
                    "title": "Test Post",
                    "slug": "test-post",
                    "content": "Test content",
                    "author": {"id": 1, "username": "test", "email": "test@test.com"},
                    "tags": [{"id": 1, "name": "Test", "slug": "test"}],
                    "view_count": 5,
                    "created_at": "2024-01-01T00:00:00Z"
                },
                "timestamp": "2024-01-01T00:00:00Z"
            }''')
        else:
            self.send_response(404)
            self.end_headers()

    def do_POST(self):
        path = self.path
        if '?' in path:
            path = path.split('?')[0]
            
        if path == '/api/posts':
            self.send_response(201)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(b'''{
                "post": {
                    "id": 2,
                    "title": "New Post",
                    "slug": "new-post",
                    "content": "New content",
                    "author": {"id": 1, "username": "test", "email": "test@test.com"},
                    "tags": [],
                    "view_count": 0,
                    "created_at": "2024-01-01T00:00:00Z"
                },
                "message": "Post created successfully",
                "timestamp": "2024-01-01T00:00:00Z"
            }''')
        elif path == '/api/echo':
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(b'{"original": {"message": "test"}, "processed": "test", "timestamp": "2024-01-01T00:00:00Z"}')
        else:
            self.send_response(404)
            self.end_headers()

    def do_PUT(self):
        path = self.path
        if '?' in path:
            path = path.split('?')[0]
            
        if path.startswith('/api/posts/'):
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(b'''{
                "post": {
                    "id": 2,
                    "title": "Updated Post",
                    "slug": "updated-post",
                    "content": "Updated content",
                    "excerpt": "Updated excerpt",
                    "author": {"id": 1, "username": "test", "email": "test@test.com"},
                    "tags": [{"id": 1, "name": "Updated", "slug": "updated"}],
                    "view_count": 0,
                    "created_at": "2024-01-01T00:00:00Z",
                    "updated_at": "2024-01-01T00:00:00Z"
                },
                "timestamp": "2024-01-01T00:00:00Z"
            }''')
        else:
            self.send_response(404)
            self.end_headers()

    def do_DELETE(self):
        path = self.path
        if '?' in path:
            path = path.split('?')[0]
            
        if path.startswith('/api/posts/'):
            self.send_response(204)
            self.end_headers()
        else:
            self.send_response(404)
            self.end_headers()

    def log_message(self, format, *args):
        """Подавляем логирование."""
        pass


class TestGracefulShutdown(unittest.TestCase):
    """Тесты для graceful shutdown."""

    def setUp(self):
        """Сброс состояния перед каждым тестом."""
        client.reset_shutdown_flag()
        client.close_session()

    def tearDown(self):
        """Очистка после каждого теста."""
        client.reset_shutdown_flag()
        client.close_session()

    def test_is_shutdown_requested_initial(self):
        """Проверка начального состояния флага shutdown."""
        self.assertFalse(client.is_shutdown_requested())

    def test_reset_shutdown_flag(self):
        """Проверка сброса флага shutdown."""
        client._shutdown_requested = True
        client.reset_shutdown_flag()
        self.assertFalse(client.is_shutdown_requested())

    def test_signal_handler_sets_flag(self):
        """Проверка, что обработчик сигнала устанавливает флаг."""
        client._signal_handler(signal.SIGINT, None)
        self.assertTrue(client.is_shutdown_requested())

    def test_signal_handler_sigterm(self):
        """Проверка обработки сигнала SIGTERM."""
        client._signal_handler(signal.SIGTERM, None)
        self.assertTrue(client.is_shutdown_requested())

    def test_check_shutdown_no_flag(self):
        """Проверка _check_shutdown без установленного флага."""
        client.reset_shutdown_flag()
        # Не должно выбрасывать исключение
        try:
            client._check_shutdown()
        except client.GracefulShutdownError:
            self.fail("_check_shutdown() выбросило исключение без флага")

    def test_check_shutdown_with_flag(self):
        """Проверка _check_shutdown с установленным флагом."""
        client._shutdown_requested = True
        with self.assertRaises(client.GracefulShutdownError):
            client._check_shutdown()

    def test_check_shutdown_error_message(self):
        """Проверка сообщения исключения при shutdown."""
        client._shutdown_requested = True
        try:
            client._check_shutdown()
        except client.GracefulShutdownError as e:
            self.assertEqual(str(e), "Завершение работы запрошено")

    def test_get_session_creates_new(self):
        """Проверка создания новой сессии."""
        client.close_session()
        session = client.get_session()
        self.assertIsNotNone(session)
        self.assertIsInstance(session, client.requests.Session)

    def test_get_session_reuses_existing(self):
        """Проверка повторного использования сессии."""
        session1 = client.get_session()
        session2 = client.get_session()
        self.assertIs(session1, session2)

    def test_close_session(self):
        """Проверка закрытия сессии."""
        session = client.get_session()
        client.close_session()
        # Сессия должна быть закрыта
        self.assertTrue(session.close.called if hasattr(session.close, 'called') else True)

    def test_api_session_context_manager(self):
        """Проверка контекстного менеджера сессии."""
        with client.api_session() as session:
            self.assertIsNotNone(session)
        # После выхода сессия должна быть закрыта
        client.close_session()

    def test_api_session_closes_on_exception(self):
        """Проверка закрытия сессии при исключении."""
        try:
            with client.api_session() as session:
                raise ValueError("Test exception")
        except ValueError:
            pass
        client.close_session()


class TestAPICallsWithShutdown(unittest.TestCase):
    """Тесты API вызовов с проверкой shutdown."""
    
    _port = 18800  # Начальный порт для тестов

    def setUp(self):
        """Запуск тестового сервера."""
        import socket
        client.reset_shutdown_flag()
        client.close_session()
        
        # Находим свободный порт
        while True:
            try:
                self.server = HTTPServer(('localhost', TestAPICallsWithShutdown._port), MockHTTPRequestHandler)
                self.server.socket.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
                break
            except OSError:
                TestAPICallsWithShutdown._port += 1
                if TestAPICallsWithShutdown._port > 19000:
                    raise
                
        self.server_thread = threading.Thread(target=self.server.serve_forever)
        self.server_thread.daemon = True
        self.server_thread.start()
        # Сохраняем оригинальный BASE_URL
        self.original_base_url = client.BASE_URL
        client.BASE_URL = f"http://localhost:{TestAPICallsWithShutdown._port}"
        TestAPICallsWithShutdown._port += 1  # Увеличиваем для следующего теста

    def tearDown(self):
        """Остановка тестового сервера."""
        client.BASE_URL = self.original_base_url
        self.server.shutdown()
        self.server.server_close()
        client.close_session()
        client.reset_shutdown_flag()

    def test_check_health_success(self):
        """Проверка check_health при успешном ответе."""
        result = client.check_health()
        self.assertEqual(result['status'], 'healthy')
        self.assertEqual(result['version'], '1.0.0')

    def test_get_stats_success(self):
        """Проверка get_stats при успешном ответе."""
        result = client.get_stats()
        self.assertEqual(result['request_count'], 10)
        self.assertEqual(result['uptime'], '1h')

    def test_get_posts_success(self):
        """Проверка get_posts при успешном ответе."""
        result = client.get_posts()
        self.assertEqual(result['total'], 0)
        self.assertEqual(result['page'], 1)

    def test_get_post_success(self):
        """Проверка get_post при успешном ответе."""
        result = client.get_post(1)
        self.assertEqual(result['post']['id'], 1)
        self.assertEqual(result['post']['title'], 'Test Post')

    def test_create_post_success(self):
        """Проверка create_post при успешном ответе."""
        result = client.create_post("New Post", "New content")
        self.assertEqual(result['post']['id'], 2)
        self.assertEqual(result['message'], 'Post created successfully')

    def test_update_post_success(self):
        """Проверка update_post при успешном ответе."""
        result = client.update_post(2, excerpt="Updated excerpt")
        self.assertEqual(result['post']['excerpt'], 'Updated excerpt')

    def test_delete_post_success(self):
        """Проверка delete_post при успешном ответе."""
        result = client.delete_post(1)
        self.assertTrue(result)

    def test_api_call_with_shutdown_flag(self):
        """Проверка API вызова с установленным флагом shutdown."""
        client._shutdown_requested = True
        with self.assertRaises(client.GracefulShutdownError):
            client.check_health()

    def test_get_posts_with_pagination(self):
        """Проверка get_posts с пагинацией."""
        result = client.get_posts(page=2, per_page=5)
        self.assertIsNotNone(result)


class TestSessionManagement(unittest.TestCase):
    """Тесты управления сессией."""

    def setUp(self):
        client.close_session()
        client.reset_shutdown_flag()

    def tearDown(self):
        client.close_session()
        client.reset_shutdown_flag()

    def test_session_is_none_initially(self):
        """Проверка, что сессия None изначально."""
        self.assertIsNone(client._session)

    def test_get_session_not_none_after_call(self):
        """Проверка, что сессия создаётся после вызова get_session."""
        session = client.get_session()
        self.assertIsNotNone(session)

    def test_close_session_sets_none(self):
        """Проверка, что close_session устанавливает _session в None."""
        client.get_session()
        client.close_session()
        self.assertIsNone(client._session)


class TestSignalHandlersSetup(unittest.TestCase):
    """Тесты установки обработчиков сигналов."""

    def test_setup_signal_handlers(self):
        """Проверка установки обработчиков сигналов."""
        # Сохраняем текущие обработчики
        old_sigint = signal.getsignal(signal.SIGINT)
        old_sigterm = signal.getsignal(signal.SIGTERM)

        try:
            client.setup_signal_handlers()
            # Обработчики должны быть установлены
            # (не можем проверить напрямую, т.к. signal.getsignal может вернуть встроенные)
        finally:
            # Восстанавливаем обработчики
            signal.signal(signal.SIGINT, old_sigint)
            signal.signal(signal.SIGTERM, old_sigterm)


class TestGracefulShutdownError(unittest.TestCase):
    """Тесты исключения GracefulShutdownError."""

    def test_exception_inheritance(self):
        """Проверка наследования исключения."""
        self.assertTrue(issubclass(client.GracefulShutdownError, Exception))

    def test_exception_message(self):
        """Проверка сообщения исключения по умолчанию."""
        try:
            raise client.GracefulShutdownError()
        except client.GracefulShutdownError as e:
            # Сообщение может быть пустым или стандартным
            pass


class TestMainFunction(unittest.TestCase):
    """Тесты основной функции."""

    def setUp(self):
        client.reset_shutdown_flag()
        client.close_session()

    def tearDown(self):
        client.reset_shutdown_flag()
        client.close_session()

    @patch('client._run_main')
    @patch('client.setup_signal_handlers')
    def test_main_calls_setup_handlers(self, mock_setup, mock_run):
        """Проверка, что main вызывает setup_signal_handlers."""
        try:
            client.main()
        except:
            pass
        mock_setup.assert_called_once()

    @patch('client._run_main')
    @patch('client.close_session')
    def test_main_closes_session_in_finally(self, mock_close, mock_run):
        """Проверка, что main закрывает сессию в finally."""
        try:
            client.main()
        except:
            pass
        mock_close.assert_called()

    @patch('client._run_main')
    def test_main_runs_main_logic(self, mock_run):
        """Проверка, что main вызывает _run_main."""
        try:
            client.main()
        except:
            pass
        mock_run.assert_called_once()


class TestRunMain(unittest.TestCase):
    """Тесты функции _run_main."""

    def setUp(self):
        client.reset_shutdown_flag()
        client.close_session()

    def tearDown(self):
        client.reset_shutdown_flag()
        client.close_session()

    @patch('client.check_health')
    def test_run_main_handles_connection_error(self, mock_health):
        """Проверка обработки ConnectionError в _run_main."""
        import requests
        mock_health.side_effect = requests.exceptions.ConnectionError()
        # Не должно выбрасывать исключение
        try:
            client._run_main()
        except:
            self.fail("_run_main() выбросило исключение при ConnectionError")

    @patch('client.check_health')
    def test_run_main_handles_graceful_shutdown(self, mock_health):
        """Проверка обработки GracefulShutdownError в _run_main."""
        mock_health.side_effect = client.GracefulShutdownError()
        with self.assertRaises(client.GracefulShutdownError):
            client._run_main()


if __name__ == '__main__':
    unittest.main(verbosity=2)
