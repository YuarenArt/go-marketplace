# Marketplace API

## Описание

**Marketplace API** — это тестовое задание для стажировки, реализующее backend для онлайн-маркетплейса с поддержкой регистрации, аутентификации пользователей и операциями создания и просмотра объявлений. Проект построен на Go, использует PostgreSQL, фреймворк Gin, JWT для авторизации и Swagger для автодокументации.

---

## Основные возможности

- **Регистрация и аутентификация пользователей** (JWT, bcrypt)
- **Создание и просмотр объявлений** с фильтрацией, сортировкой и пагинацией
- **REST API** с подробной документацией (Swagger UI)
- **Docker**-окружение для быстрого старта
- **Покрытие тестами** (unit и integration)
- **Строгая валидация входных данных**
- **Логирование** запросов и ошибок

---

## Быстрый старт

### 1. Клонирование репозитория

```sh
git clone https://github.com/YuarenArt/marketgo.git
cd marketgo
```

### 2. Запуск через Docker Compose

> Требуется установленный Docker и docker-compose.

```sh
docker compose up --build
```

- API будет доступен на: [http://localhost:8080](http://localhost:8080)
- Swagger UI: [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)
- База данных PostgreSQL: порт 5432

### 3. Локальный запуск (без Docker)

1. Установите Go 1.24+ и PostgreSQL.
2. Создайте базу данных и пользователя (или используйте параметры по умолчанию).
3. Установите переменные окружения (или создайте `.env`):

```env
PORT=8080
SECRET_KEY=supersecret
PG_HOST=localhost
PG_PORT=5432
PG_USER=postgres
PG_PASSWORD=password
PG_DBNAME=marketgo
```

4. Соберите и запустите приложение:

```sh
make build
make run
```

---

## Архитектура и основные компоненты

- **Gin** — HTTP-фреймворк, маршрутизация, middlewares
- **PostgreSQL** — хранение пользователей и объявлений
- **JWT** — авторизация (заголовок `X-Auth-Token`)
- **bcrypt** — безопасное хранение паролей
- **Swagger** — автогенерация и просмотр API-документации
- **Тесты** — покрытие бизнес-логики и работы с БД

---

## Работа с API

### Аутентификация

- Используется JWT-токен, который возвращается при логине.
- Для защищённых эндпоинтов требуется заголовок:  
  `X-Auth-Token: <jwt>`

### Основные эндпоинты

#### Регистрация

```
POST /register
Content-Type: application/json

{
  "login": "username",
  "password": "password123"
}
```

- Ответ: объект пользователя или ошибка

#### Логин

```
POST /login
Content-Type: application/json

{
  "login": "username",
  "password": "password123"
}
```

- Ответ: `{ "token": "<jwt>" }`

#### Получение объявлений

```
GET /ads
X-Auth-Token: <jwt>
```

- Параметры query:
  - `page` (int, default=1)
  - `page_size` (int, default=10)
  - `sort_by` (`created_at` или `price`)
  - `sort_order` (`ASC` или `DESC`)
  - `min_price`, `max_price` (фильтрация по цене)

#### Создание объявления

```
POST /ads
X-Auth-Token: <jwt>
Content-Type: application/json

{
  "title": "Название",
  "text": "Описание",
  "image_url": "https://...",
  "price": 10000
}
```

- Ответ: созданное объявление

### Swagger UI

- Открыть: [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)
- Вся спецификация и примеры запросов доступны в интерфейсе.

---

## Тестирование

- Запуск всех тестов:
  ```sh
  make test
  ```
- Используются:
  - **testcontainers-go** — для изолированных тестов с реальной PostgreSQL
  - **testify** — для удобных assert/require

---

## Переменные окружения

| Переменная      | Описание                | Значение по умолчанию |
|-----------------|------------------------|-----------------------|
| PORT            | Порт HTTP сервера       | 8080                  |
| SECRET_KEY      | JWT secret              | supersecret           |
| PG_HOST         | Хост PostgreSQL         | localhost             |
| PG_PORT         | Порт PostgreSQL         | 5432                  |
| PG_USER         | Пользователь PostgreSQL | postgres              |
| PG_PASSWORD     | Пароль PostgreSQL       | password              |
| PG_DBNAME       | Имя БД                  | marketgo              |

---

## Сборка и запуск вручную

```sh
make build      # Сборка бинарника
make run        # Запуск приложения
make swagger    # Генерация Swagger-документации
make docker-up  # Запуск через docker-compose
make docker-down # Остановка docker-compose
make clean      # Очистка артефактов сборки
```

---

## Валидация и ограничения

- **Пароль**: 8–72 символа, хранится в bcrypt
- **Логин**: 4–20 символов, уникальный
- **Объявление**:
  - title: 2–100 символов
  - text: 1–2000 символов
  - image_url: валидный URL
  - price: 1–100 000 000 

---

## Примечания

- Все ошибки возвращаются в формате JSON с понятным сообщением.
- Для локального запуска можно использовать `.env` или переменные окружения.
- Swagger-документация генерируется командой `make swagger` (требуется установленный swag).
