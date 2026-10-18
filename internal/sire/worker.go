package sire

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type TokenProvider interface {
	IsCooldown(ruc string) (bool, time.Time, string)
}

type Worker struct {
	repo       *Repository
	sireClient *SunatSireClient
	tokenMgr   TokenProvider
	storageDir string
	queue      chan int64
	stopChan   chan struct{}
	activeMu   sync.Mutex
	activeJobs map[int64]bool
	running    bool
}

func NewWorker(repo *Repository, sireClient *SunatSireClient, tokenMgr TokenProvider, storageDir string) *Worker {
	if storageDir == "" {
		storageDir = "descargas"
	}
	return &Worker{
		repo:       repo,
		sireClient: sireClient,
		tokenMgr:   tokenMgr,
		storageDir: storageDir,
		queue:      make(chan int64, 500),
		stopChan:   make(chan struct{}),
		activeJobs: make(map[int64]bool),
	}
}

// Start inicia el worker en segundo plano y recupera tickets pendientes de ejecuciones anteriores
func (w *Worker) Start() {
	w.activeMu.Lock()
	if w.running {
		w.activeMu.Unlock()
		return
	}
	w.running = true
	w.activeMu.Unlock()

	// 1. Recuperación de tickets pendientes de SQLite al iniciar el servidor
	w.recoverPendingTickets()

	// 2. Goroutine consumidora de tickets
	go w.workerLoop()

	// 3. Goroutine periódica para escanear tickets rezagados cada 30 segundos
	go w.periodicScanner()
}

func (w *Worker) Stop() {
	w.activeMu.Lock()
	defer w.activeMu.Unlock()
	if !w.running {
		return
	}
	w.running = false
	close(w.stopChan)
}

// Enqueue añade un ID de descarga para seguimiento en segundo plano
func (w *Worker) Enqueue(id int64) {
	w.activeMu.Lock()
	if w.activeJobs[id] {
		w.activeMu.Unlock()
		return
	}
	w.activeJobs[id] = true
	w.activeMu.Unlock()

	select {
	case w.queue <- id:
	default:
		log.Printf("[SIRE-WORKER] Advertencia: Cola de tickets llena, se procesará en el siguiente escaneo periódico (ID: %d)", id)
	}
}

func (w *Worker) recoverPendingTickets() {
	tickets, err := w.repo.GetPendingTickets()
	if err != nil {
		log.Printf("[SIRE-WORKER] Error al recuperar tickets pendientes: %v", err)
		return
	}

	if len(tickets) > 0 {
		log.Printf("[SIRE-WORKER] Recuperados %d tickets pendientes en SQLite tras reinicio del servidor.", len(tickets))
		for _, t := range tickets {
			w.Enqueue(t.ID)
		}
	}
}

func (w *Worker) periodicScanner() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopChan:
			return
		case <-ticker.C:
			tickets, err := w.repo.GetPendingTickets()
			if err != nil {
				continue
			}
			for _, t := range tickets {
				w.Enqueue(t.ID)
			}
		}
	}
}

func (w *Worker) workerLoop() {
	for {
		select {
		case <-w.stopChan:
			return
		case id := <-w.queue:
			go func(jobID int64) {
				w.processJob(jobID)
				w.activeMu.Lock()
				delete(w.activeJobs, jobID)
				w.activeMu.Unlock()
			}(id)
		}
	}
}

