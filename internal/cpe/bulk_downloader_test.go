package cpe

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SwodLore/sire-api/internal/database"
)

type mockSolTokenProvider struct {
	calls        int32
	refreshCalls int32
	tok1         string
	tok2         string
}

func (m *mockSolTokenProvider) GetToken(ctx context.Context, ruc, usuarioSOL, claveSOL string, forceRefresh bool) (string, error) {
	atomic.AddInt32(&m.calls, 1)
	if forceRefresh {
		atomic.AddInt32(&m.refreshCalls, 1)
		return m.tok2, nil
	}
	return m.tok1, nil
}

func (m *mockSolTokenProvider) Invalidate(ruc string) {}

func buildFullSampleInvoiceXML(rucEmisor, razonEmisor, serie, numero string, numItems int) []byte {
	var lines strings.Builder
	for i := 1; i <= numItems; i++ {
		lines.WriteString(fmt.Sprintf(`
  <cac:InvoiceLine>
    <cbc:ID>%d</cbc:ID>
    <cbc:InvoicedQuantity unitCode="NIU">2</cbc:InvoicedQuantity>
    <cbc:LineExtensionAmount currencyID="PEN">100.00</cbc:LineExtensionAmount>
    <cac:PricingReference>
      <cac:AlternativeConditionPrice>
        <cbc:PriceAmount currencyID="PEN">59.00</cbc:PriceAmount>
        <cbc:PriceTypeCode>01</cbc:PriceTypeCode>
      </cac:AlternativeConditionPrice>
    </cac:PricingReference>
    <cac:TaxTotal>
      <cbc:TaxAmount currencyID="PEN">18.00</cbc:TaxAmount>
    </cac:TaxTotal>
    <cac:Item>
      <cbc:Description>SERVICIO DE CONSULTORIA TRIBUTARIA ITEM %d CON DESCRIPCION EXTENSA PARA PRUEBA</cbc:Description>
      <cac:SellersItemIdentification><cbc:ID>SKU-%03d</cbc:ID></cac:SellersItemIdentification>
    </cac:Item>
    <cac:Price><cbc:PriceAmount currencyID="PEN">50.00</cbc:PriceAmount></cac:Price>
  </cac:InvoiceLine>`, i, i, i))
	}

	totalBase := float64(numItems) * 100.0
	totalIGV := float64(numItems) * 18.0
	totalPay := totalBase + totalIGV

	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
         xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"
         xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
         xmlns:ext="urn:oasis:names:specification:ubl:schema:xsd:CommonExtensionComponents-2"
         xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
  <ext:UBLExtensions>
    <ext:UBLExtension>
      <ext:ExtensionContent>
        <ds:Signature>
          <ds:SignedInfo>
            <ds:Reference>
              <ds:DigestValue>AbCdEfGhIjKlMnOpQrStUvWxYz123456=</ds:DigestValue>
            </ds:Reference>
          </ds:SignedInfo>
        </ds:Signature>
      </ext:ExtensionContent>
    </ext:UBLExtension>
  </ext:UBLExtensions>
  <cbc:ID>%s-%s</cbc:ID>
  <cbc:IssueDate>2026-10-15</cbc:IssueDate>
  <cbc:IssueTime>14:22:10</cbc:IssueTime>
  <cbc:DueDate>2026-11-15</cbc:DueDate>
  <cbc:InvoiceTypeCode>01</cbc:InvoiceTypeCode>
  <cbc:Note languageLocaleID="1000">SON: CIENTO DIECIOCHO CON 00/100 SOLES</cbc:Note>
  <cbc:DocumentCurrencyCode>PEN</cbc:DocumentCurrencyCode>
  <cac:AccountingSupplierParty>
    <cac:Party>
      <cac:PartyIdentification><cbc:ID schemeID="6">%s</cbc:ID></cac:PartyIdentification>
      <cac:PartyLegalEntity>
        <cbc:RegistrationName>%s</cbc:RegistrationName>
        <cac:RegistrationAddress><cac:AddressLine><cbc:Line>AV. JAVIER PRADO ESTE 1234 - LIMA</cbc:Line></cac:AddressLine></cac:RegistrationAddress>
      </cac:PartyLegalEntity>
    </cac:Party>
  </cac:AccountingSupplierParty>
  <cac:AccountingCustomerParty>
    <cac:Party>
      <cac:PartyIdentification><cbc:ID schemeID="6">20000000001</cbc:ID></cac:PartyIdentification>
      <cac:PartyLegalEntity>
        <cbc:RegistrationName>EMPRESA DEMO S.A.C.</cbc:RegistrationName>
        <cac:RegistrationAddress><cac:AddressLine><cbc:Line>CALLE LOS CEDROS 456 - LIMA</cbc:Line></cac:AddressLine></cac:RegistrationAddress>
      </cac:PartyLegalEntity>
    </cac:Party>
  </cac:AccountingCustomerParty>
  <cac:PaymentTerms>
    <cbc:ID>FormaPago</cbc:ID>
    <cbc:PaymentMeansID>Credito</cbc:PaymentMeansID>
    <cbc:Amount currencyID="PEN">%.2f</cbc:Amount>
  </cac:PaymentTerms>
  <cac:PaymentTerms>
    <cbc:ID>FormaPago</cbc:ID>
    <cbc:PaymentMeansID>Cuota001</cbc:PaymentMeansID>
    <cbc:Amount currencyID="PEN">%.2f</cbc:Amount>
    <cbc:PaymentDueDate>2026-11-15</cbc:PaymentDueDate>
  </cac:PaymentTerms>
  <cac:TaxTotal>
    <cbc:TaxAmount currencyID="PEN">%.2f</cbc:TaxAmount>
    <cac:TaxSubtotal>
      <cbc:TaxableAmount currencyID="PEN">%.2f</cbc:TaxableAmount>
      <cbc:TaxAmount currencyID="PEN">%.2f</cbc:TaxAmount>
      <cac:TaxCategory><cac:TaxScheme><cbc:ID>1000</cbc:ID><cbc:Name>IGV</cbc:Name></cac:TaxScheme></cac:TaxCategory>
    </cac:TaxSubtotal>
  </cac:TaxTotal>
  <cac:LegalMonetaryTotal>
    <cbc:LineExtensionAmount currencyID="PEN">%.2f</cbc:LineExtensionAmount>
    <cbc:PayableAmount currencyID="PEN">%.2f</cbc:PayableAmount>
  </cac:LegalMonetaryTotal>%s
</Invoice>`, serie, numero, rucEmisor, razonEmisor, totalPay, totalPay, totalIGV, totalBase, totalIGV, totalBase, totalPay, lines.String()))
}

func makeZipBase64(files map[string][]byte) string {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, _ := zw.Create(name)
		_, _ = w.Write(content)
	}
	_ = zw.Close()
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestParseUBLXML_FullItemsCuotasHashQR_AndMultiPagePDF(t *testing.T) {
	xmlBytes := buildFullSampleInvoiceXML("20000000003", "PROVEEDOR DEMO TRES S.A.C.", "E001", "00000146", 32)
	meta, err := ParseUBLXML(xmlBytes)
	if err != nil {
		t.Fatalf("error parseando XML UBL completo: %v", err)
	}

	if meta.DigestValue != "AbCdEfGhIjKlMnOpQrStUvWxYz123456=" {
		t.Fatalf("DigestValue inesperado: %s", meta.DigestValue)
	}
	if meta.FormaPago != "Credito" || len(meta.Cuotas) != 1 || meta.Cuotas[0].ID != "Cuota001" {
		t.Fatalf("PaymentTerms/Cuotas inesperados: forma=%s cuotas=%+v", meta.FormaPago, meta.Cuotas)
	}
	if len(meta.Items) != 32 {
		t.Fatalf("se esperaban 32 items, obtenidos %d", len(meta.Items))
	}
	if meta.Items[0].Codigo != "SKU-001" || !strings.Contains(meta.Items[0].Descripcion, "CONSULTORIA TRIBUTARIA ITEM 1") {
		t.Fatalf("item[0] mal parseado: %+v", meta.Items[0])
	}
	if meta.DireccionEmisor == "" || meta.OpGravada != 3200.0 {
		t.Fatalf("dirección o gravada faltante: dir=%q gravada=%.2f", meta.DireccionEmisor, meta.OpGravada)
	}

	qrStr := BuildSunatQRString(meta, nil)
	wantPrefix := "20000000003|01|E001|00000146|576.00|3776.00|2026-10-15|6|20000000001|AbCdEfGhIjKlMnOpQrStUvWxYz123456=|"
	if qrStr != wantPrefix {
		t.Fatalf("cadena QR SUNAT incorrecta:\n got: %s\nwant: %s", qrStr, wantPrefix)
	}

	// Generar PDF multi-página con QR vectorial
	pdfBytes := GenerateComprobantePDFFromUBL(nil, meta, "EMPRESA DEMO S.A.C.")
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-1.4")) || !bytes.Contains(pdfBytes, []byte("%%EOF")) {
		t.Fatalf("PDF UBL inválido (len=%d)", len(pdfBytes))
	}
	// Con 32 ítems debe crear al menos 2 páginas (/Count 2 o superior)
	if !bytes.Contains(pdfBytes, []byte("/Count 2")) && !bytes.Contains(pdfBytes, []byte("/Count 3")) {
		t.Fatalf("se esperaba PDF multipágina para 32 ítems")
	}
	// Debe contener el Hash DigestValue y la descripción del primer ítem
	if !bytes.Contains(pdfBytes, []byte("AbCdEfGhIjKlMnOpQrStUvWxYz123456=")) {
		t.Fatalf("PDF UBL no incluye DigestValue")
	}

	// Verificar soporte de XML con declaración ISO-8859-1 (muy común en SEE-SOL)
	isoXML := append([]byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><Invoice xmlns:cbc=\"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2\" xmlns:cac=\"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2\"><cbc:ID>E001-99</cbc:ID><cbc:IssueDate>2026-10-10</cbc:IssueDate><cbc:InvoiceTypeCode>01</cbc:InvoiceTypeCode><cac:AccountingSupplierParty><cac:Party><cac:PartyIdentification><cbc:ID>20000000004</cbc:ID></cac:PartyIdentification><cac:PartyLegalEntity><cbc:RegistrationName>COMP"), 0xD1) // Ñ en Latin-1
	isoXML = append(isoXML, []byte("IA PERU S.A.</cbc:RegistrationName></cac:PartyLegalEntity></cac:Party></cac:AccountingSupplierParty></Invoice>")...)
	metaISO, err := ParseUBLXML(isoXML)
	if err != nil {
		t.Fatalf("error parseando XML ISO-8859-1: %v", err)
	}
	if !strings.Contains(metaISO.RazonEmisor, "Ñ") {
		t.Fatalf("decodificación ISO-8859-1 falló: %q", metaISO.RazonEmisor)
	}
}

func TestSunatCPEClient_ModesErrorsAndZipExtraction(t *testing.T) {
	sampleXML := buildFullSampleInvoiceXML("20000000003", "PROVEEDOR UNO SAC", "E001", "146", 2)
	sampleCDR := []byte(`<?xml version="1.0" encoding="UTF-8"?><ApplicationResponse><ResponseCode>0</ResponseCode></ApplicationResponse>`)
	zipB64 := makeZipBase64(map[string][]byte{
		"20000000003-01-E001-146.xml":   sampleXML,
		"R-20000000003-01-E001-146.xml": sampleCDR,
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer VALID_TOKEN" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/20000000003-01-E001-146-2/02":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"nomArchivo": "20000000003-01-E001-146.zip",
				"valArchivo": zipB64,
			})
		case "/20000000003-01-E001-146-2/01":
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"errors":[{"codError":"113","desError":"No hay resultados para la consulta"}]}`))
		case "/20000000001-01-F001-2808-2/02":
			// Simular error 105 (requiere modo -1 emitido)
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"errors":[{"codError":"105","desError":"El emisor debe ser el RUC autenticado"}]}`))
		case "/20000000001-01-F001-2808-1/02":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"nomArchivo": "20000000001-01-F001-2808.xml",
				"valArchivo": base64.StdEncoding.EncodeToString(sampleXML),
			})
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	client := NewSunatCPEClient()
	client.SetBaseURL(srv.URL)

	// 1. Descarga XML recibido (-2) con CDR embebido en el ZIP
	res, err := client.DownloadXML(context.Background(), "VALID_TOKEN", "20000000001", "20000000003", "01", "E001", "00000146")
	if err != nil {
		t.Fatalf("error descargando XML: %v", err)
	}
	if !bytes.Contains(res.Data, []byte("<Invoice")) || len(res.EmbeddedCDR) == 0 {
		t.Fatalf("extracción ZIP inesperada: xmlLen=%d cdrLen=%d", len(res.Data), len(res.EmbeddedCDR))
	}

	// 2. PDF no disponible (422/113) -> ErrComprobanteNoDisponible
	_, err = client.DownloadPDF(context.Background(), "VALID_TOKEN", "20000000001", "20000000003", "01", "E001", "146")
	if !errors.Is(err, ErrComprobanteNoDisponible) {
		t.Fatalf("se esperaba ErrComprobanteNoDisponible, obtenido: %v", err)
	}

	// 3. Error 105 -> cambio automático a modo -1 cuando no se conoce rucAuth
	res2, err := client.DownloadXML(context.Background(), "VALID_TOKEN", "", "20000000001", "01", "F001", "2808")
	if err != nil || !bytes.Contains(res2.Data, []byte("<Invoice")) {
		t.Fatalf("fallback modo 105 falló: err=%v", err)
	}

	// 4. Token inválido -> ErrSolTokenExpired
	_, err = client.DownloadXML(context.Background(), "EXPIRED", "20000000001", "20000000003", "01", "E001", "146")
	if !errors.Is(err, ErrSolTokenExpired) {
		t.Fatalf("se esperaba ErrSolTokenExpired, obtenido: %v", err)
	}
}

func TestBulkDownloader_EndToEnd_WithTokenRefresh_PDFFallback_ZIP_AndItemsCSV(t *testing.T) {
	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("error init db: %v", err)
	}
	defer db.Close()

	repo := NewRepository(db)
	svc := NewService(repo, &mockCreds{}, nil, t.TempDir())
	svc.SetBulkDelays(0, 0, 10*time.Millisecond)

	// Sembrar 3 comprobantes de COMPRAS para periodo 202610
	seed := []ComprobanteCPE{
		{
			EmpresaRUC: "20000000001", Origen: "COMPRAS", Periodo: "202610",
			TipoCP: "01", Serie: "F001", Numero: "00000101", FechaEmision: "15/10/2026",
			RucContraparte: "20000000003", RazonContraparte: "PROVEEDOR UNO S.A.C.",
			BaseImponible: 100, IGV: 18, MontoTotal: 118, Moneda: "PEN",
		},
		{
			EmpresaRUC: "20000000001", Origen: "COMPRAS", Periodo: "202610",
			TipoCP: "01", Serie: "E001", Numero: "146", FechaEmision: "16/10/2026",
			RucContraparte: "20000000004", RazonContraparte: "PROVEEDOR DOS S.A.C.",
			BaseImponible: 200, IGV: 36, MontoTotal: 236, Moneda: "PEN",
		},
		{
			EmpresaRUC: "20000000001", Origen: "COMPRAS", Periodo: "202610",
			TipoCP: "03", Serie: "B001", Numero: "55", FechaEmision: "17/10/2026",
			RucContraparte: "20000000004", RazonContraparte: "PROVEEDOR DOS S.A.C.",
			BaseImponible: 50, IGV: 9, MontoTotal: 59, Moneda: "PEN",
		},
	}
	if _, err := repo.UpsertBatch(seed); err != nil {
		t.Fatalf("error UpsertBatch: %v", err)
	}

	xml1 := buildFullSampleInvoiceXML("20000000003", "PROVEEDOR UNO S.A.C.", "F001", "00000101", 1)
	xml2 := buildFullSampleInvoiceXML("20000000004", "PROVEEDOR DOS S.A.C.", "E001", "146", 2)
	fakePDF := []byte("%PDF-1.4\n%official-sunat-pdf\n%%EOF\n")

	// Mock api-cpe: rechaza TOKEN_OLD con 401 una vez para probar auto-refresh hacia TOKEN_NEW
	cpeMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer TOKEN_NEW" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/20000000003-01-F001-101-2/02":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"nomArchivo": "20000000003-01-F001-101.zip",
				"valArchivo": makeZipBase64(map[string][]byte{"20000000003-01-F001-101.xml": xml1}),
			})
		case "/20000000003-01-F001-101-2/01":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"nomArchivo": "20000000003-01-F001-101.pdf",
				"valArchivo": base64.StdEncoding.EncodeToString(fakePDF),
			})
		case "/20000000004-01-E001-146-2/02":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"nomArchivo": "20000000004-01-E001-146.zip",
				"valArchivo": makeZipBase64(map[string][]byte{"20000000004-01-E001-146.xml": xml2}),
			})
		case "/20000000004-01-E001-146-2/01":
			// SUNAT no tiene PDF (SEE-Contribuyente) -> debe generarse automáticamente desde el XML!
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"errors":[{"codError":"113","desError":"PDF no disponible"}]}`))
		default:
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"errors":[{"codError":"113","desError":"No hay resultados"}]}`))
		}
	}))
	defer cpeMock.Close()

	// Mock SOAP CDR
	dummyCdrZip := []byte("PK\x03\x04mock-cdr-zip")
	soapMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, _ = w.Write([]byte(`<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><br:getStatusCdrResponse xmlns:br="http://service.sunat.gob.pe"><statusCdr><content>` +
			base64.StdEncoding.EncodeToString(dummyCdrZip) +
			`</content><statusCode>0004</statusCode><statusMessage>La constancia existe</statusMessage></statusCdr></br:getStatusCdrResponse></soap:Body></soap:Envelope>`))
	}))
	defer soapMock.Close()

	tokProvider := &mockSolTokenProvider{tok1: "TOKEN_OLD", tok2: "TOKEN_NEW"}
	svc.SetSolTokenProvider(tokProvider)
	svc.SetSunatCPEBaseURL(cpeMock.URL)
	svc.SetSoapEndpoint(soapMock.URL)

	job, err := svc.StartBulkDownload("20000000001", BulkDownloadRequest{
		Origen:   "COMPRAS",
		Periodo:  "202610",
		Formatos: []string{"xml", "pdf", "cdr", "json"},
		Workers:  2,
	})
	if err != nil {
		t.Fatalf("error iniciando descarga masiva: %v", err)
	}

	// Esperar finalización del lote
	deadline := time.Now().Add(5 * time.Second)
	var finalJob *BulkDownloadJob
	for time.Now().Before(deadline) {
		j, gErr := svc.GetBulkJob("20000000001", job.ID)
		if gErr == nil && (j.Estado == "COMPLETADO" || j.Estado == "ERROR") {
			finalJob = j
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if finalJob == nil || finalJob.Estado != "COMPLETADO" {
		t.Fatalf("el lote no completó a tiempo: %+v", finalJob)
	}

	if atomic.LoadInt32(&tokProvider.refreshCalls) == 0 {
		t.Fatalf("se esperaba renovación automática de token tras 401")
	}
	if finalJob.XmlDescargados != 2 || finalJob.PdfDescargados != 1 || finalJob.PdfGenerados != 1 || finalJob.CdrDescargados != 3 {
		t.Fatalf("contadores del lote inesperados: %+v", finalJob)
	}
	if !finalJob.TieneZIP {
		t.Fatalf("se esperaba ZIP consolidado listo")
	}

	// Inspeccionar contenido del ZIP consolidado
	zipBytes, zipName, err := svc.GetBulkZip("20000000001", finalJob.ID)
	if err != nil || !strings.HasSuffix(zipName, ".zip") || len(zipBytes) == 0 {
		t.Fatalf("error obteniendo ZIP del lote: err=%v name=%s", err, zipName)
	}

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("ZIP consolidado corrupto: %v", err)
	}
	entries := make(map[string]bool)
	for _, f := range zr.File {
		entries[f.Name] = true
	}
	required := []string{
		"XML/20000000003-01-F001-00000101.xml",
		"XML/20000000004-01-E001-146.xml",
		"PDF/20000000003-01-F001-00000101.pdf",
		"PDF/20000000004-01-E001-146.pdf",
		"JSON/20000000003-01-F001-00000101.json",
		"resumen_comprobantes.csv",
		"resumen_items.csv",
	}
	for _, reqEntry := range required {
		if !entries[reqEntry] {
			t.Fatalf("falta archivo esperado en ZIP: %s (presentes: %v)", reqEntry, entries)
		}
	}

	// Probar ExportItemsCSV
	csvBytes, csvName, err := svc.ExportItemsCSV("20000000001", "COMPRAS", "202610", "")
	if err != nil || !strings.HasSuffix(csvName, ".csv") {
		t.Fatalf("error en ExportItemsCSV: %v", err)
	}
	csvStr := string(csvBytes)
	if !strings.Contains(csvStr, "SKU-001") || !strings.Contains(csvStr, "CONSULTORIA TRIBUTARIA ITEM 2") {
		t.Fatalf("CSV de ítems no contiene descripciones extraídas de XML:\n%s", csvStr)
	}

	// Verificar concordancia SIRE y resumen de ítems guardados en SQLite
	listed, err := svc.ListFiltered("20000000001", "COMPRAS", "202610", "FACTURAS_NOTAS", "", true)
	if err != nil || len(listed) != 2 {
		t.Fatalf("ListFiltered FACTURAS_NOTAS inesperado: len=%d err=%v", len(listed), err)
	}
	for _, it := range listed {
		if it.ConcordanciaSIRE != "CONCORDANTE" {
			t.Fatalf("se esperaba CONCORDANTE para %s-%s, obtenido %s (%s)", it.Serie, it.Numero, it.ConcordanciaSIRE, it.DetalleConcordancia)
		}
		if !strings.Contains(it.ResumenItems, "CONSULTORIA TRIBUTARIA") {
			t.Fatalf("resumen_items vacío o inesperado para %s-%s: %q", it.Serie, it.Numero, it.ResumenItems)
		}
	}

	// Verificar que una boleta sin archivo en SUNAT (113) NO devuelve una ficha SIRE falsa cuando allowSireFallback=false
	boletas, _ := svc.ListFiltered("20000000001", "COMPRAS", "202610", "03", "", true)
	if len(boletas) != 1 || boletas[0].ConcordanciaSIRE != "NO_EN_SUNAT" {
		t.Fatalf("boleta 03 esperada con NO_EN_SUNAT: %+v", boletas)
	}
	if _, _, _, _, err := svc.GetDownloadFileWithContext(context.Background(), boletas[0].ID, "20000000001", "pdf", false, false); err == nil {
		t.Fatalf("se esperaba error claro cuando no hay PDF/XML real y allowSireFallback=false")
	}

	// Verificar SyncProposalSnapshot: si la propuesta vigente ya solo tiene 1 factura, la otra pasa a vigente_sire=false
	vig, fuera, err := repo.SyncProposalSnapshot("20000000001", "COMPRAS", "202610", seed[:1])
	if err != nil || vig != 1 || fuera != 2 {
		t.Fatalf("SyncProposalSnapshot inesperado: vig=%d fuera=%d err=%v", vig, fuera, err)
	}
	onlyVig, err := svc.ListFiltered("20000000001", "COMPRAS", "202610", "", "", true)
	if err != nil || len(onlyVig) != 1 || onlyVig[0].Serie != "F001" {
		t.Fatalf("filtro soloVigentes falló: %+v", onlyVig)
	}
}
