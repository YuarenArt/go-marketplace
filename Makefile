# Параметры
APP_NAME=go-marketplace-server
CMD_DIR=cmd/go-marketplace-server
BIN_DIR=bin
BIN_PATH=$(BIN_DIR)/$(APP_NAME)
SWAGGER_DIR=docs/swagger

# Переменные окружения для локального запуска
export PORT ?= 8080
export SECRET_KEY ?= supersecret
export PG_HOST ?= localhost
export PG_PORT ?= 5432
export PG_USER ?= postgres
export PG_PASSWORD ?= password
export PG_DBNAME ?= marketgo

.PHONY: all build run test fmt int swagger swagger-ui docker-build docker-up docker-down clean

all: build

## Сборка бинарника
build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_PATH) ./$(CMD_DIR)

## Запуск приложения (локально)
run: build
	$(BIN_PATH)

## Тестирование с покрытием
test:
	go test -cover ./...

## Форматирование кода
fmt:
	go fmt ./...

## Генерация Swagger-документации
swagger:
	swag init --parseDependency --parseInternal --output docs/swagger -g cmd/go-marketplace-server/main.go

## Открыть Swagger UI в браузере (по умолчанию на localhost:8080/swagger/index.html)
swagger-ui:
	@echo "Откройте http://localhost:8080/swagger/index.html в браузере"

## Сборка Docker-образа
docker-build:
	docker build -t $(APP_NAME):latest .

## Запуск через docker-compose
docker-up:
	docker compose up --build

## Остановка docker-compose
docker-down:
	docker compose down

## Очистка артефактов сборки
clean:
	rm -rf $(BIN_DIR) 