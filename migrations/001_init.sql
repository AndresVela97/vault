CREATE TABLE IF NOT EXISTS usuarios (
    id         SERIAL PRIMARY KEY,
    nombre     TEXT NOT NULL,
    email      TEXT NOT NULL UNIQUE,
    password   TEXT NOT NULL,
    rol        TEXT NOT NULL DEFAULT 'socio' CHECK (rol IN ('admin','socio')),
    creado_en  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS clientes (
    id            SERIAL PRIMARY KEY,
    nombre        TEXT NOT NULL,
    telefono      TEXT,
    cedula        TEXT,
    direccion     TEXT,
    ubicacion_url TEXT,
    tipo          TEXT NOT NULL DEFAULT 'interes' CHECK (tipo IN ('interes','familiar')),
    parentesco    TEXT,
    creado_en     TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS prestamos (
    id                SERIAL PRIMARY KEY,
    cliente_id        INTEGER NOT NULL REFERENCES clientes(id),
    capital           BIGINT NOT NULL,
    interes_pct       NUMERIC(5,2) NOT NULL DEFAULT 20,
    interes_monto     BIGINT NOT NULL,
    tipo_pago         TEXT NOT NULL CHECK (tipo_pago IN ('total','interes','semanal','libre')),
    fecha_inicio      DATE NOT NULL,
    fecha_vencimiento DATE NOT NULL,
    estado            TEXT NOT NULL DEFAULT 'pendiente' CHECK (estado IN ('pendiente','pagado','mora')),
    origen_capital    TEXT,
    notas             TEXT,
    creado_en         TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS cobros (
    id          SERIAL PRIMARY KEY,
    prestamo_id INTEGER NOT NULL REFERENCES prestamos(id),
    monto       BIGINT NOT NULL,
    concepto    TEXT NOT NULL CHECK (concepto IN ('total','interes','capital','mora')),
    fecha       DATE NOT NULL,
    ganancia_a  BIGINT NOT NULL DEFAULT 0,
    ganancia_b  BIGINT NOT NULL DEFAULT 0,
    notas       TEXT,
    usuario_id  INTEGER REFERENCES usuarios(id),
    creado_en   TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS caja (
    id          SERIAL PRIMARY KEY,
    fecha       DATE NOT NULL,
    tipo        TEXT NOT NULL,
    descripcion TEXT,
    entrada     BIGINT NOT NULL DEFAULT 0,
    salida      BIGINT NOT NULL DEFAULT 0,
    saldo       BIGINT NOT NULL DEFAULT 0,
    cliente_id  INTEGER REFERENCES clientes(id),
    prestamo_id INTEGER REFERENCES prestamos(id),
    cobro_id    INTEGER REFERENCES cobros(id),
    creado_en   TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bolsillo (
    id          SERIAL PRIMARY KEY,
    fecha       DATE NOT NULL,
    tipo        TEXT NOT NULL,
    descripcion TEXT,
    entrada     BIGINT NOT NULL DEFAULT 0,
    salida      BIGINT NOT NULL DEFAULT 0,
    saldo       BIGINT NOT NULL DEFAULT 0,
    cliente_id  INTEGER REFERENCES clientes(id),
    prestamo_id INTEGER REFERENCES prestamos(id),
    cobro_id    INTEGER REFERENCES cobros(id),
    creado_en   TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS aportes (
    id          SERIAL PRIMARY KEY,
    fecha       DATE NOT NULL,
    monto       BIGINT NOT NULL,
    socio       TEXT NOT NULL CHECK (socio IN ('a','b')),
    descripcion TEXT,
    creado_en   TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS pagos_socios (
    id          SERIAL PRIMARY KEY,
    fecha       DATE NOT NULL,
    socio       TEXT NOT NULL CHECK (socio IN ('a','b')),
    monto       BIGINT NOT NULL,
    descripcion TEXT,
    caja_id     INTEGER REFERENCES caja(id),
    creado_en   TIMESTAMPTZ DEFAULT NOW()
);
