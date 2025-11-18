# Сервис назначения ревьюеров для Pull Request'ов (Go + PostgreSQL)

## Описание

Микросервис, реализующий логику автоматического назначения ревьюверов для PR. Основан на многослойной архитектуре (Clean Architecture) с использованием Go и PostgreSQL.

**Функциональность:**
1.  **Назначение ревьюверов:** До двух **активных** членов из команды автора, исключая самого автора.
2.  **Переназначение:** Случайная замена ревьювера на другого **активного** члена из той же команды (кроме автора и уже назначенного).
3.  **Merge-контроль:** Запрет на любые изменения состава ревьюверов после установки статуса `MERGED`.
4.  **Управление:** Эндпоинты для создания команд, добавления/обновления пользователей и управления их активностью.

## Стек

* **Язык:** Go (Golang)
* **База данных:** PostgreSQL
* **Маршрутизатор:** `gorilla/mux`
* **Запуск:** Docker и Docker Compose

## Запуск проекта

### 1. Требования

Для запуска вам понадобятся установленные Docker и Docker Compose.

### 2. Запуск

Выполните команду в корне проекта:

    ```
    docker compose up --build
    ```

Сервис будет доступен по адресу `http://localhost:8080`.

### 3. Curl запросы

Проверка Health Check : ```curl -X GET http://localhost:8080/health```

Создание команды (пример): ``` curl -X POST http://localhost:8080/team/add -H "Content-Type: application/json" -d '{"team_name":"backend-team","members":[{"user_id":"u1","username":"Alice","is_active":true},{"user_id":"u2","username":"Bob","is_active":true},{"user_id":"u3","username":"Charlie","is_active":true}]}'```

Создание PullRequest (пример) : ```curl -X POST http://localhost:8080/pullRequest/create -H "Content-Type: application/json" -d '{"pull_request_id":"pr-101","pull_request_name":"Feature X implementation","author_id":"u1"}'```

Добавление нового активного участника : ```curl -X POST http://localhost:8080/team/add -H "Content-Type: application/json" -d '{"team_name":"backend-team","members":[{"user_id":"u1","username":"Alice","is_active":true},{"user_id":"u2","username":"Bob","is_active":true},{"user_id":"u3","username":"Charlie","is_active":true},{"user_id":"u4","username":"David","is_active":true}]}' ```

Замена ревьюера : ```curl -X POST http://localhost:8080/pullRequest/reassign -H "Content-Type: application/json" -d '{"pull_request_id":"pr-101","old_user_id":"u2"}' ```

Получение PullRequest ревьюера : ```curl -X GET "http://localhost:8080/users/getReview?user_id=u4" ```
