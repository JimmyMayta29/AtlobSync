package rhe

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SwodLore/sire-api/internal/database"
	"github.com/gin-gonic/gin"
)

type mockCredentialsProvider struct {
	ruc         string
	razonSocial string
	usuarioSOL  string
	claveSOL    string
}

func (m *mockCredentialsProvider) GetDecryptedCredentials(ruc string) (string, string, string, string, string, error) {
	return m.ruc, m.razonSocial, m.usuarioSOL, m.claveSOL, "", nil
}

type mockReceptorProvider struct {
	res *ReceptorQueryResult
	err error
}

func (m *mockReceptorProvider) ConsultarPeriodo(ctx context.Context, ruc, usuarioSOL, claveSOL, periodo string) (*ReceptorQueryResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.res, nil
}

func (m *mockReceptorProvider) Status(ruc string) SolRHEStatus {
	return SolRHEStatus{
		BrowserEncontrado: true,
		BrowserNombre:     "msedge.exe",
		Estado:            "LISTO",
		Mensaje:           "Modo Local listo",
	}
}

const sampleOfficialSUNATTXT = "Fecha de Emisión|Tipo Doc. Emitido|Nro. Doc. Emitido|Estado Doc. Emitido|Tipo de Doc. Emisor|Nro. Doc. Emisor|Apellidos y Nombres, Denominación o Razón Social del Emisor|Tipo de Renta|Gratuito|Descripción|Observación|Moneda de Operación|Renta Bruta|Impuesto a la Renta|Renta Neta|Monto Neto Pendiente de Pago|\r\n" +
	"20/10/2026|RH|E001-217|NO ANULADO|RUC|10423496628|PEREZ GOMEZ JUAN CARLOS|A|-|ASESORIA CONTABLE OCTUBRE|-|SOLES|800.00|0.00|800.00|0.00|\r\n" +
	"25/10/2026|RH|E001-567|NO ANULADO|RUC|10061559375|TORRES SALAZAR MARIA ELENA|A|NO|SERVICIO DE AUDITORIA TRIBUTARIA|PAGADO|SOLES|2,826.09|226.09|2,600.00|0.00|\r\n" +
	"26/10/2026|RH|E001-568|REVERTIDO|RUC|10061559375|TORRES SALAZAR MARIA ELENA|A|NO|ERROR DE MONTO|-|SOLES|1,500.00|120.00|1,380.00|0.00|\r\n"

