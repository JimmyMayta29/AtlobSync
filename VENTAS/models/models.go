package models

type Periodo struct {
	NumEjercicio string `json:"numEjercicio"`
	DesEstado    string `json:"desEstado"`
	LisPeriodos  []struct {
		PerTributario string `json:"perTributario"`
		CodEstado     string `json:"codEstado"`
		DesEstado     string `json:"desEstado"`
	} `json:"lisPeriodos"`
}

type TipoCambioRequest struct {
	FecEmision         string   `json:"fecEmision" binding:"required"`
	CodMoneda          string   `json:"codMoneda" binding:"required"`
	MtoTipoCambio      float64  `json:"mtoTipoCambio" binding:"required"`
	MtoCambioMonedaExt *float64 `json:"mtoCambioMonedaExt,omitempty"`
}
type TipoCambioIndividualRequest struct {
	CodCar             string   `json:"codCar" binding:"required"`
	CodMoneda          string   `json:"codMoneda" binding:"required"`
	MtoTipoCambio      float64  `json:"mtoTipoCambio" binding:"required"`
	MtoCambioMonedaExt *float64 `json:"mtoCambioMonedaExt,omitempty"`
}
type EliminarComprobanteRequest struct {
	NumSerieCDP string `json:"numSerieCDP" binding:"required"`
	NumCDP      string `json:"numCDP" binding:"required"`
	CodCar      string `json:"codCar" binding:"required"`
	CodTipoCDP  string `json:"codTipoCDP" binding:"required"`
}
type EliminarComprobantePreliminarRequest struct {
	CodCar      string `json:"codCar" binding:"required"`
	CodTipoCDP  string `json:"codTipoCDP" binding:"required"`
	NumSerieCDP string `json:"numSerieCDP" binding:"required"`
	NumCDP      string `json:"numCDP" binding:"required"`
}
type ComprobanteEliminar struct {
	CodCar      string `json:"codCar" binding:"required"`
	CodTipoCDP  string `json:"codTipoCDP" binding:"required"`
	NumSerieCDP string `json:"numSerieCDP" binding:"required"`
	NumCDP      string `json:"numCDP" binding:"required"`
}
