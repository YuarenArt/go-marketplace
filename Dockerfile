# --- Build stage ---
FROM golang:1.24-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git 

# Кэшируем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходники
COPY . .

# Собираем бинарник server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server ./cmd/server

# --- Final stage ---
FROM scratch

# Копируем бинарник server
COPY --from=builder /app/server /go-marketplace-server

# (Опционально) Копируем Swagger-статик, если нужен (например, docs/swagger/*)
COPY --from=builder /src/docs/swagger /docs/swagger

# Создаем непривилегированного пользователя
USER 10001:10001

WORKDIR /

EXPOSE 8080

# Healthcheck (опционально, если есть endpoint)
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget --spider -q http://localhost:8080/swagger/index.html || exit 1

# Запуск только server
ENTRYPOINT ["/go-marketplace-server"] 