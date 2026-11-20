package sire

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var rePeriodo = regexp.MustCompile(`^\d{6}$`)

type proposalCacheKey struct {
	descargaID int64
	mtimeNano  int64
	maxBytes   int64
}

type proposalCacheEntry struct {
	key        proposalCacheKey
	parsed     *ParsedProposalTXT
	expiresAt  time.Time
	lastAccess time.Time
}

type Service struct {
	repo                *Repository
	sireClient          *SunatSireClient
	worker              *Worker
	storageDir          string
	maxProposalTXTBytes int64
	proposalCacheMu     sync.Mutex
	proposalCache       map[proposalCacheKey]*proposalCacheEntry
	proposalCacheMax    int
	proposalCacheTTL    time.Duration
	proposalCacheHits   int64
	proposalCacheMisses int64
}

func NewService(repo *Repository, sireClient *SunatSireClient, worker *Worker, storageDir string) *Service {
	if storageDir == "" {
		storageDir = "descargas"
	}
	return &Service{
		repo:       repo,
		sireClient: sireClient,
		worker:     worker,
		storageDir: storageDir,
	}
}

// SolicitarExportacion inicia el flujo asíncrono en SUNAT y lo encola en el worker
func (s *Service) SolicitarExportacion(ruc, libro, periodo string, filtros FiltrosExportacion) (*DescargaSire, error) {
	if !rePeriodo.MatchString(periodo) {
		return nil, fmt.Errorf("el periodo tributario debe tener 6 dígitos numéricos (formato YYYYMM, ej. 202410)")
	}

	libroUpper := strings.ToUpper(strings.TrimSpace(libro))
	var codLibro string
	var ticket string
	var err error

	if libroUpper == "VENTAS" || libroUpper == "RVIE" || libroUpper == "140000" {
		libroUpper = "VENTAS"
		codLibro = "140000"
		ticket, err = s.sireClient.ExportarPropuestaVentas(ruc, periodo, filtros)
	} else if libroUpper == "COMPRAS" || libroUpper == "RCE" || libroUpper == "080000" {
		libroUpper = "COMPRAS"
		codLibro = "080000"
		ticket, err = s.sireClient.ExportarPropuestaCompras(ruc, periodo, filtros)
	} else {
		return nil, fmt.Errorf("libro '%s' no válido. Use 'ventas' (140000) o 'compras' (080000)", libro)
	}

	if err != nil {
		return nil, err
	}

	// Registrar en la base de datos
	descarga := &DescargaSire{
		EmpresaRUC:   ruc,
		CodLibro:     codLibro,
		Libro:        libroUpper,
		Periodo:      periodo,
		Ticket:       ticket,
		Estado:       string(EstadoEnProceso),
		MensajeSUNAT: "Ticket registrado en SUNAT. Encolado para descarga automática.",
	}

	if err := s.repo.Create(descarga); err != nil {
		// Si ya existía por UNIQUE(empresa_ruc, ticket), recuperar el existente
		existing, errGet := s.repo.GetByTicket(ruc, ticket)
		if errGet == nil && existing != nil {
			s.worker.Enqueue(existing.ID)
			return existing, nil
		}
		return nil, fmt.Errorf("error al guardar ticket en base de datos: %w", err)
	}

	// Encolar inmediatamente en el worker en segundo plano
	s.worker.Enqueue(descarga.ID)

	return descarga, nil
}

// ConsultarDetallePropuesta obtiene los comprobantes paginados directamente de SUNAT
func (s *Service) ConsultarDetallePropuesta(ruc, libro, periodo string, f FiltrosPropuestaDetalle) (map[string]interface{}, error) {
	if !rePeriodo.MatchString(periodo) {
		return nil, fmt.Errorf("el periodo tributario debe tener 6 dígitos numéricos (formato YYYYMM)")
	}

	libroUpper := strings.ToUpper(strings.TrimSpace(libro))
	var codLibro string
	if libroUpper == "VENTAS" || libroUpper == "RVIE" || libroUpper == "140000" {
		codLibro = "140000"
	} else if libroUpper == "COMPRAS" || libroUpper == "RCE" || libroUpper == "080000" {
		codLibro = "080000"
	} else {
		return nil, fmt.Errorf("libro '%s' no válido. Use 'ventas' o 'compras'", libro)
	}

	return s.sireClient.ConsultarDetallePropuesta(ruc, codLibro, periodo, f)
}

