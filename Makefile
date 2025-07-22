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

# Путь к файлам vegeta
VEGETA_TARGETS=load/targets.txt
VEGETA_RESULTS=load/results.bin
VEGETA_REPORT=load/report.txt
VEGETA_PLOT=load/plot.html

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

## Запуск нагрузочного теста vegeta
load-test: vegeta-targets
	mkdir -p load
	vegeta attack -targets=$(VEGETA_TARGETS) -duration=30s -rate=1000 | tee $(VEGETA_RESULTS) | vegeta report > $(VEGETA_REPORT)
	vegeta plot load/results.bin > load/plot.html
	@echo "Готово: текстовый отчёт в $(VEGETA_REPORT), график: открой $(VEGETA_PLOT)"

## Создание файла с целями запроса
vegeta-targets:
	mkdir -p load
	echo "POST http://localhost:$(PORT)/register" > $(VEGETA_TARGETS)
	echo "POST http://localhost:$(PORT)/login" >> $(VEGETA_TARGETS)
	echo "GET http://localhost:$(PORT)/ads" >> $(VEGETA_TARGETS)

## Очистка результатов тестов
load-clean:
	rm -rf load