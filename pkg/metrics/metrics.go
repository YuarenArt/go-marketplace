package metrics

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"time"
)

// Metrics содержит метрики Prometheus для мониторинга приложения
type Metrics struct {
	RequestDuration *prometheus.HistogramVec
	RequestCount    *prometheus.CounterVec
	ErrorCount      *prometheus.CounterVec
}

// NewMetrics инициализирует метрики Prometheus
func NewMetrics() *Metrics {
	m := &Metrics{
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "Время обработки HTTP-запросов в секундах",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path", "status"},
		),
		RequestCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_request_total",
				Help: "Общее количество HTTP-запросов",
			},
			[]string{"method", "path", "status"},
		),
		ErrorCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_error_total",
				Help: "Общее количество ошибок HTTP-запросов",
			},
			[]string{"method", "path", "status"},
		),
	}

	// Регистрация метрик в Prometheus
	prometheus.MustRegister(m.RequestDuration, m.RequestCount, m.ErrorCount)
	return m
}

// Middleware возвращает middleware для сбора метрик Prometheus
func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		method := c.Request.Method
		path := c.Request.URL.Path

		c.Next()

		status := fmt.Sprintf("%d", c.Writer.Status())
		duration := time.Since(start).Seconds()

		m.RequestDuration.WithLabelValues(method, path, status).Observe(duration)
		m.RequestCount.WithLabelValues(method, path, status).Inc()
		if c.Writer.Status() >= 400 {
			m.ErrorCount.WithLabelValues(method, path, status).Inc()
		}
	}
}

// Handler возвращает обработчик для эндпоинта Prometheus
func Handler() gin.HandlerFunc {
	return gin.WrapH(promhttp.Handler())
}
