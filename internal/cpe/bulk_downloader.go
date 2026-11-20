package cpe

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type activeJob struct {
	mu     sync.RWMutex
	data   BulkDownloadJob
	cancel context.CancelFunc
}

func (aj *activeJob) snapshot() BulkDownloadJob {
	aj.mu.RLock()
	defer aj.mu.RUnlock()
	cp := aj.data
	if len(aj.data.Formatos) > 0 {
		cp.Formatos = append([]string(nil), aj.data.Formatos...)
	}
	if len(aj.data.IDsFallidos) > 0 {
		cp.IDsFallidos = append([]int64(nil), aj.data.IDsFallidos...)
	}
	if len(aj.data.DetalleErrores) > 0 {
		cp.DetalleErrores = append([]BulkDownloadItemResult(nil), aj.data.DetalleErrores...)
	}
	return cp
}

// BulkDownloader coordina descargas masivas asíncronas de XML, PDF, CDR y JSON desde SUNAT SOL
// utilizando un pool controlado de workers HTTP sobre api-cpe.sunat.gob.pe + empaquetado ZIP.
type BulkDownloader struct {
	svc        *Service
	mu         sync.RWMutex
	jobs       map[string]*activeJob
	jitterMin  time.Duration
	jitterMax  time.Duration
	retryDelay time.Duration
}

func NewBulkDownloader(svc *Service) *BulkDownloader {
	return &BulkDownloader{
		svc:        svc,
		jobs:       make(map[string]*activeJob),
		jitterMin:  200 * time.Millisecond,
		jitterMax:  450 * time.Millisecond,
		retryDelay: 1200 * time.Millisecond,
	}
}

// SetDelays permite ajustar pausas y tiempos de reintento (útil para pruebas rápidas).
func (b *BulkDownloader) SetDelays(jitterMin, jitterMax, retryDelay time.Duration) {
	b.jitterMin = jitterMin
	b.jitterMax = jitterMax
	b.retryDelay = retryDelay
}

// StartJob crea y arranca un trabajo asíncrono de descarga masiva para el RUC indicado.
func (b *BulkDownloader) StartJob(ruc string, req BulkDownloadRequest) (*BulkDownloadJob, error) {
	if b.svc.creds == nil {
		return nil, errors.New("proveedor de credenciales no configurado")
	}

	origen := strings.ToUpper(strings.TrimSpace(req.Origen))
	if origen == "" {
		origen = "TODOS"
	}
	periodo := strings.TrimSpace(req.Periodo)
	soloVigentes := true
	if req.SoloVigentesSIRE != nil {
		soloVigentes = *req.SoloVigentesSIRE
	}

	items, err := b.svc.repo.ListForBulk(ruc, origen, periodo, req.TiposCP, soloVigentes, req.IDs)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errors.New("no se encontraron comprobantes vigentes del SIRE para el filtro indicado; primero sincronice una propuesta SIRE o ajuste el filtro de tipo de comprobante")
	}

	formatos := normalizeFormatos(req.Formatos)
	soloPendientes := true
	if req.SoloPendientes != nil {
		soloPendientes = *req.SoloPendientes
	}
	genPDF := true
	if req.GenerarPDFDesdeXML != nil {
		genPDF = *req.GenerarPDFDesdeXML
	}
	workers := req.Workers
	if workers <= 0 {
		workers = 3
	}
	if workers > 5 {
		workers = 5
	}

	now := time.Now()
	jobID := "cpe-" + uuid.New().String()[:8]
	ctx, cancel := context.WithCancel(context.Background())

	aj := &activeJob{
		cancel: cancel,
		data: BulkDownloadJob{
			ID:            jobID,
			EmpresaRUC:    ruc,
			Origen:        origen,
			Periodo:       periodo,
			Estado:        "PENDIENTE",
			Mensaje:       fmt.Sprintf("Encolando %d comprobante(s)...", len(items)),
			Formatos:      formatos,
			Total:         len(items),
			IniciadoEn:    now,
			ActualizadoEn: now,
		},
	}

	b.mu.Lock()
	b.jobs[jobID] = aj
	b.mu.Unlock()

	go b.runJob(ctx, aj, items, formatos, soloPendientes, genPDF, workers)

	snap := aj.snapshot()
	return &snap, nil
}

func (b *BulkDownloader) GetJob(ruc, jobID string) (*BulkDownloadJob, error) {
	b.mu.RLock()
	aj, ok := b.jobs[jobID]
	b.mu.RUnlock()
	if !ok {
		return nil, errors.New("lote de descarga masiva no encontrado")
	}
	snap := aj.snapshot()
	if snap.EmpresaRUC != ruc {
		return nil, errors.New("el lote no pertenece a la empresa activa")
	}
	return &snap, nil
}

