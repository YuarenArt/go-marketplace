package services

import (
	"context"
	"github.com/YuarenArt/marketgo/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewAdService(t *testing.T) {
	t.Run("create ad service successfully", func(t *testing.T) {
		adService := NewAdService(testDB)
		assert.NotNil(t, adService)
		assert.Equal(t, testDB, adService.db)
	})
}

func TestCreateAd(t *testing.T) {
	adService := NewAdService(testDB)

	// Подготовка: создаем пользователя
	user, err := testDB.CreateUser(testCtx, "aduser", "hashedpass")
	require.NoError(t, err)

	validReq := CreateAdRequest{
		Title:    "Valid Ad Title",
		Text:     "Valid ad text content",
		ImageURL: "https://example.com/image.png",
		Price:    1000,
	}

	t.Run("create ad successfully", func(t *testing.T) {
		createdAd, err := adService.CreateAd(testCtx, validReq, user.ID)
		require.NoError(t, err)
		assert.Equal(t, validReq.Title, createdAd.Title)
		assert.Equal(t, validReq.Text, createdAd.Text)
		assert.Equal(t, validReq.ImageURL, createdAd.ImageURL)
		assert.Equal(t, validReq.Price, createdAd.Price)
		assert.Equal(t, user.ID, createdAd.UserID)
		assert.False(t, createdAd.CreatedAt.IsZero())
		assert.Equal(t, "aduser", createdAd.Author)
		assert.True(t, createdAd.IsMine)
	})

	t.Run("invalid user ID returns error", func(t *testing.T) {
		_, err := adService.CreateAd(testCtx, validReq, 999999)
		assert.ErrorIs(t, err, db.ErrUserNotFound)
	})

	t.Run("invalid title returns error", func(t *testing.T) {
		invalidReq := validReq
		invalidReq.Title = "A"
		_, err := adService.CreateAd(testCtx, invalidReq, user.ID)
		assert.ErrorIs(t, err, db.ErrInvalidTitleLength)
	})

	t.Run("invalid text returns error", func(t *testing.T) {
		invalidReq := validReq
		invalidReq.Text = ""
		_, err := adService.CreateAd(testCtx, invalidReq, user.ID)
		assert.ErrorIs(t, err, db.ErrInvalidTextLength)
	})

	t.Run("invalid image URL returns error", func(t *testing.T) {
		invalidReq := validReq
		invalidReq.ImageURL = "not-a-url"
		_, err := adService.CreateAd(testCtx, invalidReq, user.ID)
		assert.ErrorIs(t, err, db.ErrInvalidImageURL)
	})

	t.Run("invalid price returns error", func(t *testing.T) {
		invalidReq := validReq
		invalidReq.Price = 0
		_, err := adService.CreateAd(testCtx, invalidReq, user.ID)
		assert.ErrorIs(t, err, db.ErrInvalidPrice)
	})
}

func TestGetAds(t *testing.T) {
	adService := NewAdService(testDB)

	err := clearTables(testCtx, testDB)
	require.NoError(t, err)

	user1, err := testDB.CreateUser(testCtx, "user1", "pass1")
	require.NoError(t, err)
	user2, err := testDB.CreateUser(testCtx, "user2", "pass2")
	require.NoError(t, err)

	ad1 := db.Ad{
		Title:    "Ad1",
		Text:     "Text 1",
		ImageURL: "https://example.com/ad1.png",
		Price:    1000,
		UserID:   user1.ID,
	}
	ad2 := db.Ad{
		Title:    "Ad2",
		Text:     "Text 2",
		ImageURL: "https://example.com/ad2.png",
		Price:    2000,
		UserID:   user2.ID,
	}

	_, err = testDB.CreateAd(testCtx, ad1)
	require.NoError(t, err)
	_, err = testDB.CreateAd(testCtx, ad2)
	require.NoError(t, err)

	t.Run("get all ads with default sorting", func(t *testing.T) {
		req := GetAdsRequest{
			Page:      1,
			PageSize:  10,
			SortBy:    "", // должно использоваться created_at
			SortOrder: "", // должно использоваться DESC
		}
		ads, err := adService.GetAds(testCtx, req, user1.ID)
		require.NoError(t, err)
		assert.Len(t, ads, 2)
		assert.True(t, ads[0].CreatedAt.After(ads[1].CreatedAt)) // DESC порядок
		assert.True(t, ads[0].IsMine || ads[1].IsMine)
	})

	t.Run("get ads sorted by price ascending", func(t *testing.T) {
		req := GetAdsRequest{
			Page:      1,
			PageSize:  10,
			SortBy:    "price",
			SortOrder: "ASC",
		}
		ads, err := adService.GetAds(testCtx, req, user1.ID)
		require.NoError(t, err)
		assert.Len(t, ads, 2)
		assert.Equal(t, ad1.Title, ads[0].Title)
		assert.Equal(t, ad2.Title, ads[1].Title)
	})

	t.Run("filter ads by price range", func(t *testing.T) {
		req := GetAdsRequest{
			Page:      1,
			PageSize:  10,
			SortBy:    "price",
			SortOrder: "ASC",
			MinPrice:  1500,
			MaxPrice:  2500,
		}
		ads, err := adService.GetAds(testCtx, req, user1.ID)
		require.NoError(t, err)
		assert.Len(t, ads, 1)
		assert.Equal(t, ad2.Title, ads[0].Title)
	})

	t.Run("pagination works correctly", func(t *testing.T) {
		req := GetAdsRequest{
			Page:      2,
			PageSize:  1,
			SortBy:    "price",
			SortOrder: "ASC",
		}
		ads, err := adService.GetAds(testCtx, req, user1.ID)
		require.NoError(t, err)
		assert.Len(t, ads, 1)
		assert.Equal(t, ad2.Title, ads[0].Title)
	})

	t.Run("invalid sort by returns error", func(t *testing.T) {
		req := GetAdsRequest{
			Page:      1,
			PageSize:  10,
			SortBy:    "invalid_field",
			SortOrder: "ASC",
		}
		_, err := adService.GetAds(testCtx, req, user1.ID)
		assert.ErrorIs(t, err, db.ErrInvalidSortBy)
	})

	t.Run("invalid sort order returns error", func(t *testing.T) {
		req := GetAdsRequest{
			Page:      1,
			PageSize:  10,
			SortBy:    "price",
			SortOrder: "INVALID",
		}
		_, err := adService.GetAds(testCtx, req, user1.ID)
		assert.ErrorIs(t, err, db.ErrInvalidSortOrder)
	})
}

// clearTables очищает таблицы users и ads
func clearTables(ctx context.Context, db *db.DBService) error {
	return db.Exec(ctx, "TRUNCATE TABLE ads, users CASCADE")
}
