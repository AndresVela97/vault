package handlers

import (
	"context"
	"net/http"

	"vault/internal/db"
	"vault/internal/models"
)

func GetDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	mes := models.CurrentMonth()
	hoy := models.Today()

	d := models.Dashboard{}

	// ── Query 1: todos los KPIs escalares en un solo viaje ──────────────────
	db.Pool.QueryRow(ctx, `
		WITH
		neg AS (
			SELECT COALESCE(SUM(p.capital),0) AS capital, COUNT(DISTINCT p.cliente_id) AS activos
			FROM prestamos p JOIN clientes c ON c.id = p.cliente_id
			WHERE p.estado != 'pagado' AND c.tipo = 'interes'
		),
		mora_cnt AS (
			SELECT COUNT(*) AS n FROM prestamos p JOIN clientes c ON c.id = p.cliente_id
			WHERE p.estado = 'mora' AND c.tipo = 'interes'
		),
		fam AS (
			SELECT COALESCE(SUM(p.capital) - COALESCE(SUM(cob.pagado),0), 0) AS capital,
			       COUNT(DISTINCT p.cliente_id) AS activos
			FROM prestamos p JOIN clientes c ON c.id = p.cliente_id
			LEFT JOIN (
				SELECT prestamo_id, SUM(monto) AS pagado FROM cobros
				WHERE concepto = 'capital' GROUP BY prestamo_id
			) cob ON cob.prestamo_id = p.id
			WHERE p.estado != 'pagado' AND c.tipo = 'familiar'
		),
		saldos AS (
			SELECT
				(SELECT COALESCE(saldo,0) FROM caja    ORDER BY id DESC LIMIT 1) AS caja,
				(SELECT COALESCE(saldo,0) FROM bolsillo ORDER BY id DESC LIMIT 1) AS bolsillo
		),
		ganancias AS (
			SELECT COALESCE(SUM(ganancia_a),0) AS ga, COALESCE(SUM(ganancia_b),0) AS gb
			FROM cobros WHERE fecha::text LIKE $1
		),
		fam_rec AS (
			SELECT COALESCE(SUM(co.monto),0) AS total
			FROM cobros co JOIN prestamos p ON p.id = co.prestamo_id
			JOIN clientes c ON c.id = p.cliente_id
			WHERE c.tipo = 'familiar' AND co.fecha::text LIKE $1
		),
		roi_data AS (
			SELECT COALESCE(SUM(monto),0) AS aportado FROM aportes
		),
		roi_gan AS (
			SELECT COALESCE(SUM(ganancia_a),0) AS total FROM cobros
		)
		SELECT
			neg.capital, neg.activos, mora_cnt.n,
			fam.capital, fam.activos,
			saldos.caja, saldos.bolsillo,
			ganancias.ga, ganancias.gb,
			fam_rec.total,
			roi_data.aportado, roi_gan.total
		FROM neg, mora_cnt, fam, saldos, ganancias, fam_rec, roi_data, roi_gan`,
		mes+"%",
	).Scan(
		&d.CapitalCalleNegocio, &d.PrestamosActivos, &d.PrestamosMora,
		&d.CapitalCalleFamilia, &d.FamiliaActivos,
		&d.SaldoCaja, &d.SaldoBolsillo,
		&d.GananciaMesA, &d.GananciaMesB,
		&d.RecuperadoMes,
		&d.CapitalAportado, &d.ROI,
	)
	d.GananciaMes = d.GananciaMesA + d.GananciaMesB
	if d.CapitalAportado > 0 {
		d.ROI = d.ROI / float64(d.CapitalAportado) * 100
	} else {
		d.ROI = 0
	}

	// ── Query 2: historial ganancias 6 meses ────────────────────────────────
	histRows, _ := db.Pool.Query(ctx, `
		SELECT to_char(fecha,'YYYY-MM') AS mes,
		       COALESCE(SUM(ganancia_a),0), COALESCE(SUM(ganancia_b),0)
		FROM cobros
		WHERE fecha >= (CURRENT_DATE - interval '6 months')
		GROUP BY mes ORDER BY mes ASC`)
	defer histRows.Close()
	d.GananciasHistorial = []models.GananciaMes{}
	for histRows.Next() {
		var g models.GananciaMes
		histRows.Scan(&g.Mes, &g.TotalA, &g.TotalB)
		g.Total = g.TotalA + g.TotalB
		d.GananciasHistorial = append(d.GananciasHistorial, g)
	}

	// ── Query 3: top 3 deudores ─────────────────────────────────────────────
	topRows, _ := db.Pool.Query(ctx, `
		SELECT c.nombre, p.id,
		       (p.capital + p.interes_monto) - COALESCE(cob.total_pagado,0) AS saldo,
		       p.estado
		FROM prestamos p
		JOIN clientes c ON c.id = p.cliente_id
		LEFT JOIN (SELECT prestamo_id, SUM(monto) AS total_pagado FROM cobros GROUP BY prestamo_id) cob
		       ON cob.prestamo_id = p.id
		WHERE p.estado != 'pagado' AND c.tipo = 'interes'
		ORDER BY saldo DESC LIMIT 3`)
	defer topRows.Close()
	d.Top3Deudores = []models.TopDeudor{}
	for topRows.Next() {
		var t models.TopDeudor
		topRows.Scan(&t.ClienteNombre, &t.PrestamoID, &t.Saldo, &t.Estado)
		d.Top3Deudores = append(d.Top3Deudores, t)
	}

	// ── Query 4: en mora ────────────────────────────────────────────────────
	rows, _ := db.Pool.Query(ctx, `
		SELECT p.id, p.cliente_id, c.nombre, c.tipo, c.telefono,
		       p.capital, p.interes_pct, p.interes_monto, p.tipo_pago,
		       p.fecha_inicio::text, p.fecha_vencimiento::text, p.estado,
		       p.origen_capital, p.notas, COALESCE(cob.total_pagado,0)
		FROM prestamos p JOIN clientes c ON c.id = p.cliente_id
		LEFT JOIN (SELECT prestamo_id, SUM(monto) AS total_pagado FROM cobros GROUP BY prestamo_id) cob
		       ON cob.prestamo_id = p.id
		WHERE p.estado = 'mora'
		ORDER BY p.fecha_vencimiento ASC LIMIT 10`)
	d.EnMora = scanPrestamos(rows)

	// ── Query 5: vencen esta semana + últimos cobros (un solo viaje) ────────
	rows, _ = db.Pool.Query(ctx, `
		SELECT p.id, p.cliente_id, c.nombre, c.tipo, c.telefono,
		       p.capital, p.interes_pct, p.interes_monto, p.tipo_pago,
		       p.fecha_inicio::text, p.fecha_vencimiento::text, p.estado,
		       p.origen_capital, p.notas, COALESCE(cob.total_pagado,0)
		FROM prestamos p JOIN clientes c ON c.id = p.cliente_id
		LEFT JOIN (SELECT prestamo_id, SUM(monto) AS total_pagado FROM cobros GROUP BY prestamo_id) cob
		       ON cob.prestamo_id = p.id
		WHERE p.estado = 'pendiente'
		  AND p.fecha_vencimiento >= $1::date
		  AND p.fecha_vencimiento <= ($1::date + interval '7 days')
		ORDER BY p.fecha_vencimiento ASC LIMIT 10`, hoy)
	d.VencenEsta = scanPrestamos(rows)

	cobRows, _ := db.Pool.Query(ctx, `
		SELECT co.id, co.prestamo_id, c.nombre, co.monto, co.concepto,
		       co.fecha::text, co.ganancia_a, co.ganancia_b, co.notas, co.usuario_id, u.nombre
		FROM cobros co
		JOIN prestamos p ON p.id = co.prestamo_id
		JOIN clientes c ON c.id = p.cliente_id
		LEFT JOIN usuarios u ON u.id = co.usuario_id
		ORDER BY co.fecha DESC, co.id DESC LIMIT 8`)
	defer cobRows.Close()
	d.UltimosCobros = []models.Cobro{}
	for cobRows.Next() {
		var c models.Cobro
		cobRows.Scan(&c.ID, &c.PrestamoID, &c.ClienteNombre, &c.Monto, &c.Concepto,
			&c.Fecha, &c.GananciaA, &c.GananciaB, &c.Notas, &c.UsuarioID, &c.RegistradoPor)
		d.UltimosCobros = append(d.UltimosCobros, c)
	}

	jsonOK(w, d)
}

