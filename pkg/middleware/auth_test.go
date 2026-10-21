package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gridsim/internal/model"
)

func TestAdminMiddlewareRequiresAdminRole(t *testing.T) {
	adminToken, err := GenerateToken(&model.User{ID: "1", Username: "admin", Role: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	operatorToken, err := GenerateToken(&model.User{ID: "2", Username: "operator", Role: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	handler := AdminMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for name, tokenStatus := range map[string]struct {
		token string
		want  int
	}{
		"admin":     {adminToken, http.StatusNoContent},
		"operator":  {operatorToken, http.StatusForbidden},
		"missing":   {"", http.StatusUnauthorized},
		"malformed": {"not-a-jwt", http.StatusUnauthorized},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/config/clear", nil)
			if tokenStatus.token != "" {
				req.Header.Set("Authorization", "Bearer "+tokenStatus.token)
			}
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)
			if resp.Code != tokenStatus.want {
				t.Fatalf("status=%d, want %d", resp.Code, tokenStatus.want)
			}
		})
	}
}

func TestAuthenticateRequestRejectsExpiredToken(t *testing.T) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": "1", "username": "admin", "role": "admin",
		"exp": time.Now().Add(-time.Minute).Unix(),
	})
	raw, err := token.SignedString([]byte(authConfig.Secret))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	if _, err := authenticateRequest(req); err == nil {
		t.Fatal("expired token was accepted")
	}
}

func TestAuthenticateRequestRejectsNonHMACToken(t *testing.T) {
	token := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"user_id": "1", "username": "admin", "role": "admin",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	raw, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	if _, err := authenticateRequest(req); err == nil {
		t.Fatal("none algorithm token was accepted")
	}
}
