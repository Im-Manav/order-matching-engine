package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Im-Manav/ome/internal/config"
	"github.com/Im-Manav/ome/internal/ports"
	"github.com/Im-Manav/ome/pkg/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo ports.UserRepository
	cache    ports.Cache
	cfg      *config.Config
}

func NewAuthService(
	userRepo ports.UserRepository,
	cache ports.Cache,
	cfg *config.Config,
) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		cache:    cache,
		cfg:      cfg,
	}
}

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	JTI    string `json:"jti"`
	jwt.RegisteredClaims
}

func (s *AuthService) Register(
	ctx context.Context,
	req models.RegisterRequest,
) (*models.AuthResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &models.User{
		ID:           uuid.New(),
		Email:        req.Email,
		PasswordHash: string(hash),
	}

	if err := s.userRepo.CreateUser(user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	token, err := s.generateToken(user)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{Token: token, User: *user}, nil
}

func (s *AuthService) Login(
	ctx context.Context,
	req models.LoginRequest,
) (*models.AuthResponse, error) {
	user, err := s.userRepo.GetUserByEmail(req.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(req.Password),
	); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	token, err := s.generateToken(user)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{Token: token, User: *user}, nil
}

func (s *AuthService) Logout(ctx context.Context, tokenString string) error {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return err
	}

	remaining := time.Until(claims.ExpiresAt.Time)
	if remaining <= 0 {
		return nil
	}

	return s.cache.SetWithExpiry(ctx, claims.JTI, "blocked", remaining)
}

func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method:%v", t.Header["alg"])
			}
			return []byte(s.cfg.JWTSecret), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("invalid token:%w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

func (s *AuthService) IsBlocklisted(ctx context.Context, jti string) (bool, error) {
	val, err := s.cache.Get(ctx, jti)
	if err != nil {
		return false, err
	}
	return val == "blocked", nil
}

func (s *AuthService) generateToken(user *models.User) (string, error) {
	jti := uuid.New().String()

	claims := &Claims{
		UserID: user.ID.String(),
		Email:  user.Email,
		JTI:    jti,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(
				time.Now().Add(time.Duration(s.cfg.JWTExpiryHours) * time.Hour),
			),
			IssuedAt: jwt.NewNumericDate(time.Now()),
			Issuer:   "ome",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}
