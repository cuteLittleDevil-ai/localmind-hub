package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware(t *testing.T) {
	const token = "localmind-demo-token"
	mw := AuthMiddleware(token)
	called := false
	next := mw(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	t.Run("health bypass", func(t *testing.T) {
		called = false
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		rec := httptest.NewRecorder()
		next(rec, req)
		if rec.Code != http.StatusOK || !called {
			t.Fatalf("expected health bypass, code=%d called=%v", rec.Code, called)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		called = false
		req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
		rec := httptest.NewRecorder()
		next(rec, req)
		if rec.Code != http.StatusUnauthorized || called {
			t.Fatalf("expected 401, code=%d called=%v", rec.Code, called)
		}
	})

	t.Run("valid token", func(t *testing.T) {
		called = false
		req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		next(rec, req)
		if rec.Code != http.StatusOK || !called {
			t.Fatalf("expected ok, code=%d called=%v", rec.Code, called)
		}
	})
}
