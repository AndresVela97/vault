//go:build ignore

package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

func main() {
	godotenv.Load()

	sqlitePath := "/Users/evelayate/Documents/Proyectos personales/prestamos-app/prestamos.db"
	sq, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		log.Fatal("SQLite:", err)
	}
	defer sq.Close()

	pg, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("PostgreSQL:", err)
	}
	defer pg.Close()

	ctx := context.Background()

	fmt.Println("🔄 Limpiando tablas...")
	pg.Exec(ctx, `TRUNCATE pagos_socios,aportes,bolsillo,caja,cobros,prestamos,clientes,usuarios RESTART IDENTITY CASCADE`)

	migrate := func(label, sqSQL, pgSQL string) {
		rows, err := sq.Query(sqSQL)
		if err != nil {
			log.Printf("❌ %s query: %v", label, err)
			return
		}
		defer rows.Close()
		cols, _ := rows.Columns()
		ok := 0
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			rows.Scan(ptrs...)
			args := make([]any, len(cols))
			for i, v := range vals {
				if b, ok2 := v.([]byte); ok2 {
					args[i] = string(b)
				} else {
					args[i] = v
				}
			}
			if _, err := pg.Exec(ctx, pgSQL, args...); err != nil {
				log.Printf("  ⚠ %s fila omitida: %v", label, err)
			} else {
				ok++
			}
		}
		fmt.Printf("  ✓ %s: %d filas\n", label, ok)
	}

	fmt.Println("👥 Usuarios...")
	migrate("usuarios",
		`SELECT id,nombre,email,password,rol FROM usuarios`,
		`INSERT INTO usuarios(id,nombre,email,password,rol) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`)

	fmt.Println("👥 Clientes...")
	migrate("clientes",
		`SELECT id,nombre,telefono,cedula,direccion,ubicacion_url,tipo,parentesco FROM clientes`,
		`INSERT INTO clientes(id,nombre,telefono,cedula,direccion,ubicacion_url,tipo,parentesco) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING`)

	fmt.Println("💰 Préstamos...")
	migrate("prestamos",
		`SELECT id,cliente_id,capital,interes_pct,interes_monto,tipo_pago,fecha_inicio,fecha_vencimiento,estado,origen_capital,notas FROM prestamos`,
		`INSERT INTO prestamos(id,cliente_id,capital,interes_pct,interes_monto,tipo_pago,fecha_inicio,fecha_vencimiento,estado,origen_capital,notas) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT DO NOTHING`)

	fmt.Println("✅ Cobros...")
	migrate("cobros",
		`SELECT id,prestamo_id,monto,concepto,fecha,COALESCE(ganancia_a,0),COALESCE(ganancia_b,0),notas,usuario_id FROM cobros`,
		`INSERT INTO cobros(id,prestamo_id,monto,concepto,fecha,ganancia_a,ganancia_b,notas,usuario_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT DO NOTHING`)

	fmt.Println("🏦 Caja...")
	migrate("caja",
		`SELECT id,fecha,tipo,descripcion,COALESCE(entrada,0),COALESCE(salida,0),COALESCE(saldo,0),cliente_id,prestamo_id,cobro_id FROM caja`,
		`INSERT INTO caja(id,fecha,tipo,descripcion,entrada,salida,saldo,cliente_id,prestamo_id,cobro_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT DO NOTHING`)

	fmt.Println("👝 Bolsillo...")
	migrate("bolsillo",
		`SELECT id,fecha,tipo,descripcion,COALESCE(entrada,0),COALESCE(salida,0),COALESCE(saldo,0),cliente_id,prestamo_id,cobro_id FROM bolsillo`,
		`INSERT INTO bolsillo(id,fecha,tipo,descripcion,entrada,salida,saldo,cliente_id,prestamo_id,cobro_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT DO NOTHING`)

	fmt.Println("💵 Aportes...")
	migrate("aportes",
		`SELECT id,fecha,monto,'a' as socio,descripcion FROM aportes`,
		`INSERT INTO aportes(id,fecha,monto,socio,descripcion) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`)

	fmt.Println("💸 Pagos socios...")
	migrate("pagos_socios",
		`SELECT id,fecha,socio,monto,descripcion,caja_id FROM pagos_socios`,
		`INSERT INTO pagos_socios(id,fecha,socio,monto,descripcion,caja_id) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`)

	fmt.Println("\n🔧 Sincronizando secuencias...")
	for _, t := range []string{"usuarios","clientes","prestamos","cobros","caja","bolsillo","aportes","pagos_socios"} {
		pg.Exec(ctx, fmt.Sprintf(`SELECT setval('%s_id_seq', COALESCE((SELECT MAX(id) FROM %s),1))`, t, t))
		fmt.Printf("  ✓ %s_id_seq\n", t)
	}

	fmt.Println("\n✅ Migración completada")
}
