package cpe

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SwodLore/sire-api/internal/database"
)

type mockCreds struct{}

func (m *mockCreds) GetDecryptedCredentials(ruc string) (string, string, string, string, string, error) {
	return ruc, "EMPRESA DEMO S.A.C.", "MODDATOS", "moddatos", "secret", nil
}

func TestParseSIREProposalArchive_RVIEandRCE(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "LE2000000000120241000140400021112.zip")

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	f, _ := zw.Create("LE2000000000120241000140400021112.txt")
	txt := "RUC|Razon Social|Periodo|CAR SUNAT|Fecha de emision|Fecha Vcto/Pago|Tipo CP/Doc.|Serie del CDP|Nro CP o Doc. Nro Inicial (Rango)|Nro Final|Tipo Doc Identidad|Nro Doc Identidad|Apellidos Nombres/ Razon Social|Valor Facturado Exportacion|BI Gravada|Dscto BI|IGV / IPM|Dscto IGV|Mto Exonerado|Mto Inafecto|ISC|BI Grav IVAP|IVAP|ICBPER|Total CP|Moneda\n" +
		"20000000001|EMPRESA DEMO S.A.C.|202410|2000000000101F0010000000001|15/10/2024||01|F001|00000001||6|20000000004|CLIENTE DEMO SAC|0.00|100.00|0.00|18.00|0.00|0.00|0.00|0.00|0.00|0.00|0.00|118.00|PEN\n"
	_, _ = f.Write([]byte(txt))
	_ = zw.Close()

	if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("error escribiendo zip: %v", err)
	}

	items, err := ParseSIREProposalArchive(zipPath, "20000000001", "VENTAS", "202410")
	if err != nil {
		t.Fatalf("error parseando archivo SIRE: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("se esperaba 1 comprobante, se obtuvo %d", len(items))
	}
	if items[0].TipoCP != "01" || items[0].Serie != "F001" || items[0].Numero != "00000001" {
		t.Fatalf("datos parseados incorrectos: %+v", items[0])
	}
	if items[0].MontoTotal != 118.00 || items[0].IGV != 18.00 || items[0].BaseImponible != 100.00 {
		t.Fatalf("importes incorrectos: base=%.2f igv=%.2f total=%.2f", items[0].BaseImponible, items[0].IGV, items[0].MontoTotal)
	}
	if items[0].RucContraparte != "20000000004" {
		t.Fatalf("ruc contraparte incorrecto: %s", items[0].RucContraparte)
	}
}

