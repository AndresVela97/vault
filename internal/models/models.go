package models

import "time"

type Usuario struct {
	ID       int    `json:"id"`
	Nombre   string `json:"nombre"`
	Email    string `json:"email"`
	Password string `json:"-"`
	Rol      string `json:"rol"`
}

type Cliente struct {
	ID           int     `json:"id"`
	Nombre       string  `json:"nombre"`
	Telefono     *string `json:"telefono"`
	Cedula       *string `json:"cedula"`
	Direccion    *string `json:"direccion"`
	UbicacionURL *string `json:"ubicacion_url"`
	Tipo         string  `json:"tipo"`
	Parentesco   *string `json:"parentesco"`
}

type Prestamo struct {
	ID               int      `json:"id"`
	ClienteID        int      `json:"cliente_id"`
	ClienteNombre    string   `json:"cliente_nombre"`
	ClienteTipo      string   `json:"cliente_tipo"`
	Telefono         *string  `json:"telefono"`
	Cedula           *string  `json:"cedula"`
	Direccion        *string  `json:"direccion"`
	UbicacionURL     *string  `json:"ubicacion_url"`
	Capital          int64    `json:"capital"`
	InteresPct       float64  `json:"interes_pct"`
	InteresMonto     int64    `json:"interes_monto"`
	TipoPago         string   `json:"tipo_pago"`
	FechaInicio      string   `json:"fecha_inicio"`
	FechaVencimiento string   `json:"fecha_vencimiento"`
	Estado           string   `json:"estado"`
	OrigenCapital    *string  `json:"origen_capital"`
	Notas            *string  `json:"notas"`
	TotalPagado      int64    `json:"total_pagado"`
	TotalEsperado    int64    `json:"total_esperado"`
	Saldo            int64    `json:"saldo"`
	Mora             int64    `json:"mora"`
	DiasMora         int      `json:"dias_mora"`
}

type Cobro struct {
	ID            int     `json:"id"`
	PrestamoID    int     `json:"prestamo_id"`
	ClienteNombre string  `json:"cliente_nombre"`
	Monto         int64   `json:"monto"`
	Concepto      string  `json:"concepto"`
	Fecha         string  `json:"fecha"`
	GananciaA     int64   `json:"ganancia_a"`
	GananciaB     int64   `json:"ganancia_b"`
	Notas         *string `json:"notas"`
	UsuarioID     *int    `json:"usuario_id"`
	RegistradoPor *string `json:"registrado_por"`
}

type MovimientoCaja struct {
	ID          int     `json:"id"`
	Fecha       string  `json:"fecha"`
	Tipo        string  `json:"tipo"`
	Descripcion *string `json:"descripcion"`
	Entrada     int64   `json:"entrada"`
	Salida      int64   `json:"salida"`
	Saldo       int64   `json:"saldo"`
	ClienteID   *int    `json:"cliente_id"`
	PrestamoID  *int    `json:"prestamo_id"`
}

type Aporte struct {
	ID          int    `json:"id"`
	Fecha       string `json:"fecha"`
	Monto       int64  `json:"monto"`
	Socio       string `json:"socio"`
	Descripcion *string `json:"descripcion"`
}

type PagoSocio struct {
	ID          int     `json:"id"`
	Fecha       string  `json:"fecha"`
	Socio       string  `json:"socio"`
	Monto       int64   `json:"monto"`
	Descripcion *string `json:"descripcion"`
	CajaID      *int    `json:"caja_id"`
}

type GananciaMes struct {
	Mes    string `json:"mes"`
	TotalA int64  `json:"total_a"`
	TotalB int64  `json:"total_b"`
	Total  int64  `json:"total"`
}

type TopDeudor struct {
	ClienteNombre string `json:"cliente_nombre"`
	PrestamoID    int    `json:"prestamo_id"`
	Saldo         int64  `json:"saldo"`
	Estado        string `json:"estado"`
}

type Dashboard struct {
	// Negocio
	CapitalCalleNegocio int64 `json:"capital_calle_negocio"`
	PrestamosActivos    int   `json:"prestamos_activos"`
	PrestamosMora       int   `json:"prestamos_mora"`
	SaldoCaja           int64 `json:"saldo_caja"`
	GananciaMes         int64 `json:"ganancia_mes"`
	GananciaMesA        int64 `json:"ganancia_mes_a"`
	GananciaMesB        int64 `json:"ganancia_mes_b"`
	// Familia
	CapitalCalleFamilia int64 `json:"capital_calle_familia"`
	FamiliaActivos      int   `json:"familia_activos"`
	SaldoBolsillo       int64 `json:"saldo_bolsillo"`
	RecuperadoMes       int64 `json:"recuperado_mes"`
	// Analítica
	ROI              float64        `json:"roi"`
	CapitalAportado  int64          `json:"capital_aportado"`
	GananciasHistorial []GananciaMes `json:"ganancias_historial"`
	Top3Deudores     []TopDeudor    `json:"top3_deudores"`
	// Alertas
	EnMora        []Prestamo `json:"en_mora"`
	VencenEsta    []Prestamo `json:"vencen_esta_semana"`
	UltimosCobros []Cobro    `json:"ultimos_cobros"`
}

type Claims struct {
	UserID int    `json:"user_id"`
	Rol    string `json:"rol"`
	Nombre string `json:"nombre"`
}

type Paginated[T any] struct {
	Data   []T `json:"data"`
	Total  int `json:"total"`
	Page   int `json:"page"`
	Limit  int `json:"limit"`
	Pages  int `json:"pages"`
}

func Today() string {
	return time.Now().Format("2006-01-02")
}

func CurrentMonth() string {
	return time.Now().Format("2006-01")
}

func ParseMes(mes string) (time.Time, error) {
	return time.Parse("2006-01", mes)
}
