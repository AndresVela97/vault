#!/usr/bin/env python3
"""Migra datos de SQLite (prestamos-app) a PostgreSQL (vault)."""
import sqlite3, psycopg2, sys

SQLITE_PATH = "/Users/evelayate/Documents/Proyectos personales/prestamos-app/prestamos.db"
PG_DSN      = "dbname=vault host=localhost sslmode=disable"

sq = sqlite3.connect(SQLITE_PATH)
sq.row_factory = sqlite3.Row
pg = psycopg2.connect(PG_DSN)
pg.autocommit = False
cur = pg.cursor()

def run(label, sq_sql, pg_sql, transform=None):
    rows = sq.execute(sq_sql).fetchall()
    ok = 0
    for r in rows:
        row = dict(r)
        if transform:
            row = transform(row)
        if row is None:
            continue
        try:
            cur.execute(pg_sql, row)
            ok += 1
        except Exception as e:
            print(f"  ⚠ fila omitida: {e} — {dict(r)}")
    print(f"  ✓ {label}: {ok}/{len(rows)}")

print("🔄 Limpiando tablas en PostgreSQL...")
cur.execute("""
    TRUNCATE pagos_socios, aportes, bolsillo, caja, cobros, prestamos, clientes, usuarios
    RESTART IDENTITY CASCADE
""")

print("\n📋 Migrando usuarios...")
run("usuarios",
    "SELECT id, nombre, email, password, rol FROM usuarios",
    "INSERT INTO usuarios (id,nombre,email,password,rol) VALUES (%(id)s,%(nombre)s,%(email)s,%(password)s,%(rol)s) ON CONFLICT DO NOTHING"
)

print("👥 Migrando clientes...")
run("clientes",
    "SELECT id,nombre,telefono,cedula,direccion,ubicacion_url,tipo,parentesco FROM clientes",
    "INSERT INTO clientes (id,nombre,telefono,cedula,direccion,ubicacion_url,tipo,parentesco) VALUES (%(id)s,%(nombre)s,%(telefono)s,%(cedula)s,%(direccion)s,%(ubicacion_url)s,%(tipo)s,%(parentesco)s) ON CONFLICT DO NOTHING"
)

print("💰 Migrando préstamos...")
run("prestamos",
    "SELECT id,cliente_id,capital,interes_pct,interes_monto,tipo_pago,fecha_inicio,fecha_vencimiento,estado,origen_capital,notas FROM prestamos",
    "INSERT INTO prestamos (id,cliente_id,capital,interes_pct,interes_monto,tipo_pago,fecha_inicio,fecha_vencimiento,estado,origen_capital,notas) VALUES (%(id)s,%(cliente_id)s,%(capital)s,%(interes_pct)s,%(interes_monto)s,%(tipo_pago)s,%(fecha_inicio)s,%(fecha_vencimiento)s,%(estado)s,%(origen_capital)s,%(notas)s) ON CONFLICT DO NOTHING"
)

print("✅ Migrando cobros...")
run("cobros",
    "SELECT id,prestamo_id,monto,concepto,fecha,COALESCE(ganancia_a,0) as ganancia_a,COALESCE(ganancia_b,0) as ganancia_b,notas,usuario_id FROM cobros",
    "INSERT INTO cobros (id,prestamo_id,monto,concepto,fecha,ganancia_a,ganancia_b,notas,usuario_id) VALUES (%(id)s,%(prestamo_id)s,%(monto)s,%(concepto)s,%(fecha)s,%(ganancia_a)s,%(ganancia_b)s,%(notas)s,%(usuario_id)s) ON CONFLICT DO NOTHING"
)

print("🏦 Migrando caja...")
run("caja",
    "SELECT id,fecha,tipo,descripcion,COALESCE(entrada,0) as entrada,COALESCE(salida,0) as salida,COALESCE(saldo,0) as saldo,cliente_id,prestamo_id,cobro_id FROM caja",
    "INSERT INTO caja (id,fecha,tipo,descripcion,entrada,salida,saldo,cliente_id,prestamo_id,cobro_id) VALUES (%(id)s,%(fecha)s,%(tipo)s,%(descripcion)s,%(entrada)s,%(salida)s,%(saldo)s,%(cliente_id)s,%(prestamo_id)s,%(cobro_id)s) ON CONFLICT DO NOTHING"
)

print("👝 Migrando bolsillo...")
run("bolsillo",
    "SELECT id,fecha,tipo,descripcion,COALESCE(entrada,0) as entrada,COALESCE(salida,0) as salida,COALESCE(saldo,0) as saldo,cliente_id,prestamo_id,cobro_id FROM bolsillo",
    "INSERT INTO bolsillo (id,fecha,tipo,descripcion,entrada,salida,saldo,cliente_id,prestamo_id,cobro_id) VALUES (%(id)s,%(fecha)s,%(tipo)s,%(descripcion)s,%(entrada)s,%(salida)s,%(saldo)s,%(cliente_id)s,%(prestamo_id)s,%(cobro_id)s) ON CONFLICT DO NOTHING"
)

print("💵 Migrando aportes...")
run("aportes",
    "SELECT id,fecha,monto,socio,descripcion FROM aportes",
    "INSERT INTO aportes (id,fecha,monto,socio,descripcion) VALUES (%(id)s,%(fecha)s,%(monto)s,%(socio)s,%(descripcion)s) ON CONFLICT DO NOTHING"
)

print("💸 Migrando pagos_socios...")
run("pagos_socios",
    "SELECT id,fecha,socio,monto,descripcion,caja_id FROM pagos_socios",
    "INSERT INTO pagos_socios (id,fecha,socio,monto,descripcion,caja_id) VALUES (%(id)s,%(fecha)s,%(socio)s,%(monto)s,%(descripcion)s,%(caja_id)s) ON CONFLICT DO NOTHING"
)

# Sincronizar secuencias de autoincrement
print("\n🔧 Sincronizando secuencias...")
for tabla in ["usuarios","clientes","prestamos","cobros","caja","bolsillo","aportes","pagos_socios"]:
    cur.execute(f"SELECT setval('{tabla}_id_seq', COALESCE((SELECT MAX(id) FROM {tabla}),1))")
    print(f"  ✓ {tabla}_id_seq")

pg.commit()
print("\n✅ Migración completada exitosamente")
sq.close(); pg.close()