func scanPrestamos(rows interface {
	Next() bool
	Scan(...any) error
	Close()
}) []models.Prestamo {
	defer rows.Close()
	hoy := models.Today()
	result := []models.Prestamo{}
	for rows.Next() {
		var p models.Prestamo
		rows.Scan(&p.ID, &p.ClienteID, &p.ClienteNombre, &p.ClienteTipo, &p.Telefono,
			&p.Capital, &p.InteresPct, &p.InteresMonto, &p.TipoPago,
			&p.FechaInicio, &p.FechaVencimiento, &p.Estado,
			&p.OrigenCapital, &p.Notas, &p.TotalPagado)
		if p.ClienteTipo == "familiar" {
			p.TotalEsperado = p.Capital
		} else {
			p.TotalEsperado = p.Capital + p.InteresMonto
			p.Mora = calcularMora(p.FechaVencimiento, p.TotalPagado, p.TotalEsperado)
			if p.FechaVencimiento < hoy && p.TotalPagado < p.TotalEsperado {
				p.DiasMora = int(p.Mora / 5000 * 7)
			}
		}
		if p.Saldo = p.TotalEsperado + p.Mora - p.TotalPagado; p.Saldo < 0 {
			p.Saldo = 0
		}
		result = append(result, p)
	}
	return result
}
