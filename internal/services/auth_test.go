package services

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/YuarenArt/marketgo/internal/db"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/bcrypt"
)

var (
	testDB     *db.DBService
	postgresC  *postgres.PostgresContainer
	testCtx    context.Context
	cancelFunc context.CancelFunc
	secret     = "test-secret-key"
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

	testDB, err = db.NewDBService(testCtx, dsn)
	if err != nil {
		fmt.Printf("Failed to create DBService: %v\n", err)
		_ = postgresC.Terminate(testCtx)
		os.Exit(1)
	}

	exitCode := m.Run()
	_ = postgresC.Terminate(testCtx)
	os.Exit(exitCode)
}

func TestNewAuthService(t *testing.T) {
	t.Run("create auth service successfully", func(t *testing.T) {
		authService := NewAuthService(testDB, secret)
		assert.NotNil(t, authService)
		assert.Equal(t, testDB, authService.db)
		assert.Equal(t, secret, authService.secret)
	})
}

func TestRegister(t *testing.T) {
	authService := NewAuthService(testDB, secret)

	t.Run("register user successfully", func(t *testing.T) {
		input := InputUserInfo{
			Login:    "testuser",
			Password: "password123",
		}
		user, err := authService.Register(testCtx, input)
		require.NoError(t, err)
		assert.Equal(t, "testuser", user.Login)
		assert.NotZero(t, user.ID)
		assert.False(t, user.CreatedAt.IsZero())

		// Проверяем, что пароль захеширован
		storedUser, err := testDB.UserByLogin(testCtx, "testuser")
		require.NoError(t, err)
		err = bcrypt.CompareHashAndPassword([]byte(storedUser.Password), []byte("password123"))
		assert.NoError(t, err)
	})

	t.Run("duplicate login returns error", func(t *testing.T) {
		input := InputUserInfo{
			Login:    "dupuser",
			Password: "password123",
		}
		_, err := authService.Register(testCtx, input)
		require.NoError(t, err)

		_, err = authService.Register(testCtx, input)
		assert.Error(t, err)
	})

	t.Run("invalid password returns error", func(t *testing.T) {
		input := InputUserInfo{
			Login:    "invalidpass",
			Password: "short", // слишком короткий пароль
		}
		_, err := authService.Register(testCtx, input)
		assert.Error(t, err) // bcrypt вернет ошибку при слишком коротком пароле
	})
}

func TestAuthenticate(t *testing.T) {
	authService := NewAuthService(testDB, secret)

	// Подготовка: регистрируем пользователя
	input := InputUserInfo{
		Login:    "authuser",
		Password: "password123",
	}
	_, err := authService.Register(testCtx, input)
	require.NoError(t, err)

	t.Run("authenticate user successfully", func(t *testing.T) {
		token, err := authService.Authenticate(testCtx, input)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Проверяем валидность токена
		parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})
		require.NoError(t, err)
		assert.True(t, parsedToken.Valid)

		claims, ok := parsedToken.Claims.(jwt.MapClaims)
		require.True(t, ok)
		assert.Equal(t, "auth-services", claims["iss"])
		assert.Equal(t, "marketgo-api", claims["aud"])
		assert.Greater(t, claims["exp"], claims["iat"])
	})

	t.Run("invalid login returns error", func(t *testing.T) {
		input := InputUserInfo{
			Login:    "nonexistent",
			Password: "password123",
		}
		_, err := authService.Authenticate(testCtx, input)
		assert.Error(t, err)
	})

	t.Run("wrong password returns error", func(t *testing.T) {
		input := InputUserInfo{
			Login:    "authuser",
			Password: "wrongpassword",
		}
		_, err := authService.Authenticate(testCtx, input)
		assert.Error(t, err)
	})
}

func TestValidateToken(t *testing.T) {
	authService := NewAuthService(testDB, secret)

	// Подготовка: регистрируем пользователя и получаем токен
	input := InputUserInfo{
		Login:    "tokenuser",
		Password: "password123",
	}
	user, err := authService.Register(testCtx, input)
	require.NoError(t, err)
	token, err := authService.Authenticate(testCtx, input)
	require.NoError(t, err)

	t.Run("validate token successfully", func(t *testing.T) {
		userID, err := authService.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, user.ID, userID)
	})

	t.Run("invalid token returns error", func(t *testing.T) {
		_, err := authService.ValidateToken("invalid.token.string")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token is malformed")
	})

	t.Run("expired token returns error", func(t *testing.T) {
		now := time.Now()
		claims := jwt.MapClaims{
			"user_id": float64(user.ID),
			"iat":     now.Add(-48 * time.Hour).Unix(),
			"nbf":     now.Add(-48 * time.Hour).Unix(),
			"exp":     now.Add(-24 * time.Hour).Unix(),
			"iss":     "auth-services",
			"aud":     "marketgo-api",
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(secret))
		require.NoError(t, err)

		_, err = authService.ValidateToken(tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token is expired")
	})

	t.Run("invalid issuer returns error", func(t *testing.T) {
		claims := jwt.MapClaims{
			"user_id": float64(user.ID),
			"iat":     time.Now().Unix(),
			"nbf":     time.Now().Unix(),
			"exp":     time.Now().Add(24 * time.Hour).Unix(),
			"iss":     "wrong-issuer",
			"aud":     "marketgo-api",
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(secret))
		require.NoError(t, err)

		_, err = authService.ValidateToken(tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid issuer")
	})

	t.Run("invalid user_id returns error", func(t *testing.T) {
		claims := jwt.MapClaims{
			"user_id": -1.0, // отрицательный user_id
			"iat":     time.Now().Unix(),
			"nbf":     time.Now().Unix(),
			"exp":     time.Now().Add(24 * time.Hour).Unix(),
			"iss":     "auth-services",
			"aud":     "marketgo-api",
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(secret))
		require.NoError(t, err)

		_, err = authService.ValidateToken(tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid user_id claim")
	})
}