func (b *BulkDownloader) ListJobs(ruc string) []BulkDownloadJob {
	b.mu.RLock()
	list := make([]BulkDownloadJob, 0)
	for _, aj := range b.jobs {
		snap := aj.snapshot()
		if snap.EmpresaRUC == ruc {
			list = append(list, snap)
		}
	}
	b.mu.RUnlock()

	sort.Slice(list, func(i, j int) bool {
		return list[i].IniciadoEn.After(list[j].IniciadoEn)
	})
	return list
}

func (b *BulkDownloader) CancelJob(ruc, jobID string) (*BulkDownloadJob, error) {
	b.mu.RLock()
	aj, ok := b.jobs[jobID]
	b.mu.RUnlock()
	if !ok {
		return nil, errors.New("lote de descarga masiva no encontrado")
	}
	aj.mu.Lock()
	if aj.data.EmpresaRUC != ruc {
		aj.mu.Unlock()
		return nil, errors.New("el lote no pertenece a la empresa activa")
	}
	if aj.data.Estado == "PENDIENTE" || aj.data.Estado == "AUTENTICANDO" || aj.data.Estado == "EN_PROCESO" {
		if aj.cancel != nil {
			aj.cancel()
		}
		now := time.Now()
		aj.data.Estado = "CANCELADO"
		aj.data.Mensaje = "Descarga masiva cancelada por el usuario."
		aj.data.ActualizadoEn = now
		aj.data.FinalizadoEn = &now
	}
	aj.mu.Unlock()
	snap := aj.snapshot()
	return &snap, nil
}