func TestParseUBLXML_AndImport_AndPDF_AndSOAPCDR(t *testing.T) {
	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("error init db: %v", err)
	}
	defer db.Close()

	storage := t.TempDir()
	repo := NewRepository(db)
	svc := NewService(repo, &mockCreds{}, nil, storage)

	// 1. Subir un XML UBL 2.1 de proveedor (Compra)
	sampleXML := `<?xml version="1.0" encoding="UTF-8"?>
<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
         xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"
         xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2">
  <cbc:ID>F001-00000456</cbc:ID>
  <cbc:IssueDate>2024-10-20</cbc:IssueDate>
  <cbc:InvoiceTypeCode>01</cbc:InvoiceTypeCode>
  <cbc:DocumentCurrencyCode>PEN</cbc:DocumentCurrencyCode>
  <cac:AccountingSupplierParty>
    <cac:Party>
      <cac:PartyIdentification><cbc:ID>20000000004</cbc:ID></cac:PartyIdentification>
      <cac:PartyLegalEntity><cbc:RegistrationName>PROVEEDOR DEMO S.A.C.</cbc:RegistrationName></cac:PartyLegalEntity>
    </cac:Party>
  </cac:AccountingSupplierParty>
  <cac:AccountingCustomerParty>
    <cac:Party>
      <cac:PartyIdentification><cbc:ID>20000000001</cbc:ID></cac:PartyIdentification>
      <cac:PartyLegalEntity><cbc:RegistrationName>EMPRESA DEMO S.A.C.</cbc:RegistrationName></cac:PartyLegalEntity>
    </cac:Party>
  </cac:AccountingCustomerParty>
  <cac:TaxTotal><cbc:TaxAmount>36.00</cbc:TaxAmount></cac:TaxTotal>
  <cac:LegalMonetaryTotal>
    <cbc:LineExtensionAmount>200.00</cbc:LineExtensionAmount>
    <cbc:PayableAmount>236.00</cbc:PayableAmount>
  </cac:LegalMonetaryTotal>
</Invoice>`

	impRes, err := svc.ImportUploadedFile("20000000001", "COMPRAS", "20000000004-01-F001-00000456.xml", []byte(sampleXML), 0)
	if err != nil {
		t.Fatalf("error importando XML: %v", err)
	}
	if impRes.ArchivosGuardados != 1 || len(impRes.Comprobantes) != 1 {
		t.Fatalf("importación inesperada: %+v", impRes)
	}
	comp := impRes.Comprobantes[0]
	if comp.Origen != "COMPRAS" || comp.Serie != "F001" || comp.Numero != "00000456" || !comp.TieneXML {
		t.Fatalf("comprobante importado inesperado: %+v", comp)
	}

	// 2. Descargar XML original
	xmlBytes, xmlName, xmlCT, _, err := svc.GetDownloadFile(comp.ID, "20000000001", "xml")
	if err != nil || len(xmlBytes) == 0 || !strings.HasSuffix(xmlName, ".xml") || xmlCT != "application/xml" {
		t.Fatalf("error descargando XML: err=%v name=%s ct=%s", err, xmlName, xmlCT)
	}

	// 3. Descargar PDF generado automáticamente desde XML UBL 2.1 (con Código QR y detalles)
	pdfBytes, pdfName, pdfCT, pdfSrc, err := svc.GetDownloadFile(comp.ID, "20000000001", "pdf")
	if err != nil || !bytes.HasPrefix(pdfBytes, []byte("%PDF-1.4")) || pdfCT != "application/pdf" || pdfSrc != "xml-ubl" {
		t.Fatalf("error generando PDF desde XML: err=%v name=%s ct=%s src=%s", err, pdfName, pdfCT, pdfSrc)
	}

	// 3.1 Descargar representación JSON estructurada desde XML UBL
	jsonBytes, jsonName, jsonCT, jsonSrc, err := svc.GetDownloadFile(comp.ID, "20000000001", "json")
	if err != nil || !strings.HasSuffix(jsonName, ".json") || !strings.Contains(jsonCT, "application/json") || jsonSrc != "xml-ubl" || !strings.Contains(string(jsonBytes), "PROVEEDOR DEMO S.A.C.") {
		t.Fatalf("error generando JSON desde XML: err=%v name=%s ct=%s src=%s", err, jsonName, jsonCT, jsonSrc)
	}

	// 4. Mock SOAP billConsultService (getStatusCdr) y descarga CDR
	dummyZip := []byte("PK\x03\x04dummy-cdr-zip-content")
	soapMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, _ = w.Write([]byte(`<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><br:getStatusCdrResponse xmlns:br="http://service.sunat.gob.pe"><statusCdr><content>` +
			base64.StdEncoding.EncodeToString(dummyZip) +
			`</content><statusCode>0004</statusCode><statusMessage>La constancia existe</statusMessage></statusCdr></br:getStatusCdrResponse></soap:Body></soap:Envelope>`))
	}))
	defer soapMock.Close()

	svc.SetSoapEndpoint(soapMock.URL)
	cdrRes, err := svc.ConsultarYGuardarCDR(comp.ID, "20000000001")
	if err != nil {
		t.Fatalf("error consultando CDR SOAP: %v", err)
	}
	if !cdrRes.CdrDescargado || cdrRes.StatusCode != "0004" || !cdrRes.Comprobante.TieneCDR {
		t.Fatalf("resultado CDR inesperado: %+v", cdrRes)
	}

	cdrBytes, cdrName, cdrCT, _, err := svc.GetDownloadFile(comp.ID, "20000000001", "cdr")
	if err != nil || !bytes.Equal(cdrBytes, dummyZip) || !strings.HasSuffix(cdrName, ".zip") || cdrCT != "application/zip" {
		t.Fatalf("error descargando CDR guardado: err=%v name=%s ct=%s", err, cdrName, cdrCT)
	}
}

