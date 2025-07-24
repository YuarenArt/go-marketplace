package services

import (
	"context"

	"github.com/YuarenArt/marketgo/internal/db"
)

const (
	DefaultSortBy    = "created_at"
	DefaultSortOrder = "DESC"
	DefaultMaxPrice  = 100_000_000
)

// CreateAdRequest представляет запрос для создания объявления
type CreateAdRequest struct {
	Title    string `json:"title" binding:"required,min=2,max=100"`
	Text     string `json:"text" binding:"required,min=1,max=2000"`
	ImageURL string `json:"image_url" binding:"required,url"`
	Price    int64  `json:"price" binding:"required,gte=1,lte=100000000"`
}

// GetAdsRequest представляет запрос для получения списка объявлений
type GetAdsRequest struct {
	Page      int    `json:"page" binding:"required,gte=1"`
	PageSize  int    `json:"page_size" binding:"required,gte=1,lte=100"`
	SortBy    string `json:"sort_by" binding:"omitempty,oneof=created_at price"`
	SortOrder string `json:"sort_order" binding:"omitempty,oneof=ASC DESC"`
	MinPrice  int64  `json:"min_price" binding:"omitempty,gte=0"`
	MaxPrice  int64  `json:"max_price" binding:"omitempty,gte=0"`
}

// AdService предоставляет методы для работы с объявлениями
type AdService struct {
	db *db.DBService
}

// NewAdService создает новый экземпляр AdService
func NewAdService(db *db.DBService) *AdService {
	return &AdService{db: db}
}

// CreateAd создает новое объявление, связанное с userID
func (s *AdService) CreateAd(ctx context.Context, req CreateAdRequest, userID int) (db.Ad, error) {
	ad := db.Ad{
		Title:    req.Title,
		Text:     req.Text,
		ImageURL: req.ImageURL,
		Price:    req.Price,
		UserID:   userID,
	}
	return s.db.CreateAd(ctx, ad)
}

// GetAds возвращает список объявлений с учетом фильтров и сортировки
func (s *AdService) GetAds(ctx context.Context, req GetAdsRequest, userID int) ([]db.Ad, error) {
	if req.SortBy == "" {
		req.SortBy = DefaultSortBy
	}
	if req.SortOrder == "" {
		req.SortOrder = DefaultSortOrder
	}
	if req.MaxPrice == 0 {
		req.MaxPrice = DefaultMaxPrice
	}
	return s.db.Ads(ctx, userID, req.Page, req.PageSize, req.SortBy, req.SortOrder, req.MinPrice, req.MaxPrice)
}