func (w *Worker) processJob(id int64) {
	descarga, err := w.repo.GetByID(id)
	if err != nil || descarga == nil {
		return
	}

	// Si ya terminó o falló definitivamente, no procesar
	if descarga.Estado == string(EstadoCompletado) || descarga.Estado == string(EstadoError) {
		return
	}

	// 1. Verificar si la empresa está en enfriamiento de seguridad por credenciales inválidas
	if w.tokenMgr != nil {
		if inCooldown, until, reason := w.tokenMgr.IsCooldown(descarga.EmpresaRUC); inCooldown {
			log.Printf("[SIRE-WORKER] Empresa %s en enfriamiento de seguridad hasta %s (%s). Dejando ticket %s en espera sin consumir reintentos.",
				descarga.EmpresaRUC, until.Format("15:04:05"), reason, descarga.Ticket)
			return
		}
	}

	// 2. Verificar si la empresa continúa activa
	active, err := w.repo.IsCompanyActive(descarga.EmpresaRUC)
	if err == nil && !active {
		log.Printf("[SIRE-WORKER] Omitiendo procesamiento para ticket %s: la empresa con RUC %s se encuentra inactiva o fue eliminada.", descarga.Ticket, descarga.EmpresaRUC)
		return
	}

	// Consultar estado en SUNAT con codLibro y codOrigenEnvio=2 (Ventas v31 5.16 y Compras v28 5.31)
	status, err := w.sireClient.ConsultarEstadoTicket(descarga.EmpresaRUC, descarga.Periodo, descarga.Periodo, descarga.Ticket, descarga.CodLibro)
	if err != nil {
		log.Printf("[SIRE-WORKER] Error al consultar ticket %s (RUC %s): %v", descarga.Ticket, descarga.EmpresaRUC, err)

		// Si entró en enfriamiento tras el intento de token, dejar el ticket en espera sin consumir reintentos
		if w.tokenMgr != nil {
			if inCooldown, _, _ := w.tokenMgr.IsCooldown(descarga.EmpresaRUC); inCooldown {
				log.Printf("[SIRE-WORKER] Empresa %s entró en enfriamiento. Ticket %s queda en espera sin consumir intentos.", descarga.EmpresaRUC, descarga.Ticket)
				return
			}
		}

		if descarga.Reintentos >= 35 {
			_ = w.repo.UpdateStatus(descarga.ID, string(EstadoError), fmt.Sprintf("Error persistente al consultar SUNAT: %v", err))
			return
		}

		_ = w.repo.IncrementRetries(descarga.ID, fmt.Sprintf("Error temporal de red con SUNAT: %v", err))
		w.retryWithBackoff(id, descarga.Reintentos+1)
		return
	}

	// Analizar estado reportado por SUNAT
	// "06": Terminado / archivo listo para descarga
	if status.CodEstadoProceso == "06" {
		nomArchivo := ""
		codTipo := "null"

		if len(status.ArchivoReporte) > 0 {
			nomArchivo = status.ArchivoReporte[0].NomArchivoReporte
			if status.ArchivoReporte[0].CodTipoArchivoReporte != "" {
				codTipo = status.ArchivoReporte[0].CodTipoArchivoReporte
			}
		}

		if nomArchivo == "" {
			nomArchivo = fmt.Sprintf("LE%s-%s-%s.zip", descarga.EmpresaRUC, descarga.Periodo, descarga.CodLibro)
		}

		perTributario := status.PerTributario
		if perTributario == "" {
			perTributario = descarga.Periodo
		}
		codProceso := status.CodProceso
		if codProceso == "" {
			codProceso = "1"
		}

		// Descargar archivo binario con los 6 parámetros exigidos por Ventas v31 5.17 y Compras v28 5.32
		log.Printf("[SIRE-WORKER] Ticket %s listo. Descargando archivo '%s' desde SUNAT (codLibro=%s, perTributario=%s, codProceso=%s)...",
			descarga.Ticket, nomArchivo, descarga.CodLibro, perTributario, codProceso)
		fileBytes, err := w.sireClient.DescargarArchivoReporte(
			descarga.EmpresaRUC,
			nomArchivo,
			codTipo,
			perTributario,
			codProceso,
			descarga.Ticket,
			descarga.CodLibro,
		)
		if err != nil {
			log.Printf("[SIRE-WORKER] Error al descargar archivo de reporte para ticket %s: %v", descarga.Ticket, err)

			if w.tokenMgr != nil {
				if inCooldown, _, _ := w.tokenMgr.IsCooldown(descarga.EmpresaRUC); inCooldown {
					log.Printf("[SIRE-WORKER] Empresa %s en enfriamiento durante descarga. Ticket %s queda en espera.", descarga.EmpresaRUC, descarga.Ticket)
					return
				}
			}

			_ = w.repo.UpdateStatus(descarga.ID, string(EstadoError), fmt.Sprintf("Error al descargar archivo desde SUNAT: %v", err))
			return
		}

		// Preservar la extensión que SUNAT entrega en nomArchivoReporte; no forzar .xlsx arbitrariamente
		cleanNomArchivo := filepath.Base(filepath.Clean(nomArchivo))
		if bytes.HasPrefix(fileBytes, []byte("PK\x03\x04")) {
			if !strings.HasSuffix(strings.ToLower(cleanNomArchivo), ".zip") {
				cleanNomArchivo += ".zip"
			}
		} else if !strings.Contains(cleanNomArchivo, ".") {
			// Si no tiene extensión:
			// Ventas 5.18: 0 = txt, 1 = xls
			// Compras 5.34: 0 = txt, 1 = csv
			if descarga.CodLibro == "140000" {
				if codTipo == "1" {
					cleanNomArchivo += ".xls"
				} else {
					cleanNomArchivo += ".txt"
				}
			} else {
				if codTipo == "1" {
					cleanNomArchivo += ".csv"
				} else {
					cleanNomArchivo += ".txt"
				}
			}
		}

		targetDir := filepath.Join(w.storageDir, descarga.EmpresaRUC, descarga.Libro, descarga.Periodo)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			log.Printf("[SIRE-WORKER] Error al crear directorio local de almacenamiento '%s': %v", targetDir, err)
			_ = w.repo.UpdateStatus(descarga.ID, string(EstadoError), fmt.Sprintf("Error en sistema de archivos local: %v", err))
			return
		}

		targetFilePath := filepath.Join(targetDir, cleanNomArchivo)
		if err := os.WriteFile(targetFilePath, fileBytes, 0644); err != nil {
			log.Printf("[SIRE-WORKER] Error al escribir archivo '%s': %v", targetFilePath, err)
			_ = w.repo.UpdateStatus(descarga.ID, string(EstadoError), fmt.Sprintf("Error al escribir archivo en disco: %v", err))
			return
		}

		// Calcular hash SHA-256 para verificación de integridad
		hasher := sha256.New()
		hasher.Write(fileBytes)
		hashHex := hex.EncodeToString(hasher.Sum(nil))
		fileSize := int64(len(fileBytes))

		if err := w.repo.UpdateCompleted(descarga.ID, cleanNomArchivo, targetFilePath, fileSize, hashHex); err != nil {
			log.Printf("[SIRE-WORKER] Error al actualizar estado completado en BD: %v", err)
			return
		}

		log.Printf("[SIRE-WORKER] [EXITO] Ticket %s completado. Archivo: %s (%d bytes, SHA256: %s)",
			descarga.Ticket, targetFilePath, fileSize, hashHex[:12])
		return
	}

	// Estados de procesamiento en cola o progreso ("00", "01", "02", "03", "04")
	if strings.Contains("00 01 02 03 04", status.CodEstadoProceso) || status.CodEstadoProceso == "" {
		desEstado := status.DesEstadoProceso
		if desEstado == "" {
			desEstado = "En proceso en SUNAT"
		}

		if descarga.Reintentos >= 40 {
			_ = w.repo.UpdateStatus(descarga.ID, string(EstadoError), "Se agotó el tiempo máximo de espera de respuesta de SUNAT (40 intentos)")
			return
		}

		msg := fmt.Sprintf("%s (código: %s, intento %d)", desEstado, status.CodEstadoProceso, descarga.Reintentos+1)
		_ = w.repo.IncrementRetries(descarga.ID, msg)
		w.retryWithBackoff(id, descarga.Reintentos+1)
		return
	}

	// Cualquier otro estado indica fallo o rechazo en SUNAT
	_ = w.repo.UpdateStatus(descarga.ID, string(EstadoError), fmt.Sprintf("SUNAT reportó finalización con error (código %s): %s", status.CodEstadoProceso, status.DesEstadoProceso))
}

func (w *Worker) retryWithBackoff(id int64, retryCount int) {
	// Backoff exponencial con jitter razonable: 3s, 5s, 8s, 13s, 20s, max 30s
	delaySeconds := 3
	if retryCount == 2 {
		delaySeconds = 5
	} else if retryCount == 3 {
		delaySeconds = 8
	} else if retryCount == 4 {
		delaySeconds = 13
	} else if retryCount == 5 {
		delaySeconds = 20
	} else if retryCount > 5 {
		delaySeconds = 30
	}

	time.AfterFunc(time.Duration(delaySeconds)*time.Second, func() {
		w.activeMu.Lock()
		w.activeJobs[id] = false
		w.activeMu.Unlock()
		w.Enqueue(id)
	})
}
