package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type ctxKey string

const UserKey ctxKey = "usuario"

type UsuarioCtx struct {
	ID     int
	Rol    string
	Nombre string
}

func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			http.Error(w, `{"error":"no autorizado"}`, http.StatusUnauthorized)
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		}, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil || !token.Valid {
			http.Error(w, `{"error":"token inválido"}`, http.StatusUnauthorized)
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, `{"error":"token inválido"}`, http.StatusUnauthorized)
			return
		}
		u := UsuarioCtx{
			ID:     int(claims["user_id"].(float64)),
			Rol:    claims["rol"].(string),
			Nombre: claims["nombre"].(string),
		}
		ctx := context.WithValue(r.Context(), UserKey, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func SoloAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := r.Context().Value(UserKey).(UsuarioCtx)
		if u.Rol != "admin" {
			http.Error(w, `{"error":"solo administrador"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func GetUser(r *http.Request) UsuarioCtx {
	return r.Context().Value(UserKey).(UsuarioCtx)
}