func (b *BulkDownloader) runJob(
	ctx context.Context,
	aj *activeJob,
	items []ComprobanteCPE,
	formatos []string,
	soloPendientes bool,
	genPDF bool,
	workers int,
) {
	defer func() {
		if r := recover(); r != nil {
			now := time.Now()
			aj.mu.Lock()
			aj.data.Estado = "ERROR"
			aj.data.Mensaje = fmt.Sprintf("Error inesperado en descarga masiva: %v", r)
			aj.data.ActualizadoEn = now
			aj.data.FinalizadoEn = &now
			aj.mu.Unlock()
		}
	}()

	ruc := aj.data.EmpresaRUC
	wantXML := containsStr(formatos, "xml")
	wantPDF := containsStr(formatos, "pdf")
	wantCDR := containsStr(formatos, "cdr")
	wantJSON := containsStr(formatos, "json")

	_, razonEmpresa, usuarioSOL, claveSOL, _, err := b.svc.creds.GetDecryptedCredentials(ruc)
	if err != nil {
		b.failJob(aj, fmt.Sprintf("No se pudieron obtener credenciales SOL: %v", err))
		return
	}
	if razonEmpresa == "" {
		razonEmpresa = "EMPRESA RUC " + ruc
	}

	// Determinar si hace falta token SOL (para descargar XML, PDF o CDR desde api-cpe.sunat.gob.pe)
	needsSolToken := false
	if wantXML || wantPDF || wantCDR {
		for _, it := range items {
			if wantXML && (!soloPendientes || !it.TieneXML || !b.svc.safeFileExists(it.RutaXML)) {
				needsSolToken = true
				break
			}
			if wantPDF && (!soloPendientes || !it.TienePDF || !b.svc.safeFileExists(it.RutaPDF)) {
				needsSolToken = true
				break
			}
			if wantCDR && (!soloPendientes || !it.TieneCDR || !b.svc.safeFileExists(it.RutaCDR)) {
				needsSolToken = true
				break
			}
		}
	}

	var tokenMu sync.RWMutex
	var solToken string
	if needsSolToken {
		b.updateJobStatus(aj, "AUTENTICANDO", "Iniciando sesión SOL (1 sola vez) para obtener token JWT de SUNAT...")
		tok, tokErr := b.svc.solTokens.GetToken(ctx, ruc, usuarioSOL, claveSOL, false)
		if tokErr != nil {
			b.failJob(aj, fmt.Sprintf("No se pudo autenticar en SUNAT SOL: %v", tokErr))
			return
		}
		solToken = tok
	}

	getToken := func() string {
		tokenMu.RLock()
		defer tokenMu.RUnlock()
		return solToken
	}
	refreshToken := func() (string, error) {
		tokenMu.Lock()
		defer tokenMu.Unlock()
		tok, rErr := b.svc.solTokens.GetToken(ctx, ruc, usuarioSOL, claveSOL, true)
		if rErr != nil {
			return "", rErr
		}
		solToken = tok
		return tok, nil
	}

	b.updateJobStatus(aj, "EN_PROCESO", fmt.Sprintf("Descargando %d comprobante(s) con %d workers en paralelo...", len(items), workers))

	// Canal de trabajos y pool de workers
	ch := make(chan ComprobanteCPE, len(items))
	for _, it := range items {
		ch <- it
	}
	close(ch)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)*997))
			for item := range ch {
				if ctx.Err() != nil {
					return
				}

				res := b.processOneItem(ctx, &item, razonEmpresa, usuarioSOL, claveSOL, wantXML, wantPDF, wantCDR, soloPendientes, genPDF, getToken, refreshToken)

				aj.mu.Lock()
				aj.data.Procesados++
				hadSuccess := false
				hadError := false
				hadNotAvail := false

				switch res.XML {
				case "ok":
					aj.data.XmlDescargados++
					hadSuccess = true
				case "omitido":
					hadSuccess = true
				case "no_disponible":
					hadNotAvail = true
				case "error":
					hadError = true
				}
				switch res.PDF {
				case "ok":
					aj.data.PdfDescargados++
					hadSuccess = true
				case "generado_xml":
					aj.data.PdfGenerados++
					hadSuccess = true
				case "omitido":
					hadSuccess = true
				case "no_disponible":
					hadNotAvail = true
				case "error":
					hadError = true
				}
				switch res.CDR {
				case "ok":
					aj.data.CdrDescargados++
					hadSuccess = true
				case "omitido":
					hadSuccess = true
				}

				if hadSuccess {
					aj.data.Exitosos++
				} else if hadError {
					aj.data.Errores++
					aj.data.IDsFallidos = append(aj.data.IDsFallidos, item.ID)
					if len(aj.data.DetalleErrores) < 100 {
						aj.data.DetalleErrores = append(aj.data.DetalleErrores, res)
					}
				} else if hadNotAvail {
					aj.data.NoDisponibles++
					if len(aj.data.DetalleErrores) < 100 {
						aj.data.DetalleErrores = append(aj.data.DetalleErrores, res)
					}
				} else {
					aj.data.Exitosos++
				}

				aj.data.Mensaje = fmt.Sprintf("Procesados %d/%d | XML: %d | PDF: %d (+%d desde XML) | CDR: %d",
					aj.data.Procesados, aj.data.Total, aj.data.XmlDescargados, aj.data.PdfDescargados, aj.data.PdfGenerados, aj.data.CdrDescargados)
				aj.data.ActualizadoEn = time.Now()
				aj.mu.Unlock()

				// Pausa responsable (jitter) para no saturar SUNAT
				if b.jitterMax > 0 && ctx.Err() == nil {
					span := b.jitterMax - b.jitterMin
					wait := b.jitterMin
					if span > 0 {
						wait += time.Duration(rng.Int63n(int64(span)))
					}
					select {
					case <-ctx.Done():
						return
					case <-time.After(wait):
					}
				}
			}
		}(w)
	}

	wg.Wait()

	if ctx.Err() != nil {
		now := time.Now()
		aj.mu.Lock()
		if aj.data.Estado != "CANCELADO" {
			aj.data.Estado = "CANCELADO"
			aj.data.Mensaje = fmt.Sprintf("Cancelado tras procesar %d/%d comprobantes.", aj.data.Procesados, aj.data.Total)
		}
		aj.data.ActualizadoEn = now
		aj.data.FinalizadoEn = &now
		aj.mu.Unlock()
		return
	}

	// Empaquetar ZIP consolidado del lote
	b.updateJobStatus(aj, "EMPAQUETANDO", "Empaquetando archivos en ZIP consolidado (XML / PDF / CDR / JSON / CSV de ítems)...")
	zipPath, zipName, packErr := b.packageBulkZIP(ruc, aj.data.Origen, aj.data.Periodo, aj.data.ID, items, razonEmpresa, wantXML, wantPDF, wantCDR, wantJSON)

	now := time.Now()
	aj.mu.Lock()
	if packErr != nil {
		aj.data.Estado = "COMPLETADO"
		aj.data.Mensaje = fmt.Sprintf("Descarga finalizada (%d/%d), pero hubo un error empaquetando el ZIP: %v", aj.data.Procesados, aj.data.Total, packErr)
	} else {
		aj.data.Estado = "COMPLETADO"
		aj.data.RutaZIP = zipPath
		aj.data.NombreZIP = zipName
		aj.data.TieneZIP = true
		aj.data.Mensaje = fmt.Sprintf("Lote completado: %d/%d procesados | XML: %d | PDF: %d (+%d desde XML) | CDR: %d | No disp.: %d | Errores: %d",
			aj.data.Procesados, aj.data.Total, aj.data.XmlDescargados, aj.data.PdfDescargados, aj.data.PdfGenerados, aj.data.CdrDescargados, aj.data.NoDisponibles, aj.data.Errores)
	}
	aj.data.ActualizadoEn = now
	aj.data.FinalizadoEn = &now
	aj.mu.Unlock()

	log.Printf("[CPE-BULK] Job %s finalizado para RUC %s: %s", aj.data.ID, ruc, aj.data.Mensaje)
}