// ListarDescargas retorna el historial de tickets y descargas de la empresa
func (s *Service) ListarDescargas(ruc, libro, periodo string) ([]DescargaSire, error) {
	return s.repo.ListByEmpresa(ruc, libro, periodo)
}

// ObtenerDescarga obtiene los datos de un ticket por su ID
func (s *Service) ObtenerDescarga(id int64, ruc string) (*DescargaSire, error) {
	d, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("descarga no encontrada")
	}

	if ruc != "" && d.EmpresaRUC != ruc {
		return nil, errors.New("acceso no autorizado a este registro de descarga")
	}

	return d, nil
}

// ObtenerDescargaPorTicket obtiene los datos de un ticket por número de ticket y RUC
func (s *Service) ObtenerDescargaPorTicket(ticket, ruc string) (*DescargaSire, error) {
	d, err := s.repo.GetByTicket(ruc, ticket)
	if err != nil {
		return nil, errors.New("ticket no encontrado en el sistema")
	}
	return d, nil
}

// ReintentarDescarga vuelve a encolar un ticket existente en ERROR o EN_PROCESO para descargar su archivo
func (s *Service) ReintentarDescarga(id int64) (*DescargaSire, error) {
	d, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("descarga no encontrada")
	}
	if d.Estado == string(EstadoCompletado) {
		return d, nil
	}
	if err := s.repo.ResetForRetry(id); err != nil {
		return nil, fmt.Errorf("no se pudo reiniciar el ticket: %w", err)
	}
	s.worker.Enqueue(id)
	updated, err := s.repo.GetByID(id)
	if err != nil {
		return d, nil
	}
	return updated, nil
}

// ObtenerArchivoParaDescarga verifica permisos y ruta física en disco sin aceptar rutas arbitrarias del cliente
func (s *Service) ObtenerArchivoParaDescarga(id int64, ruc string) (*DescargaSire, string, error) {
	d, err := s.ObtenerDescarga(id, ruc)
	if err != nil {
		return nil, "", err
	}

	if d.Estado != string(EstadoCompletado) {
		return nil, "", fmt.Errorf("el archivo aún no está listo (estado actual: %s). Mensaje: %s", d.Estado, d.MensajeSUNAT)
	}

	if d.RutaLocal == "" {
		return nil, "", errors.New("no existe ruta registrada para el archivo físico")
	}

	// Seguridad y portabilidad: resuelve rutas relativas al storageDir y rutas absolutas heredadas dentro del área permitida
	safePath, err := resolveSafeStoragePath(s.storageDir, d.RutaLocal)
	if err != nil {
		return nil, "", err
	}

	return d, safePath, nil
}

// DescargarConstanciaRecepcion descarga la Constancia de Recepción (CDR) del libro SIRE y guarda copia local
func (s *Service) DescargarConstanciaRecepcion(ruc, libro, nomArchivo string) ([]byte, string, string, error) {
	libroUpper := strings.ToUpper(strings.TrimSpace(libro))
	codLibro := "140000"
	libroDir := "VENTAS"
	if libroUpper == "COMPRAS" || libroUpper == "RCE" || libroUpper == "080000" {
		codLibro = "080000"
		libroDir = "COMPRAS"
	}

	data, contentType, err := s.sireClient.DescargarConstanciaRecepcion(ruc, codLibro, nomArchivo)
	if err != nil {
		return nil, "", "", err
	}

	cleanName := filepath.Base(filepath.Clean(strings.TrimSpace(nomArchivo)))
	if cleanName == "." || cleanName == "" {
		cleanName = fmt.Sprintf("constancia_%s_%s.zip", ruc, codLibro)
	}
	if !strings.Contains(cleanName, ".") {
		if isZipBytes(data) {
			cleanName += ".zip"
		} else if len(data) >= 4 && string(data[:4]) == "%PDF" {
			cleanName += ".pdf"
		} else {
			cleanName += ".json"
		}
	}

	targetDir := filepath.Join(s.storageDir, ruc, libroDir, "constancias")
	if err := os.MkdirAll(targetDir, 0755); err == nil {
		_ = os.WriteFile(filepath.Join(targetDir, cleanName), data, 0644)
	}

	return data, cleanName, contentType, nil
}