func TestParseReceptorTXTAndResumen(t *testing.T) {
	items, err := ParseReceptorTXT([]byte(sampleOfficialSUNATTXT), "20600000001", "202610")
	if err != nil {
		t.Fatalf("error inesperado parseando TXT oficial: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("se esperaban 3 registros, se obtuvieron %d", len(items))
	}

	if items[0].Serie != "E001" || items[0].Numero != "217" || items[0].Descripcion != "ASESORIA CONTABLE OCTUBRE" {
		t.Fatalf("fila 0 inesperada: %+v", items[0])
	}
	if items[0].Gratuito != "" || items[0].Observacion != "" {
		t.Fatalf("guiones '-' deben limpiarse a cadena vacía, obtenido gratuito=%q obs=%q", items[0].Gratuito, items[0].Observacion)
	}
	if items[1].RentaBruta != 2826.09 || items[1].ImpuestoRenta != 226.09 || items[1].RentaNeta != 2600.00 {
		t.Fatalf("montos con coma decimal mal parseados: %+v", items[1])
	}
	if !items[2].EsAnulado || items[1].EsAnulado || items[0].EsAnulado {
		t.Fatalf("clasificación EsAnulado incorrecta: [0]=%v [1]=%v [2]=%v", items[0].EsAnulado, items[1].EsAnulado, items[2].EsAnulado)
	}

	res := ComputeResumen(items)
	if res.TotalRegistros != 3 || res.Activos != 2 || res.AnuladosRevertidos != 1 {
		t.Fatalf("conteos inesperados: %+v", res)
	}
	if res.ConRetencion != 1 || res.SinRetencion != 1 {
		t.Fatalf("conteos de retención inesperados: %+v", res)
	}
	if res.TotalRentaBruta != 3626.09 || res.TotalImpuestoRenta != 226.09 || res.TotalRentaNeta != 3400.00 {
		t.Fatalf("totales activos deben excluir REVERTIDO; obtenido bruta=%.2f ir=%.2f neta=%.2f",
			res.TotalRentaBruta, res.TotalImpuestoRenta, res.TotalRentaNeta)
	}
}

func TestParseReceptorTXT_Windows1252Fallback(t *testing.T) {
	// 'ó' = 0xF3, 'ñ' = 0xF1 en Windows-1252
	raw := []byte("Fecha de Emisi\xf3n|Tipo Doc. Emitido|Nro. Doc. Emitido|Estado Doc. Emitido|Tipo de Doc. Emisor|Nro. Doc. Emisor|Apellidos y Nombres|Tipo de Renta|Gratuito|Descripci\xf3n|Observaci\xf3n|Moneda de Operaci\xf3n|Renta Bruta|Impuesto a la Renta|Renta Neta|Monto Neto Pendiente de Pago|\r\n" +
		"10/09/2026|RH|E001-12|NO ANULADO|RUC|10111111111|MU\xd1OZ PE\xd1A|A|NO|DISE\xd1O GR\xc1FICO|-|DOLARES|500.00|40.00|460.00|460.00|\r\n")

	items, err := ParseReceptorTXT(raw, "20600000001", "202609")
	if err != nil {
		t.Fatalf("error inesperado con Windows-1252: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("se esperaba 1 registro, se obtuvieron %d", len(items))
	}
	if items[0].RazonEmisor != "MUÑOZ PEÑA" || items[0].Descripcion != "DISEÑO GRÁFICO" || items[0].Moneda != "DOLARES" {
		t.Fatalf("decodificación Windows-1252 incorrecta: %+v", items[0])
	}
}

func TestComputePeriodoDateRange(t *testing.T) {
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, peruTimezone)

	d1, d2, err := ComputePeriodoDateRange("202610", now)
	if err != nil || d1 != "01/10/2026" || d2 != "31/10/2026" {
		t.Fatalf("mes pasado inesperado: %s..%s err=%v", d1, d2, err)
	}

	// Mes bisiesto
	d1, d2, err = ComputePeriodoDateRange("202402", now)
	if err != nil || d1 != "01/02/2024" || d2 != "29/02/2024" {
		t.Fatalf("febrero bisiesto inesperado: %s..%s err=%v", d1, d2, err)
	}

	// Mes en curso: fechaHasta se acota a hoy (21/11/2026)
	d1, d2, err = ComputePeriodoDateRange("202611", now)
	if err != nil || d1 != "01/11/2026" || d2 != "21/11/2026" {
		t.Fatalf("mes actual debe acotarse a hoy: %s..%s err=%v", d1, d2, err)
	}

	// Mes futuro: debe rechazarse con error claro
	if _, _, err := ComputePeriodoDateRange("202612", now); err == nil {
		t.Fatalf("se esperaba error para periodo futuro 202612")
	}
}

func TestServiceAndController_ConsultListExport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("error init db: %v", err)
	}
	defer db.Close()

	repo := NewRepository(db)
	creds := &mockCredentialsProvider{
		ruc:         "20600000001",
		razonSocial: "EMPRESA DEMO S.A.C.",
		usuarioSOL:  "USUARIODEMO",
		claveSOL:    "CLAVEDEMO",
	}
	svc := NewService(repo, creds, t.TempDir())
	svc.SetProvider(&mockReceptorProvider{
		res: &ReceptorQueryResult{
			RawTXT:     []byte(sampleOfficialSUNATTXT),
			FileName:   "archivo.txt",
			FechaDesde: "01/10/2026",
			FechaHasta: "31/10/2026",
		},
	})

	ctrl := NewController(svc)
	r := gin.New()
	api := r.Group("/api")
	ctrl.RegisterRoutes(api, func(c *gin.Context) { c.Next() })

	// 1. POST /consultar
	reqConsult := httptest.NewRequest(http.MethodPost, "/api/empresas/20600000001/rhe/consultar", strings.NewReader(`{"periodo":"202610"}`))
	reqConsult.Header.Set("Content-Type", "application/json")
	wConsult := httptest.NewRecorder()
	r.ServeHTTP(wConsult, reqConsult)
	if wConsult.Code != http.StatusOK {
		t.Fatalf("POST consultar esperado 200, obtenido %d: %s", wConsult.Code, wConsult.Body.String())
	}

	var consultBody struct {
		Success bool                `json:"success"`
		Data    RHEConsultaResponse `json:"data"`
	}
	if err := json.Unmarshal(wConsult.Body.Bytes(), &consultBody); err != nil {
		t.Fatalf("JSON inválido en consultar: %v", err)
	}
	if !consultBody.Success || consultBody.Data.Total != 3 || consultBody.Data.Resumen.Activos != 2 {
		t.Fatalf("respuesta consultar inesperada: %+v", consultBody)
	}

	// 2. GET listar con filtro estado=ACTIVO
	reqList := httptest.NewRequest(http.MethodGet, "/api/empresas/20600000001/rhe?periodo=202610&estado=ACTIVO", nil)
	wList := httptest.NewRecorder()
	r.ServeHTTP(wList, reqList)
	if wList.Code != http.StatusOK {
		t.Fatalf("GET listar esperado 200, obtenido %d", wList.Code)
	}
	var listBody struct {
		Success bool             `json:"success"`
		Data    RHEListaResponse `json:"data"`
	}
	if err := json.Unmarshal(wList.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("JSON inválido en listar: %v", err)
	}
	if listBody.Data.Total != 2 || listBody.Data.TotalEmpresa != 3 {
		t.Fatalf("filtro ACTIVO inesperado: total=%d totalEmpresa=%d", listBody.Data.Total, listBody.Data.TotalEmpresa)
	}

	// 3. GET exportar XLSX y validar estructura OOXML + XML bien formado
	reqXLSX := httptest.NewRequest(http.MethodGet, "/api/empresas/20600000001/rhe/exportar?formato=xlsx&periodo=202610", nil)
	wXLSX := httptest.NewRecorder()
	r.ServeHTTP(wXLSX, reqXLSX)
	if wXLSX.Code != http.StatusOK {
		t.Fatalf("GET exportar xlsx esperado 200, obtenido %d: %s", wXLSX.Code, wXLSX.Body.String())
	}
	if !strings.Contains(wXLSX.Header().Get("Content-Disposition"), "RHE_Receptor_20600000001_202610.xlsx") {
		t.Fatalf("Content-Disposition inesperado: %s", wXLSX.Header().Get("Content-Disposition"))
	}

	zr, err := zip.NewReader(bytes.NewReader(wXLSX.Body.Bytes()), int64(wXLSX.Body.Len()))
	if err != nil {
		t.Fatalf("XLSX no es un archivo ZIP válido: %v", err)
	}
	seenSheet1 := false
	seenSheet2 := false
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("error abriendo entrada %s: %v", f.Name, err)
		}
		data, _ := io.ReadAll(rc)
		_ = rc.Close()

		dec := xml.NewDecoder(bytes.NewReader(data))
		for {
			_, tokErr := dec.Token()
			if tokErr == io.EOF {
				break
			}
			if tokErr != nil {
				t.Fatalf("XML mal formado en %s: %v", f.Name, tokErr)
			}
		}
		if f.Name == "xl/worksheets/sheet1.xml" {
			seenSheet1 = true
			if !strings.Contains(string(data), "10423496628") || !strings.Contains(string(data), "E001-567") {
				t.Fatalf("sheet1.xml no contiene los datos RHE esperados")
			}
		}
		if f.Name == "xl/worksheets/sheet2.xml" {
			seenSheet2 = true
			if !strings.Contains(string(data), "3626.09") {
				t.Fatalf("sheet2.xml no contiene el total activo esperado 3626.09")
			}
		}
	}
	if !seenSheet1 || !seenSheet2 {
		t.Fatalf("faltan hojas en XLSX: sheet1=%v sheet2=%v", seenSheet1, seenSheet2)
	}

	// 4. GET exportar TXT
	reqTXT := httptest.NewRequest(http.MethodGet, "/api/empresas/20600000001/rhe/exportar?formato=txt&periodo=202610", nil)
	wTXT := httptest.NewRecorder()
	r.ServeHTTP(wTXT, reqTXT)
	if wTXT.Code != http.StatusOK {
		t.Fatalf("GET exportar txt esperado 200, obtenido %d", wTXT.Code)
	}
	if !strings.Contains(wTXT.Body.String(), "E001-217") || !strings.Contains(wTXT.Body.String(), "|") {
		t.Fatalf("contenido TXT exportado inesperado: %s", wTXT.Body.String())
	}

	// 5. Error de CAPTCHA devuelve HTTP 429 con código SOL_CAPTCHA_REQUERIDO
	svc.SetProvider(&mockReceptorProvider{err: ErrSolCaptchaRequired})
	reqCap := httptest.NewRequest(http.MethodPost, "/api/empresas/20600000001/rhe/consultar", strings.NewReader(`{"periodo":"202610"}`))
	reqCap.Header.Set("Content-Type", "application/json")
	wCap := httptest.NewRecorder()
	r.ServeHTTP(wCap, reqCap)
	if wCap.Code != http.StatusTooManyRequests {
		t.Fatalf("CAPTCHA debe retornar 429, se obtuvo %d", wCap.Code)
	}
}