func (b *BulkDownloader) processOneItem(
	ctx context.Context,
	item *ComprobanteCPE,
	razonEmpresa, usuarioSOL, claveSOL string,
	wantXML, wantPDF, wantCDR, soloPendientes, genPDF bool,
	getToken func() string,
	refreshToken func() (string, error),
) BulkDownloadItemResult {
	return b.processOneItemWithAttempts(ctx, item, razonEmpresa, usuarioSOL, claveSOL, wantXML, wantPDF, wantCDR, soloPendientes, genPDF, 5, getToken, refreshToken)
}

func isFatalSolAuthError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrSolCaptchaRequired) ||
		errors.Is(err, ErrSolAuthRejected) ||
		errors.Is(err, ErrSolCooldown) ||
		errors.Is(err, ErrSolBrowserMissing) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}

func mergeItemError(current, next error) error {
	if next == nil {
		return current
	}
	if current == nil {
		return next
	}
	if isFatalSolAuthError(next) && !isFatalSolAuthError(current) {
		return next
	}
	if errors.Is(current, ErrComprobanteNoDisponible) && !errors.Is(next, ErrComprobanteNoDisponible) {
		return next
	}
	return current
}

func (b *BulkDownloader) processOneItemWithAttempts(
	ctx context.Context,
	item *ComprobanteCPE,
	razonEmpresa, usuarioSOL, claveSOL string,
	wantXML, wantPDF, wantCDR, soloPendientes, genPDF bool,
	maxAttempts int,
	getToken func() string,
	refreshToken func() (string, error),
) BulkDownloadItemResult {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	rucEmisor := item.EmpresaRUC
	if strings.ToUpper(item.Origen) == "COMPRAS" && len(item.RucContraparte) == 11 {
		rucEmisor = item.RucContraparte
	}

	llave := fmt.Sprintf("%s-%s-%s", item.TipoCP, item.Serie, item.Numero)
	res := BulkDownloadItemResult{
		ID:          item.ID,
		Comprobante: llave,
		RucEmisor:   rucEmisor,
	}

	var xmlBytes []byte
	if item.TieneXML && b.svc.safeFileExists(item.RutaXML) {
		xmlBytes, _ = b.svc.readSafeFile(item.RutaXML)
	}

	// 1. Descargar XML desde api-cpe.sunat.gob.pe (sufijo 02)
	if wantXML {
		if soloPendientes && len(xmlBytes) > 0 {
			res.XML = "omitido"
		} else {
			dlRes, err := b.downloadWithRetryAttempts(ctx, "xml", item.EmpresaRUC, rucEmisor, item.TipoCP, item.Serie, item.Numero, maxAttempts, getToken, refreshToken)
			if err == nil && dlRes != nil && len(dlRes.Data) > 0 {
				xmlName := fmt.Sprintf("%s-%s-%s-%s.xml", rucEmisor, padTipoCP(item.TipoCP), item.Serie, item.Numero)
				if path, hashHex, wErr := b.svc.saveFile(item.EmpresaRUC, item.Origen, xmlName, dlRes.Data); wErr == nil {
					_ = b.svc.repo.UpdateFilePath(item.ID, item.EmpresaRUC, "xml", path, "", hashHex)
					item.RutaXML = path
					item.TieneXML = true
					xmlBytes = dlRes.Data
					res.XML = "ok"
					// Validar concordancia real XML vs propuesta SIRE y guardar resumen de ítems
					if meta, pErr := ParseUBLXML(xmlBytes); pErr == nil && meta != nil && !meta.EsCDR {
						conc, det, sumItems := EvaluateConcordanceSIRE(item, meta)
						_ = b.svc.repo.UpdateConcordancia(item.ID, item.EmpresaRUC, conc, det, sumItems)
						item.ConcordanciaSIRE = conc
						item.DetalleConcordancia = det
						item.ResumenItems = sumItems
					}
				} else {
					res.XML = "error"
					res.Error = appendErr(res.Error, "XML disco: "+wErr.Error())
					res.rawErr = mergeItemError(res.rawErr, wErr)
				}
				// Bonus: si el ZIP de SUNAT ya traía el CDR embebido, guardarlo también
				if len(dlRes.EmbeddedCDR) > 0 && (!item.TieneCDR || !b.svc.safeFileExists(item.RutaCDR)) {
					cdrName := dlRes.CDRFileName
					if cdrName == "" {
						cdrName = fmt.Sprintf("R-%s-%s-%s-%s.xml", rucEmisor, padTipoCP(item.TipoCP), item.Serie, item.Numero)
					}
					if cdrPath, cdrHash, cErr := b.svc.saveFile(item.EmpresaRUC, item.Origen, cdrName, dlRes.EmbeddedCDR); cErr == nil {
						_ = b.svc.repo.UpdateFilePath(item.ID, item.EmpresaRUC, "cdr", cdrPath, "CDR_ACEPTADO", cdrHash)
						item.RutaCDR = cdrPath
						item.TieneCDR = true
						res.CDR = "ok"
					}
				}
			} else if errors.Is(err, ErrComprobanteNoDisponible) {
				res.XML = "no_disponible"
				res.Error = appendErr(res.Error, "XML: "+err.Error())
				res.rawErr = mergeItemError(res.rawErr, err)
				_ = b.svc.repo.UpdateConcordancia(item.ID, item.EmpresaRUC, "NO_EN_SUNAT", "SUNAT no expone XML individual en Consulta CPE (común en Boletas 03 por Resumen Diario)", "")
			} else if err != nil {
				res.XML = "error"
				res.Error = appendErr(res.Error, "XML: "+err.Error())
				res.rawErr = mergeItemError(res.rawErr, err)
				if isFatalSolAuthError(err) {
					return res
				}
			}
		}
	}

	// 2. Descargar PDF original desde api-cpe.sunat.gob.pe (sufijo 01) o generar desde XML UBL 2.1
	if wantPDF {
		if soloPendientes && item.TienePDF && b.svc.safeFileExists(item.RutaPDF) {
			res.PDF = "omitido"
		} else {
			dlRes, err := b.downloadWithRetryAttempts(ctx, "pdf", item.EmpresaRUC, rucEmisor, item.TipoCP, item.Serie, item.Numero, maxAttempts, getToken, refreshToken)
			if err == nil && dlRes != nil && len(dlRes.Data) > 0 {
				pdfName := fmt.Sprintf("%s-%s-%s-%s.pdf", rucEmisor, padTipoCP(item.TipoCP), item.Serie, item.Numero)
				if path, hashHex, wErr := b.svc.saveFile(item.EmpresaRUC, item.Origen, pdfName, dlRes.Data); wErr == nil {
					_ = b.svc.repo.UpdateFilePath(item.ID, item.EmpresaRUC, "pdf", path, "", hashHex)
					item.RutaPDF = path
					item.TienePDF = true
					res.PDF = "ok"
				} else {
					res.PDF = "error"
					res.Error = appendErr(res.Error, "PDF disco: "+wErr.Error())
					res.rawErr = mergeItemError(res.rawErr, wErr)
				}
			} else if genPDF && len(xmlBytes) > 0 {
				// Respaldo profesional costo 0: generar PDF UBL 2.1 con QR SUNAT y detalle de ítems desde el XML real
				if meta, pErr := ParseUBLXML(xmlBytes); pErr == nil && meta != nil {
					pdfBytes := GenerateComprobantePDFFromUBL(item, meta, razonEmpresa)
					pdfName := fmt.Sprintf("%s-%s-%s-%s.pdf", rucEmisor, padTipoCP(item.TipoCP), item.Serie, item.Numero)
					if path, hashHex, wErr := b.svc.saveFile(item.EmpresaRUC, item.Origen, pdfName, pdfBytes); wErr == nil {
						_ = b.svc.repo.UpdateFilePath(item.ID, item.EmpresaRUC, "pdf", path, "", hashHex)
						item.RutaPDF = path
						item.TienePDF = true
						res.PDF = "generado_xml"
					}
				}
			} else if errors.Is(err, ErrComprobanteNoDisponible) {
				res.PDF = "no_disponible"
				res.rawErr = mergeItemError(res.rawErr, err)
			} else if err != nil {
				res.PDF = "error"
				res.Error = appendErr(res.Error, "PDF: "+err.Error())
				res.rawErr = mergeItemError(res.rawErr, err)
				if isFatalSolAuthError(err) {
					return res
				}
			}
		}
	}

	// 3. Descargar CDR oficial vía api-cpe.sunat.gob.pe (/03 — funciona para COMPRAS y VENTAS), con respaldo SOAP billConsultService
	if wantCDR && res.CDR == "" {
		if soloPendientes && item.TieneCDR && b.svc.safeFileExists(item.RutaCDR) {
			res.CDR = "omitido"
		} else {
			cdrSaved := false
			var cdrRestErr error
			if getToken != nil && getToken() != "" {
				if dlCdr, cErr := b.downloadWithRetryAttempts(ctx, "cdr", item.EmpresaRUC, rucEmisor, item.TipoCP, item.Serie, item.Numero, maxAttempts, getToken, refreshToken); cErr == nil && dlCdr != nil && len(dlCdr.Data) > 0 {
					ext := ".zip"
					if !isZipBytes(dlCdr.Data) {
						ext = ".xml"
					}
					cdrName := fmt.Sprintf("R-%s-%s-%s-%s%s", rucEmisor, padTipoCP(item.TipoCP), item.Serie, item.Numero, ext)
					if path, hashHex, wErr := b.svc.saveFile(item.EmpresaRUC, item.Origen, cdrName, dlCdr.Data); wErr == nil {
						_ = b.svc.repo.UpdateFilePath(item.ID, item.EmpresaRUC, "cdr", path, "CDR_ACEPTADO", hashHex)
						item.RutaCDR = path
						item.TieneCDR = true
						res.CDR = "ok"
						cdrSaved = true
					}
				} else if cErr != nil {
					cdrRestErr = cErr
				}
			}
			if !cdrSaved && !errors.Is(cdrRestErr, ErrSolAuthRejected) && ctx.Err() == nil {
				cdrBytes, statusCode, _, cdrErr := ConsultarCdrSOAP(
					b.svc.soapEndpoint,
					item.EmpresaRUC,
					usuarioSOL,
					claveSOL,
					rucEmisor,
					item.TipoCP,
					item.Serie,
					item.Numero,
				)
				if cdrErr == nil && len(cdrBytes) > 0 {
					estado := strings.TrimSpace(statusCode)
					if estado == "0004" || estado == "0" {
						estado = "CDR_ACEPTADO"
					} else if estado != "" {
						estado = "SUNAT_" + estado
					}
					cdrName := fmt.Sprintf("R-%s-%s-%s-%s.zip", rucEmisor, padTipoCP(item.TipoCP), item.Serie, item.Numero)
					if path, hashHex, wErr := b.svc.saveFile(item.EmpresaRUC, item.Origen, cdrName, cdrBytes); wErr == nil {
						_ = b.svc.repo.UpdateFilePath(item.ID, item.EmpresaRUC, "cdr", path, estado, hashHex)
						item.RutaCDR = path
						item.TieneCDR = true
						res.CDR = "ok"
						cdrSaved = true
					}
				} else if cdrErr != nil && cdrRestErr == nil {
					cdrRestErr = cdrErr
				}
			}
			if !cdrSaved {
				if isFatalSolAuthError(cdrRestErr) || errors.Is(cdrRestErr, ErrSunatTransitorio) {
					res.CDR = "error"
					res.Error = appendErr(res.Error, "CDR: "+cdrRestErr.Error())
					res.rawErr = mergeItemError(res.rawErr, cdrRestErr)
				} else {
					res.CDR = "no_disponible"
					if cdrRestErr != nil {
						res.rawErr = mergeItemError(res.rawErr, cdrRestErr)
					}
				}
			}
		}
	}

	return res
}

