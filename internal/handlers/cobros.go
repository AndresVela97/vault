package handlers

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"

	"vault/internal/db"
	"vault/internal/middleware"
	"vault/internal/models"
)

func splitA() float64 {
	v, _ := strconv.ParseFloat(os.Getenv("SPLIT_A"), 64)
	if v == 0 {
		v = 60
	}
	return v / 100
}

func splitB() float64 { return 1 - splitA() }

func RegistrarCobro(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r)
	var body struct {
		PrestamoID   int64   `json:"prestamo_id"`
		Monto        int64   `json:"monto"`
		Concepto     string  `json:"concepto"`
		Fecha        string  `json:"fecha"`
		Notas        *string `json:"notas"`
		InteresCobro *int64  `json:"interes_cobro"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "datos inválidos", http.StatusBadRequest)
		return
	}
	if body.PrestamoID == 0 || body.Monto <= 0 || body.Concepto == "" || body.Fecha == "" {
		jsonError(w, "campos requeridos incompletos", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		jsonError(w, "error de transacción", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	var clienteTipo, clienteNombre, origenCapital string
	var clienteID int
	var capital, interesMonto int64

	err = tx.QueryRow(ctx, `
		SELECT c.tipo, c.nombre, c.id, p.capital, p.interes_monto, COALESCE(p.origen_capital,'')
		FROM prestamos p JOIN clientes c ON c.id = p.cliente_id
		WHERE p.id = $1`, body.PrestamoID,
	).Scan(&clienteTipo, &clienteNombre, &clienteID, &capital, &interesMonto, &origenCapital)
	if err != nil {
		jsonError(w, "préstamo no encontrado", http.StatusNotFound)
		return
	}

	var gananciaA, gananciaB int64
	if clienteTipo == "interes" && body.Concepto != "capital" {
		base := body.Monto
		if body.Concepto == "total" {
			if body.InteresCobro != nil && *body.InteresCobro > 0 {
				base = *body.InteresCobro
			} else {
				var interesYaPagado int64
				tx.QueryRow(ctx, `SELECT COALESCE(SUM(monto),0) FROM cobros WHERE prestamo_id=$1 AND concepto='interes'`, body.PrestamoID).Scan(&interesYaPagado)
				interesRestante := interesMonto - interesYaPagado
				if interesRestante < 0 {
					interesRestante = 0
				}
				base = interesRestante
			}
		}
		gananciaA = int64(math.Round(float64(base) * splitA()))
		gananciaB = int64(math.Round(float64(base) * splitB()))
	}

	var cobroID int
	err = tx.QueryRow(ctx, `
		INSERT INTO cobros (prestamo_id, monto, concepto, fecha, ganancia_a, ganancia_b, notas, usuario_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		body.PrestamoID, body.Monto, body.Concepto, body.Fecha,
		gananciaA, gananciaB, body.Notas, u.ID,
	).Scan(&cobroID)
	if err != nil {
		jsonError(w, "error registrando cobro", http.StatusInternalServerError)
		return
	}

	usaCaja := clienteTipo != "familiar" || origenCapital == "caja"
	if usaCaja {
		var saldo int64
		tx.QueryRow(ctx, `SELECT COALESCE(saldo,0) FROM caja ORDER BY fecha DESC, id DESC LIMIT 1`).Scan(&saldo)
		tipoCaja := map[string]string{
			"capital": "cobro_capital", "mora": "cobro_mora",
		}[body.Concepto]
		if tipoCaja == "" {
			tipoCaja = "cobro_interes"
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO caja (fecha, tipo, descripcion, entrada, salida, saldo, cliente_id, prestamo_id, cobro_id)
			VALUES ($1,$2,$3,$4,0,$5,$6,$7,$8)`,
			body.Fecha, tipoCaja, clienteNombre+" - "+body.Concepto,
			body.Monto, saldo+body.Monto, clienteID, body.PrestamoID, cobroID)
	} else {
		var saldo int64
		tx.QueryRow(ctx, `SELECT COALESCE(saldo,0) FROM bolsillo ORDER BY fecha DESC, id DESC LIMIT 1`).Scan(&saldo)
		_, err = tx.Exec(ctx, `
			INSERT INTO bolsillo (fecha, tipo, descripcion, entrada, salida, saldo, cliente_id, prestamo_id, cobro_id)
			VALUES ($1,'cobro',$2,$3,0,$4,$5,$6,$7)`,
			body.Fecha, "Cobro bolsillo - "+clienteNombre,
			body.Monto, saldo+body.Monto, clienteID, body.PrestamoID, cobroID)
	}
	if err != nil {
		jsonError(w, "error actualizando caja", http.StatusInternalServerError)
		return
	}

	var totalPagado int64
	tx.QueryRow(ctx, `SELECT COALESCE(SUM(monto),0) FROM cobros WHERE prestamo_id = $1`, body.PrestamoID).Scan(&totalPagado)

	var totalEsperado int64
	if clienteTipo == "familiar" {
		totalEsperado = capital
	} else {
		totalEsperado = capital + interesMonto
	}
	if totalPagado >= totalEsperado {
		tx.Exec(ctx, `UPDATE prestamos SET estado = 'pagado' WHERE id = $1`, body.PrestamoID)
	}

	if err := tx.Commit(ctx); err != nil {
		jsonError(w, "error guardando cobro", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	jsonOK(w, map[string]any{
		"id":         cobroID,
		"ganancia_a": gananciaA,
		"ganancia_b": gananciaB,
		"mensaje":    "Cobro registrado",
	})
}

func EliminarCobro(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := context.Background()
	tx, _ := db.Pool.Begin(ctx)
	defer tx.Rollback(ctx)

	var prestamoID int
	var monto int64
	err := tx.QueryRow(ctx, `SELECT prestamo_id, monto FROM cobros WHERE id = $1`, id).Scan(&prestamoID, &monto)
	if err != nil {
		jsonError(w, "cobro no encontrado", http.StatusNotFound)
		return
	}

	// Obtener el movimiento de caja asociado antes de borrarlo
	var cajaID *int
	var cajaEntrada, cajaSalida int64
	tx.QueryRow(ctx, `SELECT id, entrada, salida FROM caja WHERE cobro_id = $1`, id).
		Scan(&cajaID, &cajaEntrada, &cajaSalida)

	var bolsilloID *int
	var bolsilloEntrada, bolsilloSalida int64
	tx.QueryRow(ctx, `SELECT id, entrada, salida FROM bolsillo WHERE cobro_id = $1`, id).
		Scan(&bolsilloID, &bolsilloEntrada, &bolsilloSalida)

	tx.Exec(ctx, `DELETE FROM caja WHERE cobro_id = $1`, id)
	tx.Exec(ctx, `DELETE FROM bolsillo WHERE cobro_id = $1`, id)

	// Ajustar saldos posteriores en O(1) — restar la entrada o sumar la salida eliminada
	if cajaID != nil {
		delta := cajaSalida - cajaEntrada
		tx.Exec(ctx, `UPDATE caja SET saldo = saldo + $1 WHERE id > $2`, delta, *cajaID)
	}
	if bolsilloID != nil {
		delta := bolsilloSalida - bolsilloEntrada
		tx.Exec(ctx, `UPDATE bolsillo SET saldo = saldo + $1 WHERE id > $2`, delta, *bolsilloID)
	}

	tx.Exec(ctx, `DELETE FROM cobros WHERE id = $1`, id)

	// Recalcular estado préstamo
	var clienteTipo string
	var cap, intM int64
	var fechaVenc string
	tx.QueryRow(ctx, `
		SELECT c.tipo, p.capital, p.interes_monto, p.fecha_vencimiento::text
		FROM prestamos p JOIN clientes c ON c.id = p.cliente_id WHERE p.id = $1`, prestamoID,
	).Scan(&clienteTipo, &cap, &intM, &fechaVenc)

	var totalPagado int64
	tx.QueryRow(ctx, `SELECT COALESCE(SUM(monto),0) FROM cobros WHERE prestamo_id = $1`, prestamoID).Scan(&totalPagado)
	var totalEsperado int64
	if clienteTipo == "familiar" {
		totalEsperado = cap
	} else {
		totalEsperado = cap + intM
	}
	if totalPagado < totalEsperado {
		hoy := models.Today()
		estado := "pendiente"
		if fechaVenc < hoy && clienteTipo != "familiar" {
			estado = "mora"
		}
		tx.Exec(ctx, `UPDATE prestamos SET estado = $1 WHERE id = $2`, estado, prestamoID)
	}

	if err := tx.Commit(ctx); err != nil {
		jsonError(w, "error eliminando cobro", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"mensaje": "Cobro eliminado"})
}

func ListarCobros(w http.ResponseWriter, r *http.Request) {
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
	if v := queryStr(r, "prestamo_id"); v != "" {
		where += " AND co.prestamo_id = $" + strconv.Itoa(i)
		args = append(args, v)
		i++
	}
	if v := queryStr(r, "desde"); v != "" {
		where += " AND co.fecha >= $" + strconv.Itoa(i)
		args = append(args, v)
		i++
	}
	if v := queryStr(r, "hasta"); v != "" {
		where += " AND co.fecha <= $" + strconv.Itoa(i)
		args = append(args, v)
		i++
	}

	var total int
	db.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM cobros co JOIN prestamos p ON p.id = co.prestamo_id JOIN clientes c ON c.id = p.cliente_id `+where,
		args...,
	).Scan(&total)

	rows, err := db.Pool.Query(context.Background(), `
		SELECT co.id, co.prestamo_id, c.nombre, co.monto, co.concepto, co.fecha::text,
		       co.ganancia_a, co.ganancia_b, co.notas, co.usuario_id, u.nombre
		FROM cobros co
		JOIN prestamos p ON p.id = co.prestamo_id
		JOIN clientes c ON c.id = p.cliente_id
		LEFT JOIN usuarios u ON u.id = co.usuario_id
		`+where+`
		ORDER BY co.fecha DESC
		LIMIT $`+strconv.Itoa(i)+` OFFSET $`+strconv.Itoa(i+1),
		append(args, limit, offset)...,
	)
	if err != nil {
		jsonError(w, "error consultando cobros", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	data := []models.Cobro{}
	for rows.Next() {
		var c models.Cobro
		rows.Scan(&c.ID, &c.PrestamoID, &c.ClienteNombre, &c.Monto, &c.Concepto, &c.Fecha,
			&c.GananciaA, &c.GananciaB, &c.Notas, &c.UsuarioID, &c.RegistradoPor)
		data = append(data, c)
	}

	pages := total / limit
	if total%limit != 0 {
		pages++
	}
	jsonOK(w, models.Paginated[models.Cobro]{Data: data, Total: total, Page: page, Limit: limit, Pages: pages})
}

// GET /cobros/ganancias?mes=YYYY-MM — resumen de ganancias de un mes con comparación al anterior
func GetGanancias(w http.ResponseWriter, r *http.Request) {
	mesActual := queryStr(r, "mes")
	if mesActual == "" {
		mesActual = models.CurrentMonth()
	}

	ctx := context.Background()

	type resumen struct {
		TotalA int64 `json:"total_a"`
		TotalB int64 `json:"total_b"`
		Total  int64 `json:"total"`
	}

	getResumen := func(mes string) resumen {
		var rs resumen
		db.Pool.QueryRow(ctx,
			`SELECT COALESCE(SUM(ganancia_a),0), COALESCE(SUM(ganancia_b),0)
			 FROM cobros WHERE fecha::text LIKE $1`, mes+"%",
		).Scan(&rs.TotalA, &rs.TotalB)
		rs.Total = rs.TotalA + rs.TotalB
		return rs
	}

	// Mes anterior
	t, _ := models.ParseMes(mesActual)
	prev := t.AddDate(0, -1, 0).Format("2006-01")

	actual := getResumen(mesActual)
	anterior := getResumen(prev)

	// Detalle cobros del mes con ganancia
	rows, err := db.Pool.Query(ctx, `
		SELECT co.id, co.prestamo_id, c.nombre, co.monto, co.concepto,
		       co.fecha::text, co.ganancia_a, co.ganancia_b, co.notas, co.usuario_id, u.nombre
		FROM cobros co
		JOIN prestamos p ON p.id = co.prestamo_id
		JOIN clientes c ON c.id = p.cliente_id
		LEFT JOIN usuarios u ON u.id = co.usuario_id
		WHERE co.fecha::text LIKE $1 AND (co.ganancia_a > 0 OR co.ganancia_b > 0)
		ORDER BY co.fecha DESC`, mesActual+"%")
	if err != nil {
		jsonError(w, "error consultando ganancias", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	detalle := []models.Cobro{}
	for rows.Next() {
		var c models.Cobro
		rows.Scan(&c.ID, &c.PrestamoID, &c.ClienteNombre, &c.Monto, &c.Concepto,
			&c.Fecha, &c.GananciaA, &c.GananciaB, &c.Notas, &c.UsuarioID, &c.RegistradoPor)
		detalle = append(detalle, c)
	}

	// Meses disponibles (con cobros)
	mesesRows, _ := db.Pool.Query(ctx,
		`SELECT DISTINCT to_char(fecha,'YYYY-MM') as mes FROM cobros ORDER BY mes DESC`)
	defer mesesRows.Close()
	meses := []string{}
	for mesesRows.Next() {
		var m string
		mesesRows.Scan(&m)
		meses = append(meses, m)
	}

	jsonOK(w, map[string]any{
		"mes":      mesActual,
		"actual":   actual,
		"anterior": anterior,
		"detalle":  detalle,
		"meses":    meses,
	})
}