func TestParseRealSunatRVIEAndRCE_SpannedZip_ModificadoColumns_AndMultiSupplierCompras(t *testing.T) {
	dir := t.TempDir()

	// 1. Archivo real RVIE (Ventas) con columnas "Doc Modificado" al final y prefijo ZIP SUNAT PK\x07\x08PK\x03\x04
	rvieBuf := new(bytes.Buffer)
	zw := zip.NewWriter(rvieBuf)
	f1, _ := zw.Create("LE200000000012026110014040001EXP2.txt")
	rvieTxt := "\xEF\xBB\xBFRuc|Razon Social|Periodo|CAR SUNAT|Fecha de emisión|Fecha Vcto/Pago|Tipo CP/Doc.|Serie del CDP|Nro CP o Doc. Nro Inicial (Rango)|Nro Final (Rango)|Tipo Doc Identidad|Nro Doc Identidad|Apellidos Nombres/ Razón Social|Valor Facturado Exportación|BI Gravada|Dscto BI|IGV / IPM|Dscto IGV / IPM|Mto Exonerado|Mto Inafecto|ISC|BI Grav IVAP|IVAP|ICBPER|Otros Tributos|Total CP|Moneda|Tipo Cambio|Fecha Emisión Doc Modificado|Tipo CP Modificado|Serie CP Modificado|Nro CP Modificado|ID Proyecto Operadores Atribución|Tipo de Nota|Est. Comp|Valor FOB Embarcado|Valor OP Gratuitas|Tipo Operación|DAM / CP|CLU\n" +
		"20000000001|EMPRESA DEMO S.A.C.|202611|2000000000101F0010000002808|01/11/2026||01|F001|2808||6|20000000002|CLIENTE DEMO UNO S.R.L.|0|24.58|0|4.42|0|0|0|0|0|0|0|0|29|PEN|1.000|||||||1|0|0|0101||\n"
	_, _ = f1.Write([]byte(rvieTxt))
	_ = zw.Close()

	rvieZipPath := filepath.Join(dir, "rvie_spanned.zip")
	if err := os.WriteFile(rvieZipPath, append([]byte("PK\x07\x08"), rvieBuf.Bytes()...), 0644); err != nil {
		t.Fatalf("error escribiendo rvie zip: %v", err)
	}

	ventasItems, err := ParseSIREProposalArchive(rvieZipPath, "20000000001", "VENTAS", "202611")
	if err != nil {
		t.Fatalf("error parseando RVIE real: %v", err)
	}
	if len(ventasItems) != 1 {
		t.Fatalf("se esperaba 1 venta parseada, obtenido %d", len(ventasItems))
	}
	v := ventasItems[0]
	if v.TipoCP != "01" || v.Serie != "F001" || v.Numero != "2808" || v.FechaEmision != "01/11/2026" || v.RucContraparte != "20000000002" || v.RazonContraparte != "CLIENTE DEMO UNO S.R.L." || v.MontoTotal != 29.0 || v.EstadoSUNAT != "ACTIVO" {
		t.Fatalf("venta mal parseada: %+v", v)
	}

	// 2. Archivo real RCE (Compras) con col 1 = razón social de empresa y col 13 = proveedor,
	//    más dos proveedores distintos con el MISMO número de factura 01-E001-146
	rceBuf := new(bytes.Buffer)
	zw2 := zip.NewWriter(rceBuf)
	f2, _ := zw2.Create("20000000001-20261119-1201-propuesta.txt")
	rceTxt := "RUC|Apellidos y Nombres o Razón social|Periodo|CAR SUNAT|Fecha de emisión|Fecha Vcto/Pago|Tipo CP/Doc.|Serie del CDP|Año|Nro CP o Doc. Nro Inicial (Rango)|Nro Final (Rango)|Tipo Doc Identidad|Nro Doc Identidad|Apellidos Nombres/ Razón  Social|BI Gravado DG|IGV / IPM DG|BI Gravado DGNG|IGV / IPM DGNG|BI Gravado DNG|IGV / IPM DNG|Valor Adq. NG|ISC|ICBPER|Otros Trib/ Cargos|Total CP|Moneda|Tipo de Cambio|Fecha Emisión Doc Modificado|Tipo CP Modificado|Serie CP Modificado|COD. DAM O DSI|Nro CP Modificado|Clasif de Bss y Sss|ID Proyecto Operadores|PorcPart|IMB|CAR Orig/ Ind E o I|Detracción|Tipo de Nota|Est. Comp.|Incal\n" +
		"20000000001|EMPRESA DEMO S.A.C.|202610|2000000000301E0010000000146|05/10/2026||01|E001||146||6|20000000003|PROVEEDOR DEMO TRES S.A.C.|152.54|27.46|0.00|0.00|0.00|0.00|0.00|0.00|0.00|0.00|180.00|PEN|1.000|||||||||0.00||||1|0\n" +
		"20000000001|EMPRESA DEMO S.A.C.|202610|1000000000201E0010000000146|06/10/2026||01|E001||146||6|10000000002|PROVEEDOR DOS EIRL|100.00|18.00|0.00|0.00|0.00|0.00|0.00|0.00|0.00|0.00|118.00|PEN|1.000|||||||||0.00||||1|0\n"
	_, _ = f2.Write([]byte(rceTxt))
	_ = zw2.Close()

	rceZipPath := filepath.Join(dir, "rce_spanned.zip")
	if err := os.WriteFile(rceZipPath, append([]byte("PK\x07\x08"), rceBuf.Bytes()...), 0644); err != nil {
		t.Fatalf("error escribiendo rce zip: %v", err)
	}

	comprasItems, err := ParseSIREProposalArchive(rceZipPath, "20000000001", "COMPRAS", "202610")
	if err != nil {
		t.Fatalf("error parseando RCE real: %v", err)
	}
	if len(comprasItems) != 2 {
		t.Fatalf("se esperaban 2 compras parseadas, obtenido %d", len(comprasItems))
	}
	if comprasItems[0].RazonContraparte != "PROVEEDOR DEMO TRES S.A.C." || comprasItems[0].RucContraparte != "20000000003" || comprasItems[0].MontoTotal != 180.0 {
		t.Fatalf("compra[0] mal parseada: %+v", comprasItems[0])
	}

	// Verificar que ambos proveedores con 01-E001-146 se persisten sin colisionar en SQLite
	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("error init db: %v", err)
	}
	defer db.Close()

	repo := NewRepository(db)
	n, err := repo.UpsertBatch(comprasItems)
	if err != nil || n != 2 {
		t.Fatalf("error UpsertBatch compras: n=%d err=%v", n, err)
	}
	listed, err := repo.List("20000000001", "COMPRAS", "202610", "")
	if err != nil || len(listed) != 2 {
		t.Fatalf("se esperaban 2 compras distintas en BD (sin colisión de serie E001-146), obtenido len=%d err=%v", len(listed), err)
	}
}

