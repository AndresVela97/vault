package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"vault/internal/db"
	"vault/internal/models"

	"github.com/go-chi/chi/v5"
)

func GetResumenSocios(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	var totalA, totalB int64
	db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(ganancia_a),0), COALESCE(SUM(ganancia_b),0) FROM cobros`).Scan(&totalA, &totalB)

	var pagadoA, pagadoB int64
	db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(monto),0) FROM pagos_socios WHERE socio='a'`).Scan(&pagadoA)
	db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(monto),0) FROM pagos_socios WHERE socio='b'`).Scan(&pagadoB)

	rows, _ := db.Pool.Query(ctx, `SELECT id, fecha::text, socio, monto, descripcion FROM pagos_socios ORDER BY fecha DESC, id DESC`)
	defer rows.Close()
	pagos := []models.PagoSocio{}
	for rows.Next() {
		var p models.PagoSocio
		rows.Scan(&p.ID, &p.Fecha, &p.Socio, &p.Monto, &p.Descripcion)
		pagos = append(pagos, p)
	}

	jsonOK(w, map[string]any{
		"total_a":   totalA,
		"total_b":   totalB,
		"pagado_a":  pagadoA,
		"pagado_b":  pagadoB,
		"pendiente_a": totalA - pagadoA,
		"pendiente_b": totalB - pagadoB,
		"pagos":     pagos,
	})
}

func RegistrarPagoSocio(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Socio       string  `json:"socio"`
		Monto       int64   `json:"monto"`
		Fecha       string  `json:"fecha"`
		Descripcion *string `json:"descripcion"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "datos inválidos", http.StatusBadRequest)
		return
	}
	if body.Socio != "a" && body.Socio != "b" {
		jsonError(w, "socio debe ser 'a' o 'b'", http.StatusBadRequest)
		return
	}
	if body.Monto <= 0 {
		jsonError(w, "monto inválido", http.StatusBadRequest)
		return
	}
	if body.Fecha == "" {
		jsonError(w, "fecha requerida", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	tx, _ := db.Pool.Begin(ctx)
	defer tx.Rollback(ctx)

	var saldo int64
	tx.QueryRow(ctx, `SELECT COALESCE(saldo,0) FROM caja ORDER BY fecha DESC, id DESC LIMIT 1`).Scan(&saldo)
	if body.Monto > saldo {
		jsonError(w, "saldo insuficiente en caja", http.StatusBadRequest)
		return
	}

	nombre := map[string]string{"a": "Edwin", "b": "Stiven"}[body.Socio]
	desc := "Pago ganancia " + nombre
	if body.Descripcion != nil && *body.Descripcion != "" {
		desc = *body.Descripcion
	}

	var cajaID int
	tx.QueryRow(ctx, `
		INSERT INTO caja (fecha, tipo, descripcion, entrada, salida, saldo)
		VALUES ($1,'retiro',$2,0,$3,$4) RETURNING id`,
		body.Fecha, desc, body.Monto, saldo-body.Monto,
	).Scan(&cajaID)

	var id int
	tx.QueryRow(ctx, `
		INSERT INTO pagos_socios (fecha, socio, monto, descripcion, caja_id)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		body.Fecha, body.Socio, body.Monto, desc, cajaID,
	).Scan(&id)

	if err := tx.Commit(ctx); err != nil {
		jsonError(w, "error guardando pago", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, map[string]any{"id": id, "mensaje": "Pago registrado a " + nombre})
}

func EliminarPagoSocio(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := context.Background()
	tx, _ := db.Pool.Begin(ctx)
	defer tx.Rollback(ctx)

	var cajaID *int
	err := tx.QueryRow(ctx, `SELECT caja_id FROM pagos_socios WHERE id=$1`, id).Scan(&cajaID)
	if err != nil {
		jsonError(w, "pago no encontrado", http.StatusNotFound)
		return
	}
	if cajaID != nil {
		var entrada, salida int64
		tx.QueryRow(ctx, `SELECT entrada, salida FROM caja WHERE id=$1`, *cajaID).Scan(&entrada, &salida)
		tx.Exec(ctx, `DELETE FROM caja WHERE id=$1`, *cajaID)
		delta := salida - entrada
		tx.Exec(ctx, `UPDATE caja SET saldo = saldo + $1 WHERE id > $2`, delta, *cajaID)
	}
	tx.Exec(ctx, `DELETE FROM pagos_socios WHERE id=$1`, id)
	if err := tx.Commit(ctx); err != nil {
		jsonError(w, "error eliminando pago", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"mensaje": "Pago eliminado"})
}
