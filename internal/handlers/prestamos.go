package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"vault/internal/db"
	"vault/internal/models"

	"github.com/go-chi/chi/v5"
)

func calcularMora(fechaVenc string, pagado, totalEsperado int64) int64 {
	if pagado >= totalEsperado {
		return 0
	}
	venc, err := time.Parse("2006-01-02", fechaVenc)
	if err != nil {
		return 0
	}
	hoy := time.Now().Truncate(24 * time.Hour)
	if !hoy.After(venc) {
		return 0
	}
	dias := int(hoy.Sub(venc).Hours() / 24)
	if dias <= 0 {
		return 0
	}
	semanas := int64((dias-1)/7) + 1
	return semanas * 5000
}

func ListarPrestamos(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 50)
	if limit > 200 {
		limit = 200
	}
	page := queryInt(r, "page", 1)
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	tipo   := queryStr(r, "tipo")
	estado := queryStr(r, "estado")

	where := "WHERE 1=1"
	args := []any{}
	i := 1
	if tipo != "" {
		where += " AND c.tipo = $" + strconv.Itoa(i)
		args = append(args, tipo)
		i++
	}
	if estado != "" {
		where += " AND p.estado = $" + strconv.Itoa(i)
		args = append(args, estado)
		i++
	}

	var total int
	db.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM prestamos p JOIN clientes c ON c.id = p.cliente_id `+where,
		args...,
	).Scan(&total)

	rows, err := db.Pool.Query(context.Background(), `
		SELECT p.id, p.cliente_id, c.nombre, c.tipo, c.telefono,
		       p.capital, p.interes_pct, p.interes_monto, p.tipo_pago,
		       p.fecha_inicio::text, p.fecha_vencimiento::text,
		       p.estado, p.origen_capital, p.notas,
		       COALESCE(cob.total_pagado, 0)
		FROM prestamos p
		JOIN clientes c ON c.id = p.cliente_id
		LEFT JOIN (
		    SELECT prestamo_id, SUM(monto) as total_pagado
		    FROM cobros GROUP BY prestamo_id
		) cob ON cob.prestamo_id = p.id
		`+where+`
		ORDER BY p.fecha_vencimiento ASC
		LIMIT $`+strconv.Itoa(i)+` OFFSET $`+strconv.Itoa(i+1),
		append(args, limit, offset)...,
	)
	if err != nil {
		jsonError(w, "error consultando préstamos", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	hoy := time.Now().Format("2006-01-02")
	data := []models.Prestamo{}
	for rows.Next() {
		var p models.Prestamo
		rows.Scan(&p.ID, &p.ClienteID, &p.ClienteNombre, &p.ClienteTipo, &p.Telefono,
			&p.Capital, &p.InteresPct, &p.InteresMonto, &p.TipoPago,
			&p.FechaInicio, &p.FechaVencimiento,
			&p.Estado, &p.OrigenCapital, &p.Notas, &p.TotalPagado)

		if p.ClienteTipo == "familiar" {
			p.TotalEsperado = p.Capital
		} else {
			p.TotalEsperado = p.Capital + p.InteresMonto
			p.Mora = calcularMora(p.FechaVencimiento, p.TotalPagado, p.TotalEsperado)
			if p.FechaVencimiento < hoy && p.TotalPagado < p.TotalEsperado {
				venc, _ := time.Parse("2006-01-02", p.FechaVencimiento)
				p.DiasMora = int(time.Since(venc).Hours() / 24)
			}
		}
		if p.Saldo = p.TotalEsperado + p.Mora - p.TotalPagado; p.Saldo < 0 {
			p.Saldo = 0
		}
		data = append(data, p)
	}

	pages := total / limit
	if total%limit != 0 {
		pages++
	}
	jsonOK(w, models.Paginated[models.Prestamo]{Data: data, Total: total, Page: page, Limit: limit, Pages: pages})
}

func ObtenerPrestamo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var p models.Prestamo
	err := db.Pool.QueryRow(context.Background(), `
		SELECT p.id, p.cliente_id, c.nombre, c.tipo, c.telefono,
		       p.capital, p.interes_pct, p.interes_monto, p.tipo_pago,
		       p.fecha_inicio::text, p.fecha_vencimiento::text,
		       p.estado, p.origen_capital, p.notas,
		       COALESCE(cob.total_pagado, 0),
		       c.cedula, c.direccion, c.ubicacion_url
		FROM prestamos p
		JOIN clientes c ON c.id = p.cliente_id
		LEFT JOIN (SELECT prestamo_id, SUM(monto) as total_pagado FROM cobros GROUP BY prestamo_id) cob
		       ON cob.prestamo_id = p.id
		WHERE p.id = $1`, id,
	).Scan(&p.ID, &p.ClienteID, &p.ClienteNombre, &p.ClienteTipo, &p.Telefono,
		&p.Capital, &p.InteresPct, &p.InteresMonto, &p.TipoPago,
		&p.FechaInicio, &p.FechaVencimiento,
		&p.Estado, &p.OrigenCapital, &p.Notas, &p.TotalPagado,
		&p.Cedula, &p.Direccion, &p.UbicacionURL)
	if err != nil {
		jsonError(w, "préstamo no encontrado", http.StatusNotFound)
		return
	}
	if p.ClienteTipo == "familiar" {
		p.TotalEsperado = p.Capital
	} else {
		p.TotalEsperado = p.Capital + p.InteresMonto
		p.Mora = calcularMora(p.FechaVencimiento, p.TotalPagado, p.TotalEsperado)
	}
	if p.Saldo = p.TotalEsperado + p.Mora - p.TotalPagado; p.Saldo < 0 {
		p.Saldo = 0
	}

	cobros := []models.Cobro{}
	rows, _ := db.Pool.Query(context.Background(), `
		SELECT co.id, co.prestamo_id, co.monto, co.concepto, co.fecha::text,
		       co.ganancia_a, co.ganancia_b, co.notas, co.usuario_id, u.nombre
		FROM cobros co
		LEFT JOIN usuarios u ON u.id = co.usuario_id
		WHERE co.prestamo_id = $1
		ORDER BY co.fecha DESC`, id)
	defer rows.Close()
	for rows.Next() {
		var c models.Cobro
		rows.Scan(&c.ID, &c.PrestamoID, &c.Monto, &c.Concepto, &c.Fecha,
			&c.GananciaA, &c.GananciaB, &c.Notas, &c.UsuarioID, &c.RegistradoPor)
		cobros = append(cobros, c)
	}

	jsonOK(w, map[string]any{"prestamo": p, "cobros": cobros})
}

func CrearPrestamo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ClienteID        int     `json:"cliente_id"`
		Capital          int64   `json:"capital"`
		InteresPct       float64 `json:"interes_pct"`
		TipoPago         string  `json:"tipo_pago"`
		FechaInicio      string  `json:"fecha_inicio"`
		FechaVencimiento string  `json:"fecha_vencimiento"`
		Fuente           string  `json:"fuente"` // "caja" o "bolsillo"
		OrigenCapital    *string `json:"origen_capital"`
		Notas            *string `json:"notas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "datos inválidos", http.StatusBadRequest)
		return
	}
	if body.ClienteID == 0 || body.Capital <= 0 || body.FechaInicio == "" || body.FechaVencimiento == "" {
		jsonError(w, "campos requeridos incompletos", http.StatusBadRequest)
		return
	}
	if body.Fuente == "" {
		body.Fuente = "caja"
	}
	interesMonto := int64(float64(body.Capital) * body.InteresPct / 100)

	ctx := context.Background()
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		jsonError(w, "error de transacción", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	// Guardar fuente en origen_capital si no viene otro valor
	origenCapital := body.OrigenCapital
	if origenCapital == nil {
		origenCapital = &body.Fuente
	}

	var id int
	err = tx.QueryRow(ctx, `
		INSERT INTO prestamos (cliente_id, capital, interes_pct, interes_monto, tipo_pago,
		                       fecha_inicio, fecha_vencimiento, origen_capital, notas)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		body.ClienteID, body.Capital, body.InteresPct, interesMonto, body.TipoPago,
		body.FechaInicio, body.FechaVencimiento, origenCapital, body.Notas,
	).Scan(&id)
	if err != nil {
		jsonError(w, "error creando préstamo", http.StatusInternalServerError)
		return
	}

	// Registrar salida en caja o bolsillo según fuente
	// Si fuente == "deuda", el capital viene de una deuda externa y no afecta ninguna cuenta
	if body.Fuente != "deuda" {
		var clienteNombre string
		tx.QueryRow(ctx, `SELECT nombre FROM clientes WHERE id=$1`, body.ClienteID).Scan(&clienteNombre)

		if body.Fuente == "bolsillo" {
			var saldo int64
			tx.QueryRow(ctx, `SELECT COALESCE(saldo,0) FROM bolsillo ORDER BY fecha DESC, id DESC LIMIT 1`).Scan(&saldo)
			tx.Exec(ctx, `
				INSERT INTO bolsillo (fecha, tipo, descripcion, entrada, salida, saldo, cliente_id, prestamo_id)
				VALUES ($1,'prestamo',$2,0,$3,$4,$5,$6)`,
				body.FechaInicio, clienteNombre, body.Capital, saldo-body.Capital, body.ClienteID, id)
		} else {
			var saldo int64
			tx.QueryRow(ctx, `SELECT COALESCE(saldo,0) FROM caja ORDER BY fecha DESC, id DESC LIMIT 1`).Scan(&saldo)
			tipoCaja := "prestamo"
			if body.TipoPago == "libre" {
				tipoCaja = "prestamo_familiar"
			}
			tx.Exec(ctx, `
				INSERT INTO caja (fecha, tipo, descripcion, entrada, salida, saldo, cliente_id, prestamo_id)
				VALUES ($1,$2,$3,0,$4,$5,$6,$7)`,
				body.FechaInicio, tipoCaja, clienteNombre, body.Capital, saldo-body.Capital, body.ClienteID, id)
		}
	}

	tx.Commit(ctx)
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, map[string]any{"id": id, "mensaje": "Préstamo creado"})
}

func CrearCliente(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Nombre       string  `json:"nombre"`
		Telefono     *string `json:"telefono"`
		Cedula       *string `json:"cedula"`
		Direccion    *string `json:"direccion"`
		UbicacionURL *string `json:"ubicacion_url"`
		Tipo         string  `json:"tipo"`
		Parentesco   *string `json:"parentesco"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Nombre == "" {
		jsonError(w, "nombre requerido", http.StatusBadRequest)
		return
	}
	if body.Tipo == "" {
		body.Tipo = "interes"
	}
	var id int
	err := db.Pool.QueryRow(context.Background(), `
		INSERT INTO clientes (nombre, telefono, cedula, direccion, ubicacion_url, tipo, parentesco)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		body.Nombre, body.Telefono, body.Cedula, body.Direccion,
		body.UbicacionURL, body.Tipo, body.Parentesco,
	).Scan(&id)
	if err != nil {
		jsonError(w, "error creando cliente", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, map[string]any{"id": id, "nombre": body.Nombre, "tipo": body.Tipo, "mensaje": "Cliente creado"})
}

func EditarCliente(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Nombre       string  `json:"nombre"`
		Telefono     *string `json:"telefono"`
		Cedula       *string `json:"cedula"`
		Direccion    *string `json:"direccion"`
		UbicacionURL *string `json:"ubicacion_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Nombre == "" {
		jsonError(w, "nombre requerido", http.StatusBadRequest)
		return
	}
	_, err := db.Pool.Exec(context.Background(), `
		UPDATE clientes SET nombre=$1, telefono=$2, cedula=$3, direccion=$4, ubicacion_url=$5
		WHERE id=$6`,
		body.Nombre, body.Telefono, body.Cedula, body.Direccion, body.UbicacionURL, id)
	if err != nil {
		jsonError(w, "error actualizando cliente", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"mensaje": "Cliente actualizado"})
}

func EditarPrestamo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Capital          int64   `json:"capital"`
		InteresPct       float64 `json:"interes_pct"`
		InteresMonto     int64   `json:"interes_monto"`
		FechaInicio      string  `json:"fecha_inicio"`
		FechaVencimiento string  `json:"fecha_vencimiento"`
		TipoPago         string  `json:"tipo_pago"`
		Notas            *string `json:"notas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "datos inválidos", http.StatusBadRequest)
		return
	}
	if body.Capital <= 0 || body.TipoPago == "" || body.FechaInicio == "" || body.FechaVencimiento == "" {
		jsonError(w, "capital, tipo_pago y fechas son requeridos", http.StatusBadRequest)
		return
	}
	_, err := db.Pool.Exec(context.Background(), `
		UPDATE prestamos SET
			capital           = $1,
			interes_pct       = $2,
			interes_monto     = $3,
			fecha_inicio      = $4::date,
			fecha_vencimiento = $5::date,
			tipo_pago         = $6,
			notas             = $7
		WHERE id = $8`,
		body.Capital, body.InteresPct, body.InteresMonto,
		body.FechaInicio, body.FechaVencimiento,
		body.TipoPago, body.Notas, id)
	if err != nil {
		jsonError(w, "error actualizando préstamo", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"mensaje": "Préstamo actualizado"})
}

func CambiarEstado(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Estado string `json:"estado"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "datos inválidos", http.StatusBadRequest)
		return
	}
	if body.Estado != "pendiente" && body.Estado != "pagado" && body.Estado != "mora" {
		jsonError(w, "estado inválido", http.StatusBadRequest)
		return
	}
	_, err := db.Pool.Exec(context.Background(),
		`UPDATE prestamos SET estado=$1 WHERE id=$2`, body.Estado, id)
	if err != nil {
		jsonError(w, "error actualizando estado", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"mensaje": "Estado actualizado"})
}