func (b *BulkDownloader) downloadWithRetry(
	ctx context.Context,
	kind string, // "xml", "pdf" o "cdr"
	rucAuth, rucEmisor, tipoCP, serie, numero string,
	getToken func() string,
	refreshToken func() (string, error),
) (*SunatCPEDownloadResult, error) {
	return b.downloadWithRetryAttempts(ctx, kind, rucAuth, rucEmisor, tipoCP, serie, numero, 5, getToken, refreshToken)
}

func (b *BulkDownloader) downloadWithRetryAttempts(
	ctx context.Context,
	kind string, // "xml", "pdf" o "cdr"
	rucAuth, rucEmisor, tipoCP, serie, numero string,
	maxAttempts int,
	getToken func() string,
	refreshToken func() (string, error),
) (*SunatCPEDownloadResult, error) {
	// En lote masivo: hasta 5 intentos para absorber HTTP 500 intermitentes de api-cpe.
	// En descarga individual interactiva: acotado (ej. 2) para evitar bloqueos largos.
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	refreshed := false

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		tok := getToken()
		var dl *SunatCPEDownloadResult
		var err error
		switch kind {
		case "xml":
			dl, err = b.svc.sunatCPE.DownloadXML(ctx, tok, rucAuth, rucEmisor, tipoCP, serie, numero)
		case "pdf":
			dl, err = b.svc.sunatCPE.DownloadPDF(ctx, tok, rucAuth, rucEmisor, tipoCP, serie, numero)
		case "cdr":
			dl, err = b.svc.sunatCPE.DownloadCDR(ctx, tok, rucAuth, rucEmisor, tipoCP, serie, numero)
		default:
			return nil, fmt.Errorf("tipo de descarga no soportado: %s", kind)
		}
		if err == nil {
			return dl, nil
		}

		// Token expirado -> renovar 1 vez y reintentar de inmediato
		if errors.Is(err, ErrSolTokenExpired) {
			if refreshed {
				return nil, err
			}
			refreshed = true
			if _, rErr := refreshToken(); rErr != nil {
				return nil, fmt.Errorf("error renovando token SOL: %w", rErr)
			}
			continue
		}

		// Errores permanentes o de sesión SOL -> no reintentar
		if errors.Is(err, ErrComprobanteNoDisponible) ||
			errors.Is(err, ErrSunatSolicitudRechazada) ||
			isFatalSolAuthError(err) {
			return nil, err
		}

		// Error transitorio (HTTP 500/502/503/429 o red) -> backoff acotado
		if attempt < maxAttempts && b.retryDelay > 0 {
			backoff := b.retryDelay * time.Duration(attempt)
			if maxAttempts <= 2 && backoff > 600*time.Millisecond {
				backoff = 600 * time.Millisecond
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		} else if attempt == maxAttempts {
			return nil, err
		}
	}
	return nil, ErrSunatTransitorio
}

