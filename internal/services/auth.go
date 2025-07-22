package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/YuarenArt/marketgo/internal/db"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	ErrPasswordLength    = "password must be between 8 and 72 characters"
	ErrInvalidToken      = "invalid token"
	ErrInvalidTokenClaim = "invalid token claims"
	ErrTokenExpired      = "token expired"
	ErrTokenNotYetValid  = "token not valid yet"
	ErrTokenIssuedFuture = "token issued in the future"
	ErrInvalidIssuer     = "invalid issuer"
	ErrInvalidAudience   = "invalid audience"
	ErrInvalidUserID     = "invalid user_id claim"
	Issuer               = "auth-services"
	Audience             = "marketgo-api"
)

// InputUserInfo представляет входные данные для регистрации и входа
type InputUserInfo struct {
	Login    string `json:"login" binding:"required,min=4,max=20"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

// AuthService отвечает за регистрацию, аутентификацию и валидацию JWT-токенов
type AuthService struct {
	db     *db.DBService
	secret string
}

// NewAuthService создает новый экземпляр AuthService
func NewAuthService(db *db.DBService, secret string) *AuthService {
	return &AuthService{db: db, secret: secret}
}

// Register регистрирует нового пользователя с хешированным паролем
func (s *AuthService) Register(ctx context.Context, input InputUserInfo) (db.User, error) {
	if len(input.Password) < 8 || len(input.Password) > 72 {
		return db.User{}, errors.New(ErrPasswordLength)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return db.User{}, err
	}
	return s.db.CreateUser(ctx, input.Login, string(hashedPassword))
}

// Authenticate проверяет логин и пароль, возвращает JWT-токен при успехе
func (s *AuthService) Authenticate(ctx context.Context, input InputUserInfo) (string, error) {
	user, err := s.db.UserByLogin(ctx, input.Login)
	if err != nil {
		return "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		return "", err
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"user_id": float64(user.ID),
		"iat":     now.Unix(),
		"nbf":     now.Unix(),
		"exp":     now.Add(24 * time.Hour).Unix(),
		"iss":     Issuer,
		"aud":     Audience,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secret))
}

// ValidateToken проверяет корректность JWT-токена и возвращает user_id
func (s *AuthService) ValidateToken(tokenString string) (int, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if err != nil {
		return 0, err
	}
	if !token.Valid {
		return 0, errors.New(ErrInvalidToken)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errors.New(ErrInvalidTokenClaim)
	}

	if err := validateRegisteredClaims(claims); err != nil {
		return 0, err
	}

	userID, ok := claims["user_id"].(float64)
	if !ok || userID <= 0 {
		return 0, errors.New(ErrInvalidUserID)
	}

	return int(userID), nil
}

// validateRegisteredClaims выполняет валидацию стандартных полей токена
func validateRegisteredClaims(claims jwt.MapClaims) error {
	now := time.Now().Unix()

	if exp, ok := claims["exp"].(float64); !ok || int64(exp) < now {
		return errors.New(ErrTokenExpired)
	}
	if nbf, ok := claims["nbf"].(float64); ok && int64(nbf) > now {
		return errors.New(ErrTokenNotYetValid)
	}
	if iat, ok := claims["iat"].(float64); ok && int64(iat) > now+60 {
		return errors.New(ErrTokenIssuedFuture)
	}
	if iss, ok := claims["iss"].(string); !ok || iss != Issuer {
		return errors.New(ErrInvalidIssuer)
	}
	if aud, ok := claims["aud"].(string); !ok || aud != Audience {
		return errors.New(ErrInvalidAudience)
	}
	return nil
}
