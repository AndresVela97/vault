-- Índices para queries frecuentes
CREATE INDEX IF NOT EXISTS idx_prestamos_estado      ON prestamos(estado);
CREATE INDEX IF NOT EXISTS idx_prestamos_cliente_id  ON prestamos(cliente_id);
CREATE INDEX IF NOT EXISTS idx_prestamos_vencimiento ON prestamos(fecha_vencimiento);
CREATE INDEX IF NOT EXISTS idx_cobros_prestamo_id    ON cobros(prestamo_id);
CREATE INDEX IF NOT EXISTS idx_cobros_fecha          ON cobros(fecha);
CREATE INDEX IF NOT EXISTS idx_caja_fecha            ON caja(fecha);
CREATE INDEX IF NOT EXISTS idx_caja_cobro_id         ON caja(cobro_id);
CREATE INDEX IF NOT EXISTS idx_bolsillo_prestamo_id  ON bolsillo(prestamo_id);

-- Fix constraint de socios (acepta nombres reales)
ALTER TABLE aportes DROP CONSTRAINT IF EXISTS aportes_socio_check;
ALTER TABLE aportes ADD CONSTRAINT aportes_socio_check
    CHECK (socio = ANY (ARRAY['Edwin','Stiven','a','b']));

ALTER TABLE pagos_socios DROP CONSTRAINT IF EXISTS pagos_socios_socio_check;
ALTER TABLE pagos_socios ADD CONSTRAINT pagos_socios_socio_check
    CHECK (socio = ANY (ARRAY['a','b']));