// AnalizarPropuestaTXT lee en memoria el archivo TXT oficial dentro del ZIP de una descarga COMPLETADA,
// valida ruta dentro de storageDir, valida layout/cabeceras y devuelve resumen + filas paginadas
// sin disparar sincronización con comprobantes_cpe ni escribir archivos en disco.
func (s *Service) AnalizarPropuestaTXT(id int64, opts ProposalQueryOptions) (*ProposalAnalysisResponse, error) {
	d, err := s.repo.GetByID(id)
	if err != nil {
		return nil, &ProposalParseError{
			Code:       ErrCodeNoEncontrado,
			HTTPStatus: http.StatusNotFound,
			Message:    "descarga no encontrada",
		}
	}

	if d.Estado != string(EstadoCompletado) {
		return nil, &ProposalParseError{
			Code:       ErrCodeNoCompletado,
			HTTPStatus: http.StatusConflict,
			Message:    "la descarga aún no está completada",
			Detail:     fmt.Sprintf("estado actual: %s", d.Estado),
		}
	}

	safePath, err := resolveSafeStoragePath(s.storageDir, d.RutaLocal)
	if err != nil {
		return nil, err
	}

	maxBytes := opts.MaxUncompressedBytes
	if maxBytes <= 0 {
		maxBytes = s.maxProposalTXTBytes
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxProposalTXTBytes
	}

	fileInfo, statErr := os.Stat(safePath)
	if statErr != nil || fileInfo.IsDir() {
		return nil, &ProposalParseError{
			Code:       ErrCodeNoEncontrado,
			HTTPStatus: http.StatusNotFound,
			Message:    "el archivo físico no fue encontrado en el servidor",
		}
	}

	cacheKey := proposalCacheKey{
		descargaID: d.ID,
		mtimeNano:  fileInfo.ModTime().UnixNano(),
		maxBytes:   maxBytes,
	}

	parsed := s.getCachedProposal(cacheKey)
	if parsed == nil {
		parsed, err = ParseProposalZIPFile(safePath, d.Libro, d.CodLibro, maxBytes)
		if err != nil {
			return nil, err
		}

		parsed.Resumen.DescargaID = d.ID
		if d.Periodo != "" {
			parsed.Resumen.Periodo = d.Periodo
		}
		parsed.Resumen.NombreArchivoZIP = d.NombreArchivo
		parsed.Resumen.Ticket = d.Ticket

		s.putCachedProposal(cacheKey, parsed)
	}

	return PaginateProposalResult(parsed, opts), nil
}

func (s *Service) getCachedProposal(key proposalCacheKey) *ParsedProposalTXT {
	now := time.Now()
	s.proposalCacheMu.Lock()
	defer s.proposalCacheMu.Unlock()

	if s.proposalCache == nil {
		s.proposalCacheMisses++
		return nil
	}

	entry, ok := s.proposalCache[key]
	if !ok {
		s.proposalCacheMisses++
		return nil
	}
	if now.After(entry.expiresAt) {
		delete(s.proposalCache, key)
		s.proposalCacheMisses++
		return nil
	}

	entry.lastAccess = now
	s.proposalCacheHits++
	return entry.parsed
}

func (s *Service) putCachedProposal(key proposalCacheKey, parsed *ParsedProposalTXT) {
	now := time.Now()
	s.proposalCacheMu.Lock()
	defer s.proposalCacheMu.Unlock()

	if s.proposalCache == nil {
		s.proposalCache = make(map[proposalCacheKey]*proposalCacheEntry)
	}

	maxEntries := s.proposalCacheMax
	if maxEntries <= 0 {
		maxEntries = DefaultProposalCacheMaxEntries
	}
	ttl := s.proposalCacheTTL
	if ttl <= 0 {
		ttl = time.Duration(DefaultProposalCacheTTL) * time.Second
	}

	// Purgar entradas expiradas o de la misma descarga con mtime distinto
	for k, e := range s.proposalCache {
		if now.After(e.expiresAt) || (k.descargaID == key.descargaID && k.mtimeNano != key.mtimeNano) {
			delete(s.proposalCache, k)
		}
	}

	// Evicción LRU si se alcanzó el límite máximo de entradas en memoria
	for len(s.proposalCache) >= maxEntries {
		var oldestKey proposalCacheKey
		var oldestTime time.Time
		first := true
		for k, e := range s.proposalCache {
			if first || e.lastAccess.Before(oldestTime) {
				oldestKey = k
				oldestTime = e.lastAccess
				first = false
			}
		}
		delete(s.proposalCache, oldestKey)
	}

	s.proposalCache[key] = &proposalCacheEntry{
		key:        key,
		parsed:     parsed,
		expiresAt:  now.Add(ttl),
		lastAccess: now,
	}
}