// packageBulkZIP crea el paquete ZIP organizado (XML/, PDF/, CDR/, JSON/, resumen_comprobantes.csv y resumen_items.csv)
func (b *BulkDownloader) packageBulkZIP(
	ruc, origen, periodo, jobID string,
	items []ComprobanteCPE,
	razonEmpresa string,
	wantXML, wantPDF, wantCDR, wantJSON bool,
) (string, string, error) {
	lotesDir := filepath.Join(b.svc.storageDir, ruc, "cpe", "lotes")
	if err := os.MkdirAll(lotesDir, 0755); err != nil {
		return "", "", err
	}

	tagPeriodo := periodo
	if tagPeriodo == "" {
		tagPeriodo = "TODOS"
	}
	zipName := fmt.Sprintf("CPE_%s_%s_%s_%s.zip", ruc, origen, tagPeriodo, time.Now().Format("20060102_150405"))
	relZipPath := filepath.ToSlash(filepath.Join(ruc, "cpe", "lotes", zipName))
	zipPath := filepath.Join(lotesDir, zipName)

	zf, err := os.Create(zipPath)
	if err != nil {
		return "", "", err
	}
	defer zf.Close()

	zw := zip.NewWriter(zf)
	addedEntries := make(map[string]bool)
	addFile := func(entryName string, content []byte) {
		if len(content) == 0 || addedEntries[entryName] {
			return
		}
		if w, cErr := zw.Create(entryName); cErr == nil {
			_, _ = w.Write(content)
			addedEntries[entryName] = true
		}
	}

	// Buffers para CSV de comprobantes y CSV de detalle de ítems (con BOM UTF-8 para Excel)
	var compCSV bytes.Buffer
	compCSV.Write([]byte{0xEF, 0xBB, 0xBF})
	cw := csv.NewWriter(&compCSV)
	_ = cw.Write([]string{
		"Origen", "Periodo", "TipoCP", "Serie", "Numero", "FechaEmision",
		"RucEmisor", "RazonEmisor", "DocCliente", "RazonCliente",
		"Moneda", "BaseImponible", "IGV", "MontoTotal", "FormaPago",
		"EstadoSUNAT", "ConcordanciaSIRE", "DetalleConcordancia", "ResumenItems",
		"TieneXML", "TienePDF", "TieneCDR", "HashXML",
	})

	var itemsCSV bytes.Buffer
	itemsCSV.Write([]byte{0xEF, 0xBB, 0xBF})
	iw := csv.NewWriter(&itemsCSV)
	_ = iw.Write([]string{
		"Origen", "Periodo", "TipoCP", "Serie", "Numero", "FechaEmision",
		"RucEmisor", "RazonEmisor", "Moneda", "ItemNro", "Codigo",
		"Cantidad", "Unidad", "Descripcion", "ValorUnitario", "PrecioUnitario", "IGVItem", "ValorVenta",
	})

	// Recargar estado fresco desde BD para cada comprobante del lote
	for _, orig := range items {
		comp, gErr := b.svc.repo.GetByID(orig.ID, ruc)
		if gErr != nil || comp == nil {
			comp = &orig
		}

		rucEmisor := comp.EmpresaRUC
		if strings.ToUpper(comp.Origen) == "COMPRAS" && len(comp.RucContraparte) == 11 {
			rucEmisor = comp.RucContraparte
		}
		baseFile := fmt.Sprintf("%s-%s-%s-%s", rucEmisor, padTipoCP(comp.TipoCP), comp.Serie, comp.Numero)

		var ublMeta *UBLMetadata
		if comp.RutaXML != "" {
			if rawXML, rErr := b.svc.readSafeFile(comp.RutaXML); rErr == nil && len(rawXML) > 0 {
				if wantXML {
					addFile("XML/"+baseFile+".xml", rawXML)
				}
				ublMeta, _ = ParseUBLXML(rawXML)
			}
		}

		if wantPDF {
			if comp.RutaPDF != "" {
				if rawPDF, rErr := b.svc.readSafeFile(comp.RutaPDF); rErr == nil && len(rawPDF) > 0 {
					addFile("PDF/"+baseFile+".pdf", rawPDF)
				}
			} else if ublMeta != nil {
				addFile("PDF/"+baseFile+".pdf", GenerateComprobantePDFFromUBL(comp, ublMeta, razonEmpresa))
			}
		}

		if wantCDR && comp.RutaCDR != "" {
			if rawCDR, rErr := b.svc.readSafeFile(comp.RutaCDR); rErr == nil && len(rawCDR) > 0 {
				ext := filepath.Ext(comp.RutaCDR)
				if ext == "" {
					ext = ".zip"
				}
				addFile("CDR/R-"+baseFile+ext, rawCDR)
			}
		}

		docJSON := buildComprobanteJSON(comp, ublMeta, razonEmpresa)
		if wantJSON {
			if jBytes, jErr := json.MarshalIndent(docJSON, "", "  "); jErr == nil {
				addFile("JSON/"+baseFile+".json", jBytes)
			}
		}

		_ = cw.Write([]string{
			comp.Origen, comp.Periodo, comp.TipoCP, comp.Serie, comp.Numero, docJSON.FechaEmision,
			docJSON.RucEmisor, docJSON.RazonEmisor, docJSON.NumDocCliente, docJSON.RazonCliente,
			docJSON.Moneda, fmt.Sprintf("%.2f", docJSON.BaseImponible), fmt.Sprintf("%.2f", docJSON.IGV),
			fmt.Sprintf("%.2f", docJSON.MontoTotal), docJSON.FormaPago, comp.EstadoSUNAT,
			comp.ConcordanciaSIRE, comp.DetalleConcordancia, comp.ResumenItems,
			boolSiNo(comp.TieneXML), boolSiNo(comp.TienePDF), boolSiNo(comp.TieneCDR), docJSON.HashXML,
		})

		for _, it := range docJSON.Items {
			_ = iw.Write([]string{
				comp.Origen, comp.Periodo, comp.TipoCP, comp.Serie, comp.Numero, docJSON.FechaEmision,
				docJSON.RucEmisor, docJSON.RazonEmisor, docJSON.Moneda, it.ID, it.Codigo,
				formatCantidad(it.Cantidad), it.Unidad, it.Descripcion,
				fmt.Sprintf("%.4f", it.ValorUnitario), fmt.Sprintf("%.4f", it.PrecioUnitario),
				fmt.Sprintf("%.2f", it.IGV), fmt.Sprintf("%.2f", it.ValorVenta),
			})
		}
	}

	cw.Flush()
	iw.Flush()
	addFile("resumen_comprobantes.csv", compCSV.Bytes())
	addFile("resumen_items.csv", itemsCSV.Bytes())

	if err := zw.Close(); err != nil {
		return "", "", err
	}
	return relZipPath, zipName, nil
}

