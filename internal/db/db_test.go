package db

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	testDB     *DBService
	postgresC  *postgres.PostgresContainer
	testCtx    context.Context
	cancelFunc context.CancelFunc
)

func TestMain(m *testing.M) {
	testCtx, cancelFunc = context.WithCancel(context.Background())
	defer cancelFunc()

	var err error
	postgresC, err = postgres.Run(testCtx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithHostPortAccess(5432),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(10*time.Second),
		),
	)
	if err != nil {
		fmt.Printf("Failed to start PostgreSQL container: %v\n", err)
		os.Exit(1)
	}

	dsn, err := postgresC.ConnectionString(testCtx, "sslmode=disable")
	if err != nil {
		fmt.Printf("Failed to get connection string: %v\n", err)
		_ = postgresC.Terminate(testCtx)
		os.Exit(1)
	}

	testDB, err = NewDBService(testCtx, dsn)
	if err != nil {
		fmt.Printf("Failed to create DBService: %v\n", err)
		_ = postgresC.Terminate(testCtx)
		os.Exit(1)
	}

	exitCode := m.Run()
	os.Exit(exitCode)
}

func TestNewDBService(t *testing.T) {
	t.Run("valid DSN", func(t *testing.T) {
		assert.NotNil(t, testDB)
	})

	t.Run("invalid DSN returns error", func(t *testing.T) {
		_, err := NewDBService(testCtx, "invalid_dsn")
		assert.Error(t, err)
	})
}

func TestCreateUser(t *testing.T) {
	t.Run("create user successfully", func(t *testing.T) {
		user, err := testDB.CreateUser(testCtx, "testuser1", "hashedpass")
		require.NoError(t, err)
		assert.Equal(t, "testuser1", user.Login)
		assert.NotZero(t, user.ID)
		assert.False(t, user.CreatedAt.IsZero())
	})

	t.Run("duplicate login returns error", func(t *testing.T) {
		_, err := testDB.CreateUser(testCtx, "dupuser", "pass1")
		require.NoError(t, err)

		_, err = testDB.CreateUser(testCtx, "dupuser", "pass2")
		assert.Error(t, err)
	})
}

func TestUserByLogin(t *testing.T) {
	login := "userbylogin"
	createdUser, err := testDB.CreateUser(testCtx, login, "pass")
	require.NoError(t, err)

	t.Run("get existing user", func(t *testing.T) {
		user, err := testDB.UserByLogin(testCtx, login)
		require.NoError(t, err)
		assert.Equal(t, createdUser.ID, user.ID)
		assert.Equal(t, login, user.Login)
		assert.False(t, user.CreatedAt.IsZero())
	})

	t.Run("get non-existing user returns error", func(t *testing.T) {
		_, err := testDB.UserByLogin(testCtx, "nonexistent")
		assert.Error(t, err)
	})
}

func TestCreateAd(t *testing.T) {
	user, err := testDB.CreateUser(testCtx, "aduser", "pass")
	require.NoError(t, err)

	validAd := Ad{
		Title:    "Valid Title",
		Text:     "Valid text content for ad",
		ImageURL: "https://example.com/image.png",
		Price:    1000,
		UserID:   user.ID,
	}

	t.Run("create valid ad", func(t *testing.T) {
		createdAd, err := testDB.CreateAd(testCtx, validAd)
		require.NoError(t, err)
		assert.Equal(t, validAd.Title, createdAd.Title)
		assert.Equal(t, validAd.Text, createdAd.Text)
		assert.Equal(t, validAd.ImageURL, createdAd.ImageURL)
		assert.Equal(t, validAd.Price, createdAd.Price)
		assert.Equal(t, validAd.UserID, createdAd.UserID)
		assert.Equal(t, "aduser", createdAd.Author)
		assert.True(t, createdAd.IsMine)
		assert.False(t, createdAd.CreatedAt.IsZero())
	})

	t.Run("ad creation with invalid user returns error", func(t *testing.T) {
		invalidAd := validAd
		invalidAd.UserID = 999999
		_, err := testDB.CreateAd(testCtx, invalidAd)
		assert.ErrorIs(t, err, ErrUserNotFound)
	})

	t.Run("ad creation with invalid title returns error", func(t *testing.T) {
		invalidAd := validAd
		invalidAd.Title = "A"
		_, err := testDB.CreateAd(testCtx, invalidAd)
		assert.ErrorIs(t, err, ErrInvalidTitleLength)
	})

	t.Run("ad creation with invalid text returns error", func(t *testing.T) {
		invalidAd := validAd
		invalidAd.Text = ""
		_, err := testDB.CreateAd(testCtx, invalidAd)
		assert.ErrorIs(t, err, ErrInvalidTextLength)
	})

	t.Run("ad creation with invalid image URL returns error", func(t *testing.T) {
		invalidAd := validAd
		invalidAd.ImageURL = "not-a-url"
		_, err := testDB.CreateAd(testCtx, invalidAd)
		assert.ErrorIs(t, err, ErrInvalidImageURL)
	})

	t.Run("ad creation with invalid price returns error", func(t *testing.T) {
		invalidAd := validAd
		invalidAd.Price = 0
		_, err := testDB.CreateAd(testCtx, invalidAd)
		assert.ErrorIs(t, err, ErrInvalidPrice)
	})

	t.Run("ad creation with invalid UserID returns error", func(t *testing.T) {
		invalidAd := validAd
		invalidAd.UserID = 0
		_, err := testDB.CreateAd(testCtx, invalidAd)
		assert.ErrorIs(t, err, ErrInvalidUserID)
	})
}

