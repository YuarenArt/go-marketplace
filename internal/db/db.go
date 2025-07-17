package db

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultPingTimeoutSecond = 5 * time.Second

	minTitleLength = 2
	maxTitleLength = 100
	maxTextLength  = 2000
	minPrice       = 1
	maxPrice       = 100_000_000
)

var (
	ErrUserNotFound       = errors.New("пользователь с указанным ID не существует")
	ErrInvalidSortBy      = errors.New("допустима сортировка только по полям created_at или price")
	ErrInvalidSortOrder   = errors.New("сортировка должна быть ASC или DESC")
	ErrInvalidTitleLength = errors.New("заголовок должен содержать от 2 до 100 символов")
	ErrInvalidTextLength  = errors.New("текст должен содержать от 1 до 2000 символов")
	ErrInvalidImageURL    = errors.New("некорректный формат URL изображения")
	ErrInvalidPrice       = errors.New("цена должна быть в диапазоне от 1 до 100 000 000 (копеек)")
	ErrInvalidUserID      = errors.New("некорректный идентификатор пользователя")
)

// DBService предоставляет методы для взаимодействия с базой данных PostgreSQL.
type DBService struct {
	pool *pgxpool.Pool
}

// User представляет пользователя системы.
type User struct {
	ID        int       `json:"id"`
	Login     string    `json:"login"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

// Ad представляет объявление. Цена указана в копейках.
type Ad struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Text      string    `json:"text"`
	ImageURL  string    `json:"image_url"`
	Price     int64     `json:"price"`
	UserID    int       `json:"user_id"`
	Author    string    `json:"author"`
	IsMine    bool      `json:"is_mine,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// DBOption определяет функцию, изменяющую конфигурацию подключения.
type DBOption func(*pgxpool.Config)

// WithMaxConns задаёт максимальное количество соединений в пуле.
func WithMaxConns(n int32) DBOption {
	return func(cfg *pgxpool.Config) {
		cfg.MaxConns = n
	}
}

// WithMinConns задаёт минимальное количество соединений в пуле.
func WithMinConns(n int32) DBOption {
	return func(cfg *pgxpool.Config) {
		cfg.MinConns = n
	}
}

// WithConnMaxLifetime задаёт максимальное время жизни соединения.
func WithConnMaxLifetime(d time.Duration) DBOption {
	return func(cfg *pgxpool.Config) {
		cfg.MaxConnLifetime = d
	}
}

// WithConnIdleLifetime задаёт время жизни неактивного соединения.
func WithConnIdleLifetime(d time.Duration) DBOption {
	return func(cfg *pgxpool.Config) {
		cfg.MaxConnIdleTime = d
	}
}

// NewDBService создаёт сервис базы данных с заданными параметрами.
func NewDBService(ctx context.Context, dsn string, opts ...DBOption) (*DBService, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSN: %w", err)
	}

	for _, opt := range opts {
		opt(cfg)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	ctxPing, cancel := context.WithTimeout(ctx, defaultPingTimeoutSecond)
	defer cancel()

	if err := pool.Ping(ctxPing); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if _, err := pool.Exec(ctx, CreateDb); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &DBService{pool: pool}, nil
}

// Close закрывает соединение с базой данных.
func (s *DBService) Close() error {
	s.pool.Close()
	return nil
}

// CreateUser создаёт нового пользователя в базе данных с переданным логином и хешированным паролем.
func (s *DBService) CreateUser(ctx context.Context, login, hashedPassword string) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, QueryCreateUser, login, hashedPassword).Scan(
		&user.ID, &user.Login, &user.CreatedAt,
	)
	if err != nil {
		return User{}, fmt.Errorf("failed to create user: %w", err)
	}
	return user, nil
}

// UserByLogin возвращает пользователя по логину.
func (s *DBService) UserByLogin(ctx context.Context, login string) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, QueryGetUserByLogin, login).Scan(
		&user.ID, &user.Login, &user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, fmt.Errorf("user not found: %w", err)
		}
		return User{}, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

// CreateAd создаёт новое объявление.
func (s *DBService) CreateAd(ctx context.Context, ad Ad) (Ad, error) {
	if err := validateAd(ad); err != nil {
		return Ad{}, err
	}

	var user User
	err := s.pool.QueryRow(ctx, QueryGetUserById, ad.UserID).Scan(
		&user.ID, &user.Login, &user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Ad{}, ErrUserNotFound
		}
		return Ad{}, fmt.Errorf("failed to verify user: %w", err)
	}

	var createdAd Ad
	err = s.pool.QueryRow(ctx, QueryCreateAd, ad.Title, ad.Text, ad.ImageURL, ad.Price, ad.UserID).Scan(
		&createdAd.ID, &createdAd.Title, &createdAd.Text, &createdAd.ImageURL,
		&createdAd.Price, &createdAd.UserID, &createdAd.CreatedAt, &createdAd.Author, &createdAd.IsMine,
	)
	if err != nil {
		return Ad{}, fmt.Errorf("failed to create ad: %w", err)
	}

	return createdAd, nil
}

// Ads возвращает список объявлений по фильтрам, пагинации и сортировке.
func (s *DBService) Ads(
	ctx context.Context,
	userID int,
	page, size int,
	sortBy, sortOrder string,
	minPrice, maxPrice float64,
) ([]Ad, error) {
	if sortBy != "created_at" && sortBy != "price" {
		return nil, ErrInvalidSortBy
	}
	if sortOrder != "ASC" && sortOrder != "DESC" {
		return nil, ErrInvalidSortOrder
	}

	offset := (page - 1) * size
	query := fmt.Sprintf(QueryGetAds, sortBy, sortOrder)

	rows, err := s.pool.Query(ctx, query, userID, minPrice, maxPrice, size, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query ads: %w", err)
	}
	defer rows.Close()

	var ads []Ad
	for rows.Next() {
		var ad Ad
		err := rows.Scan(
			&ad.ID, &ad.Title, &ad.Text, &ad.ImageURL, &ad.Price,
			&ad.UserID, &ad.CreatedAt, &ad.Author, &ad.IsMine,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to query ads: %w", err)
		}
		ads = append(ads, ad)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	return ads, nil
}

// validateAd выполняет валидацию объявления.
func validateAd(ad Ad) error {
	title := strings.TrimSpace(ad.Title)
	if len(title) < minTitleLength || len(title) > maxTitleLength {
		return ErrInvalidTitleLength
	}

	text := strings.TrimSpace(ad.Text)
	if len(text) == 0 || len(text) > maxTextLength {
		return ErrInvalidTextLength
	}

	if ad.ImageURL != "" {
		_, err := url.ParseRequestURI(ad.ImageURL)
		if err != nil {
			return ErrInvalidImageURL
		}
	}

	if ad.Price < minPrice || ad.Price > maxPrice {
		return ErrInvalidPrice
	}

	if ad.UserID <= 0 {
		return ErrInvalidUserID
	}

	return nil
}