func (b *BulkDownloader) updateJobStatus(aj *activeJob, estado, msg string) {
	aj.mu.Lock()
	aj.data.Estado = estado
	aj.data.Mensaje = msg
	aj.data.ActualizadoEn = time.Now()
	aj.mu.Unlock()
}

func (b *BulkDownloader) failJob(aj *activeJob, msg string) {
	now := time.Now()
	aj.mu.Lock()
	aj.data.Estado = "ERROR"
	aj.data.Mensaje = msg
	aj.data.ActualizadoEn = now
	aj.data.FinalizadoEn = &now
	aj.mu.Unlock()
}

func normalizeFormatos(in []string) []string {
	if len(in) == 0 {
		return []string{"xml", "pdf", "cdr", "json"}
	}
	seen := make(map[string]bool)
	out := make([]string, 0, len(in))
	for _, f := range in {
		f = strings.ToLower(strings.TrimSpace(f))
		switch f {
		case "xml", "pdf", "cdr", "json":
			if !seen[f] {
				seen[f] = true
				out = append(out, f)
			}
		}
	}
	if len(out) == 0 {
		return []string{"xml", "pdf"}
	}
	return out
}

func containsStr(slice []string, target string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, target) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

func appendErr(existing, msg string) string {
	if existing == "" {
		return msg
	}
	return existing + " | " + msg
}

func boolSiNo(v bool) string {
	if v {
		return "SI"
	}
	return "NO"
}
