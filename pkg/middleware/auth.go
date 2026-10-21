package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gridsim/internal/model"
)

type AuthConfig struct {
	Secret     string
	TokenExpiry time.Duration
}

var (
	authConfig = &AuthConfig{
		Secret:     "iec104-sim-secret-key-2026",
		TokenExpiry: 24 * time.Hour,
	}
)

type contextKey string

const (
	UserIDKey   contextKey = "user_id"
	UsernameKey contextKey = "username"
	RoleKey     contextKey = "role"
)

func authenticateRequest(r *http.Request) (context.Context, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing authorization header")
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" || strings.TrimSpace(parts[1]) == "" {
		return nil, fmt.Errorf("invalid authorization header")
	}

	token, err := jwt.Parse(parts[1], func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %s", token.Method.Alg())
		}
		return []byte(authConfig.Secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	ctx := context.WithValue(r.Context(), UserIDKey, claims["user_id"])
	ctx = context.WithValue(ctx, UsernameKey, claims["username"])
	ctx = context.WithValue(ctx, RoleKey, claims["role"])
	return ctx, nil
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/api/v1/auth/login" || path == "/api/v1/status" || path == "/" || strings.HasPrefix(path, "/assets") {
			next.ServeHTTP(w, r)
			return
		}
		ctx, err := authenticateRequest(r)
		if err != nil {
			slog.Warn("invalid token", "error", err)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AdminMiddleware protects destructive administration operations. It is mounted
// only on /api/v1/admin/ so existing public and operator APIs remain compatible.
func AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, err := authenticateRequest(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		role, _ := ctx.Value(RoleKey).(string)
		if role != "admin" {
			http.Error(w, "admin role required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GenerateToken(user *model.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(authConfig.TokenExpiry).Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(authConfig.Secret))
}