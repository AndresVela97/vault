# Vault

App de gestión de préstamos personales. Controla capital, cobros, ganancias y caja para dos socios.

## Funcionalidades

- Préstamos de negocio (con interés) y familia (sin interés)
- Registro de cobros con split automático de ganancias entre socios
- Control de caja y bolsillo con historial de movimientos
- Dashboard con KPIs, mora, vencimientos y gráfica de ganancias
- Exportación CSV
- PWA instalable en móvil
- Roles: admin (operaciones completas) y socio (solo lectura + cobros)

## Stack

- **Backend:** Go + Chi + pgx
- **Base de datos:** PostgreSQL
- **Frontend:** HTML/CSS/JS vanilla (SPA, sin frameworks)

## Variables de entorno

Crea un archivo `.env` en la raíz:

```env
DATABASE_URL=postgres://usuario:password@localhost/vault?sslmode=disable
JWT_SECRET=cambia-esto-por-una-cadena-larga-y-aleatoria
PORT=4000

# Credenciales iniciales (solo se usan si la DB está vacía)
ADMIN_EMAIL=admin@tudominio.com
ADMIN_PASSWORD=tu-password-seguro
SOCIO_EMAIL=socio@tudominio.com
SOCIO_PASSWORD=su-password-seguro

# Porcentaje de ganancia para el socio A (por defecto 60%)
SPLIT_A=60
```

## Correr localmente

```bash
# Requiere Go 1.21+ y PostgreSQL

# Crear la base de datos
createdb vault

# Levantar el servidor
go run ./cmd/server/main.go

# Abre http://localhost:4000
```

## Despliegue en Railway

1. Subir el código a GitHub
2. En [railway.app](https://railway.app) → New Project → Deploy from GitHub
3. Agregar servicio PostgreSQL
4. Configurar las variables de entorno listadas arriba
5. Railway detecta el `Dockerfile` y despliega automáticamente
6. Settings → Networking → Generate Domain para obtener la URL pública

## Migraciones

Las migraciones se ejecutan automáticamente al arrancar el servidor. Los archivos SQL en `migrations/` se aplican en orden alfabético. Para agregar cambios de schema crear un nuevo archivo `003_descripcion.sql`.

## Estructura

```
vault/
├── cmd/server/       # Punto de entrada
├── internal/
│   ├── db/           # Conexión PostgreSQL
│   ├── handlers/     # Handlers HTTP
│   ├── middleware/   # Auth, rate limiting
│   └── models/       # Structs y tipos
├── migrations/       # SQL de schema e índices
└── static/           # Frontend (index.html, PWA)
```
