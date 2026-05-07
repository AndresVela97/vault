package handlers

import (
	"context"
	"fmt"
	"net/http"

	"vault/internal/db"
)

func ExportarCSV(w http.ResponseWriter, r *http.Request) {
	mes := queryStr(r, "mes")
	if mes == "" {
		jsonError(w, "parámetro mes requerido (YYYY-MM)", http.StatusBadRequest)
		return
	}

	rows, err := db.Pool.Query(context.Background(), `
		SELECT co.fecha::text, c.nombre, p.capital, p.interes_monto,
		       co.concepto, co.monto, co.ganancia_a, co.ganancia_b, COALESCE(co.notas,'')
		FROM cobros co
		JOIN prestamos p ON p.id = co.prestamo_id
		JOIN clientes c  ON c.id = p.cliente_id
		WHERE co.fecha::text LIKE $1
		ORDER BY co.fecha ASC`, mes+"%")
	if err != nil {
		jsonError(w, "error generando reporte", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="vault-cobros-%s.csv"`, mes))

	fmt.Fprintf(w, "Fecha,Cliente,Capital,Interes,Concepto,Monto,Ganancia Edwin,Ganancia Stiven,Notas\n")

	var totalMonto, totalA, totalB int64
	for rows.Next() {
		var fecha, cliente, concepto, notas string
		var capital, interes, monto, ganA, ganB int64
		rows.Scan(&fecha, &cliente, &capital, &interes, &concepto, &monto, &ganA, &ganB, &notas)
		fmt.Fprintf(w, "%s,%s,%d,%d,%s,%d,%d,%d,%s\n",
			fecha, cliente, capital, interes, concepto, monto, ganA, ganB, notas)
		totalMonto += monto
		totalA += ganA
		totalB += ganB
	}
	fmt.Fprintf(w, "TOTAL,,,,,%d,%d,%d,\n", totalMonto, totalA, totalB)
}
