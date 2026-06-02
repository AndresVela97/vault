package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"vault/internal/db"
	"vault/internal/models"
)

func ListarCaja(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 50)
	if limit > 200 {
		limit = 200
	}
	page := queryInt(r, "page", 1)
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	where := "WHERE 1=1"
	args := []any{}
	i := 1
	if v := queryStr(r, "tipo"); v != "" {
		where += " AND tipo = $" + strconv.Itoa(i)
		args = append(args, v)
		i++
	}
	if v := queryStr(r, "desde"); v != "" {
		where += " AND fecha >= $" + strconv.Itoa(i)
		args = append(args, v)
		i++
	}
	if v := queryStr(r, "hasta"); v != "" {
		where += " AND fecha <= $" + strconv.Itoa(i)
		args = append(args, v)
		i++
	}

	var total int
	db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM caja `+where, args...).Scan(&total)

	var saldo int64
	db.Pool.QueryRow(context.Background(), `SELECT COALESCE(saldo,0) FROM caja ORDER BY id DESC LIMIT 1`).Scan(&saldo)

	orden := "DESC"
	if queryStr(r, "orden") == "asc" {
		orden = "ASC"
	}
	rows, err := db.Pool.Query(context.Background(),
		`SELECT id, fecha::text, tipo, descripcion, entrada, salida, saldo, cliente_id, prestamo_id
		 FROM caja `+where+`
		 ORDER BY fecha `+orden+`, id `+orden+`
		 LIMIT $`+strconv.Itoa(i)+` OFFSET $`+strconv.Itoa(i+1),
		append(args, limit, offset)...,
	)
	if err != nil {
		jsonError(w, "error consultando caja", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	data := []models.MovimientoCaja{}
	for rows.Next() {
		var m models.MovimientoCaja
		rows.Scan(&m.ID, &m.Fecha, &m.Tipo, &m.Descripcion, &m.Entrada, &m.Salida, &m.Saldo, &m.ClienteID, &m.PrestamoID)
		data = append(data, m)
	}

	pages := total / limit
	if total%limit != 0 {
		pages++
	}
	jsonOK(w, map[string]any{
		"data": data, "total": total, "page": page,
		"limit": limit, "pages": pages, "saldo_actual": saldo,
	})
}

func RegistrarAporte(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Fecha       string  `json:"fecha"`
		Monto       int64   `json:"monto"`
		Socio       string  `json:"socio"`
		Descripcion *string `json:"descripcion"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Monto <= 0 || body.Fecha == "" {
		jsonError(w, "datos inválidos", http.StatusBadRequest)
		return
	}
	ctx := context.Background()
	tx, _ := db.Pool.Begin(ctx)
	defer tx.Rollback(ctx)

	var saldo int64
	tx.QueryRow(ctx, `SELECT COALESCE(saldo,0) FROM caja ORDER BY id DESC LIMIT 1`).Scan(&saldo)

	tx.Exec(ctx, `INSERT INTO aportes (fecha, monto, socio, descripcion) VALUES ($1,$2,$3,$4)`,
		body.Fecha, body.Monto, body.Socio, body.Descripcion)

	desc := "Aporte"
	if body.Descripcion != nil {
		desc = *body.Descripcion
	}
	tx.Exec(ctx, `INSERT INTO caja (fecha, tipo, descripcion, entrada, salida, saldo) VALUES ($1,'aporte',$2,$3,0,$4)`,
		body.Fecha, desc, body.Monto, saldo+body.Monto)

	if err := tx.Commit(ctx); err != nil {
		jsonError(w, "error guardando aporte", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, map[string]string{"mensaje": "Aporte registrado"})
}

func RegistrarRetiro(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Fecha       string  `json:"fecha"`
		Monto       int64   `json:"monto"`
		Descripcion *string `json:"descripcion"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Monto <= 0 || body.Fecha == "" {
		jsonError(w, "datos inválidos", http.StatusBadRequest)
		return
	}
	ctx := context.Background()
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		jsonError(w, "error de transacción", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	var saldo int64
	tx.QueryRow(ctx, `SELECT COALESCE(saldo,0) FROM caja ORDER BY id DESC LIMIT 1`).Scan(&saldo)
	if body.Monto > saldo {
		jsonError(w, "saldo insuficiente en caja", http.StatusBadRequest)
		return
	}
	desc := "Retiro"
	if body.Descripcion != nil {
		desc = *body.Descripcion
	}
	_, err = tx.Exec(ctx, `INSERT INTO caja (fecha, tipo, descripcion, entrada, salida, saldo) VALUES ($1,'retiro',$2,0,$3,$4)`,
		body.Fecha, desc, body.Monto, saldo-body.Monto)
	if err != nil {
		jsonError(w, "error registrando retiro", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		jsonError(w, "error guardando retiro", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, map[string]string{"mensaje": "Retiro registrado"})
}
