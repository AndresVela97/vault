package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"vault/internal/db"
	"vault/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" || body.Password == "" {
		jsonError(w, "email y password requeridos", http.StatusBadRequest)
		return
	}

	var id int
	var nombre, rol, hash string
	err := db.Pool.QueryRow(context.Background(),
		`SELECT id, nombre, rol, password FROM usuarios WHERE email = $1`, body.Email,
	).Scan(&id, &nombre, &rol, &hash)
	if err != nil {
		jsonError(w, "credenciales inválidas", http.StatusUnauthorized)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		jsonError(w, "credenciales inválidas", http.StatusUnauthorized)
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": id,
		"rol":     rol,
		"nombre":  nombre,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	})
	signed, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		jsonError(w, "error generando token", http.StatusInternalServerError)
		return
	}

	jsonOK(w, map[string]any{
		"token":   signed,
		"usuario": map[string]any{"id": id, "nombre": nombre, "rol": rol},
	})
}

func Me(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r)
	jsonOK(w, map[string]any{"id": u.ID, "nombre": u.Nombre, "rol": u.Rol})
}

func CambiarPassword(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r)
	var body struct {
		Actual string `json:"actual"`
		Nueva  string `json:"nueva"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Actual == "" || body.Nueva == "" {
		jsonError(w, "contraseña actual y nueva requeridas", http.StatusBadRequest)
		return
	}
	if len(body.Nueva) < 6 {
		jsonError(w, "la nueva contraseña debe tener al menos 6 caracteres", http.StatusBadRequest)
		return
	}
	var hash string
	db.Pool.QueryRow(context.Background(), `SELECT password FROM usuarios WHERE id=$1`, u.ID).Scan(&hash)
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Actual)) != nil {
		jsonError(w, "contraseña actual incorrecta", http.StatusUnauthorized)
		return
	}
	nuevo, _ := bcrypt.GenerateFromPassword([]byte(body.Nueva), 12)
	db.Pool.Exec(context.Background(), `UPDATE usuarios SET password=$1 WHERE id=$2`, string(nuevo), u.ID)
	jsonOK(w, map[string]string{"mensaje": "Contraseña actualizada"})
}