func clearTables(ctx context.Context, db *DBService) error {
	// TRUNCATE users, ads CASCADE чтобы очистить все объявления и пользователей
	_, err := db.pool.Exec(ctx, "TRUNCATE TABLE ads, users CASCADE")
	return err
}

// TestAds tests retrieval of ads with filtering, sorting and pagination.
func TestAds(t *testing.T) {

	err := clearTables(testCtx, testDB)
	require.NoError(t, err)

	user1, err := testDB.CreateUser(testCtx, "user1", "pass1")
	require.NoError(t, err)

	user2, err := testDB.CreateUser(testCtx, "user2", "pass2")
	require.NoError(t, err)

	ad1 := Ad{
		Title:  "Ad1",
		Text:   "Text 1",
		Price:  1000,
		UserID: user1.ID,
	}

	ad2 := Ad{
		Title:  "Ad2",
		Text:   "Text 2",
		Price:  2000,
		UserID: user2.ID,
	}

	_, err = testDB.CreateAd(testCtx, ad1)
	require.NoError(t, err)

	_, err = testDB.CreateAd(testCtx, ad2)
	require.NoError(t, err)

	t.Run("retrieve all ads sorted by price ascending", func(t *testing.T) {
		ads, err := testDB.Ads(testCtx, user1.ID, 1, 10, "price", "ASC", 0, 10000)
		require.NoError(t, err)
		assert.Len(t, ads, 2)
		assert.Equal(t, ad1.Title, ads[0].Title)
		assert.Equal(t, ad2.Title, ads[1].Title)
		assert.True(t, ads[0].IsMine)
		assert.False(t, ads[1].IsMine)
	})

	t.Run("filter ads by price range", func(t *testing.T) {
		ads, err := testDB.Ads(testCtx, user1.ID, 1, 10, "price", "ASC", 1500, 2500)
		require.NoError(t, err)
		assert.Len(t, ads, 1)
		assert.Equal(t, ad2.Title, ads[0].Title)
	})

	t.Run("pagination works correctly", func(t *testing.T) {
		ads, err := testDB.Ads(testCtx, user1.ID, 2, 1, "price", "ASC", 0, 10000)
		require.NoError(t, err)
		assert.Len(t, ads, 1)
		assert.Equal(t, ad2.Title, ads[0].Title)
	})
}

func TestDBOptions(t *testing.T) {
	ctx := context.Background()

	dsn, err := postgresC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	t.Run("custom MaxConns and MinConns applied", func(t *testing.T) {
		db, err := NewDBService(
			ctx,
			dsn,
			WithMaxConns(25),
			WithMinConns(10),
			WithConnMaxLifetime(30*time.Minute),
			WithConnIdleLifetime(5*time.Minute),
		)
		require.NoError(t, err)
		defer db.pool.Close()

		stats := db.pool.Stat()
		assert.Equal(t, int32(25), db.pool.Config().MaxConns)
		assert.Equal(t, int32(10), db.pool.Config().MinConns)
		assert.Equal(t, 30*time.Minute, db.pool.Config().MaxConnLifetime)
		assert.Equal(t, 5*time.Minute, db.pool.Config().MaxConnIdleTime)
		assert.Equal(t, stats.MaxConns(), int32(25))
	})

	t.Run("zero MaxConns returns error", func(t *testing.T) {
		_, err := NewDBService(ctx, dsn, WithMaxConns(0))
		assert.Error(t, err)
	})

	t.Run("invalid DSN with options still returns error", func(t *testing.T) {
		_, err := NewDBService(ctx, "invalid_dsn", WithMaxConns(10))
		assert.Error(t, err)
	})
}
