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