func ProrrogarPrestamo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		FechaVencimiento string `json:"fecha_vencimiento"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.FechaVencimiento == "" {
		jsonError(w, "fecha_vencimiento requerida", http.StatusBadRequest)
		return
	}
	_, err := db.Pool.Exec(context.Background(),
		`UPDATE prestamos SET fecha_vencimiento=$1, estado='pendiente' WHERE id=$2`,
		body.FechaVencimiento, id)
	if err != nil {
		jsonError(w, "error prorrogando préstamo", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"mensaje": "Vencimiento actualizado"})
}

func RenovarPrestamo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		FechaVencimiento string `json:"fecha_vencimiento"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.FechaVencimiento == "" {
		jsonError(w, "fecha_vencimiento requerida", http.StatusBadRequest)
		return
	}
	ctx := context.Background()
	var clienteTipo string
	var capital, interesMonto int64
	err := db.Pool.QueryRow(ctx, `
		SELECT c.tipo, p.capital, p.interes_pct * p.capital / 100
		FROM prestamos p JOIN clientes c ON c.id=p.cliente_id WHERE p.id=$1`, id,
	).Scan(&clienteTipo, &capital, &interesMonto)
	if err != nil {
		jsonError(w, "préstamo no encontrado", http.StatusNotFound)
		return
	}
	if clienteTipo == "familiar" {
		jsonError(w, "los préstamos familiares no se renuevan con interés", http.StatusBadRequest)
		return
	}
	_, err = db.Pool.Exec(ctx, `
		UPDATE prestamos SET
			interes_monto = interes_monto + $1,
			fecha_vencimiento = $2,
			estado = 'pendiente'
		WHERE id = $3`, interesMonto, body.FechaVencimiento, id)
	if err != nil {
		jsonError(w, "error renovando préstamo", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{
		"mensaje":           "Préstamo renovado",
		"interes_agregado":  interesMonto,
	})
}

func ListarClientes(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Pool.Query(context.Background(),
		`SELECT id, nombre, telefono, cedula, tipo, parentesco FROM clientes ORDER BY nombre ASC`)
	if err != nil {
		jsonError(w, "error consultando clientes", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	clientes := []models.Cliente{}
	for rows.Next() {
		var c models.Cliente
		rows.Scan(&c.ID, &c.Nombre, &c.Telefono, &c.Cedula, &c.Tipo, &c.Parentesco)
		clientes = append(clientes, c)
	}
	jsonOK(w, clientes)
}