func TestStoragePathPortability_RelativeAbsoluteAndTraversal(t *testing.T) {
	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("error init db: %v", err)
	}
	defer db.Close()

	storage1 := t.TempDir()
	storage2 := t.TempDir()
	outsideDir := t.TempDir()

	repo := NewRepository(db)
	svc1 := NewService(repo, &mockCreds{}, nil, storage1)

	sampleXML := `<?xml version="1.0" encoding="UTF-8"?>
<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
         xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"
         xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2">
  <cbc:ID>F001-00000777</cbc:ID>
  <cbc:IssueDate>2026-10-20</cbc:IssueDate>
  <cbc:InvoiceTypeCode>01</cbc:InvoiceTypeCode>
  <cbc:DocumentCurrencyCode>PEN</cbc:DocumentCurrencyCode>
  <cac:AccountingSupplierParty><cac:Party><cac:PartyIdentification><cbc:ID>20000000004</cbc:ID></cac:PartyIdentification><cac:PartyLegalEntity><cbc:RegistrationName>PROVEEDOR PORTABLE SAC</cbc:RegistrationName></cac:PartyLegalEntity></cac:Party></cac:AccountingSupplierParty>
  <cac:AccountingCustomerParty><cac:Party><cac:PartyIdentification><cbc:ID>20000000001</cbc:ID></cac:PartyIdentification><cac:PartyLegalEntity><cbc:RegistrationName>EMPRESA DEMO S.A.C.</cbc:RegistrationName></cac:PartyLegalEntity></cac:Party></cac:AccountingCustomerParty>
  <cac:TaxTotal><cbc:TaxAmount>18.00</cbc:TaxAmount></cac:TaxTotal>
  <cac:LegalMonetaryTotal><cbc:LineExtensionAmount>100.00</cbc:LineExtensionAmount><cbc:PayableAmount>118.00</cbc:PayableAmount></cac:LegalMonetaryTotal>
</Invoice>`

	impRes, err := svc1.ImportUploadedFile("20000000001", "COMPRAS", "20000000004-01-F001-00000777.xml", []byte(sampleXML), 0)
	if err != nil || len(impRes.Comprobantes) != 1 {
		t.Fatalf("error importando XML: err=%v res=%+v", err, impRes)
	}
	comp := impRes.Comprobantes[0]
	if filepath.IsAbs(comp.RutaXML) {
		t.Fatalf("se esperaba RutaXML relativa portable, se obtuvo absoluta: %s", comp.RutaXML)
	}

	// Simular traslado de toda la carpeta de descargas a otro directorio (storage2)
	srcFile := filepath.Join(storage1, filepath.FromSlash(comp.RutaXML))
	dstFile := filepath.Join(storage2, filepath.FromSlash(comp.RutaXML))
	if err := os.MkdirAll(filepath.Dir(dstFile), 0755); err != nil {
		t.Fatalf("error mkdir dst: %v", err)
	}
	raw, err := os.ReadFile(srcFile)
	if err != nil {
		t.Fatalf("error leyendo srcFile: %v", err)
	}
	if err := os.WriteFile(dstFile, raw, 0644); err != nil {
		t.Fatalf("error copiando a storage2: %v", err)
	}

	svc2 := NewService(repo, &mockCreds{}, nil, storage2)
	gotXML, _, _, _, err := svc2.GetDownloadFile(comp.ID, "20000000001", "xml")
	if err != nil || len(gotXML) == 0 {
		t.Fatalf("el archivo relativo no sobrevivió al cambio de storageDir: err=%v", err)
	}

	// Compatibilidad con ruta absoluta dentro del storage actual
	if _, err := svc2.readSafeFile(dstFile); err != nil {
		t.Fatalf("readSafeFile debe seguir leyendo rutas absolutas dentro del storage actual: %v", err)
	}

	// Traversal y rutas fuera del storage deben ser bloqueados
	outsideFile := filepath.Join(outsideDir, "secreto.xml")
	_ = os.WriteFile(outsideFile, []byte("<secret/>"), 0644)
	for _, bad := range []string{"../secreto.xml", "20000000001/../../secreto.xml", outsideFile, "/etc/passwd"} {
		if _, err := svc2.readSafeFile(bad); err == nil {
			t.Fatalf("readSafeFile debió rechazar ruta insegura %q", bad)
		}
	}
}
