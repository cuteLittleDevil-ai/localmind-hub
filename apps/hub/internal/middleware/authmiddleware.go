package middleware

import (
	"net/http"
	"strings"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// AuthMiddleware validates Bearer demo token; health path is exempt.
func AuthMiddleware(demoToken string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/health" {
				next(w, r)
				return
			}

			auth := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if !strings.HasPrefix(auth, prefix) {
				httpx.WriteJson(w, http.StatusUnauthorized, map[string]string{
					"message": "missing or invalid authorization",
				})
				return
			}

			token := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
			if token == "" || token != demoToken {
				httpx.WriteJson(w, http.StatusUnauthorized, map[string]string{
					"message": "missing or invalid authorization",
				})
				return
			}

			next(w, r)
		}
	}
}
