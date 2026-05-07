package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"vault/internal/db"
	"vault/internal/handlers"
	"vault/internal/middleware"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	godotenv.Load()

	if secret := os.Getenv("JWT_SECRET"); secret == "" || secret == "vault-secret-change-in-production" {
		log.Println("⚠️  ADVERTENCIA: JWT_SECRET no configurado o inseguro — cambia el valor en .env")
	}

	if err := db.Connect(); err != nil {
		log.Fatal("DB:", err)
	}
	defer db.Pool.Close()

	runMigrations()
	seedAdmin()

	r := chi.NewRouter()
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)

	// Archivos estáticos
	r.Handle("/*", http.FileServer(http.Dir("./static")))

	// API
	r.Route("/api", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				next.ServeHTTP(w, r)
			})
		})
		r.With(middleware.RateLimit(10, time.Minute)).Post("/auth/login", handlers.Login)

		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth)

			r.Get("/auth/me", handlers.Me)
			r.Patch("/auth/password", handlers.CambiarPassword)
			r.Get("/dashboard", handlers.GetDashboard)
			r.Get("/export/csv", handlers.ExportarCSV)

			// Préstamos y clientes
			r.Get("/prestamos", handlers.ListarPrestamos)
			r.Get("/prestamos/{id}", handlers.ObtenerPrestamo)
			r.With(middleware.SoloAdmin).Post("/prestamos", handlers.CrearPrestamo)
			r.With(middleware.SoloAdmin).Patch("/prestamos/{id}/datos", handlers.EditarPrestamo)
			r.With(middleware.SoloAdmin).Patch("/prestamos/{id}/estado", handlers.CambiarEstado)
			r.With(middleware.SoloAdmin).Patch("/prestamos/{id}/vencimiento", handlers.ProrrogarPrestamo)
			r.With(middleware.SoloAdmin).Post("/prestamos/{id}/renovar", handlers.RenovarPrestamo)
			r.Get("/clientes", handlers.ListarClientes)
			r.With(middleware.SoloAdmin).Post("/clientes", handlers.CrearCliente)
			r.With(middleware.SoloAdmin).Patch("/clientes/{id}", handlers.EditarCliente)

			// Cobros
			r.Post("/cobros", handlers.RegistrarCobro)
			r.Get("/cobros", handlers.ListarCobros)
			r.Get("/cobros/ganancias", handlers.GetGanancias)
			r.With(middleware.SoloAdmin).Delete("/cobros/{id}", handlers.EliminarCobro)

			// Caja
			r.Get("/caja", handlers.ListarCaja)
			r.With(middleware.SoloAdmin).Post("/caja/aporte", handlers.RegistrarAporte)
			r.With(middleware.SoloAdmin).Post("/caja/retiro", handlers.RegistrarRetiro)

			// Pagos socios
			r.Get("/pagos-socios", handlers.GetResumenSocios)
			r.With(middleware.SoloAdmin).Post("/pagos-socios", handlers.RegistrarPagoSocio)
			r.With(middleware.SoloAdmin).Delete("/pagos-socios/{id}", handlers.EliminarPagoSocio)
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "4000"
	}
	fmt.Printf("✅ Vault corriendo en http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func runMigrations() {
	entries, err := os.ReadDir("migrations")
	if err != nil {
		log.Fatal("Error leyendo directorio de migraciones:", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".sql" {
			continue
		}
		sql, err := os.ReadFile(filepath.Join("migrations", e.Name()))
		if err != nil {
			log.Fatalf("Error leyendo %s: %v", e.Name(), err)
		}
		if _, err := db.Pool.Exec(context.Background(), string(sql)); err != nil {
			log.Fatalf("Error ejecutando %s: %v", e.Name(), err)
		}
		log.Printf("✅ Migración aplicada: %s", e.Name())
	}
}

func seedAdmin() {
	var count int
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM usuarios`).Scan(&count)
	if count > 0 {
		return
	}
	adminPass := os.Getenv("ADMIN_PASSWORD")
	if adminPass == "" {
		adminPass = "admin123"
		log.Println("⚠️  ADMIN_PASSWORD no configurado, usando contraseña por defecto — cámbiala")
	}
	socioPass := os.Getenv("SOCIO_PASSWORD")
	if socioPass == "" {
		socioPass = "socio123"
		log.Println("⚠️  SOCIO_PASSWORD no configurado, usando contraseña por defecto — cámbiala")
	}
	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "admin@vault.com"
	}
	socioEmail := os.Getenv("SOCIO_EMAIL")
	if socioEmail == "" {
		socioEmail = "socio@vault.com"
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(adminPass), 12)
	db.Pool.Exec(context.Background(),
		`INSERT INTO usuarios (nombre, email, password, rol) VALUES ($1,$2,$3,$4)`,
		"Admin", adminEmail, string(hash), "admin")
	hash2, _ := bcrypt.GenerateFromPassword([]byte(socioPass), 12)
	db.Pool.Exec(context.Background(),
		`INSERT INTO usuarios (nombre, email, password, rol) VALUES ($1,$2,$3,$4)`,
		"Socio", socioEmail, string(hash2), "socio")
	log.Println("✅ Usuarios iniciales creados")
}
