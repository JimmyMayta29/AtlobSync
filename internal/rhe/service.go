package rhe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Service struct {
	repo       *Repository
	creds      CredentialsProvider
	provider   ReceptorProvider
	storageDir string
}

func NewService(repo *Repository, creds CredentialsProvider, storageDir string) *Service {
	if strings.TrimSpace(storageDir) == "" {
		storageDir = "descargas"
	}
	return &Service{
		repo:       repo,
		creds:      creds,
		provider:   NewLocalSOLReceptorClient(),
		storageDir: storageDir,
	}
}

// SetProvider permite inyectar un proveedor alternativo o mock (útil en pruebas unitarias).
func (s *Service) SetProvider(p ReceptorProvider) {
	if p != nil {
		s.provider = p
	}
}

// List consulta los RHE almacenados en SQLite local (0 llamadas a SUNAT).
func (s *Service) List(ruc string, filter RHEListFilter) (*RHEListaResponse, error) {
	ruc = strings.TrimSpace(ruc)
	if filter.Periodo != "" {
		if _, _, err := ValidatePeriodo(filter.Periodo); err != nil {
			return nil, err
		}
	}

	items, err := s.repo.List(ruc, filter)
	if err != nil {
		return nil, err
	}
	totalEmpresa, _ := s.repo.CountByEmpresa(ruc)
	meta, _ := s.repo.GetConsultaMeta(ruc, filter.Periodo)

	return &RHEListaResponse{
		EmpresaRUC:     ruc,
		Periodo:        filter.Periodo,
		Rol:            "RECEPTOR",
		Modo:           "LOCAL_SOL",
		TotalEmpresa:   totalEmpresa,
		Total:          len(items),
		Recibos:        items,
		Resumen:        ComputeResumen(items),
		UltimaConsulta: meta,
		EstadoSOL:      s.provider.Status(ruc),
	}, nil
}

// ConsultarEnSUNAT ejecuta una consulta en tiempo real contra SUNAT SOL (Consulta Receptor 11.5.1.1.14),
// almacena el reporte oficial TXT en la carpeta de descargas local y persiste los registros en SQLite.
func (s *Service) ConsultarEnSUNAT(ctx context.Context, ruc, periodo string) (*RHEConsultaResponse, error) {
	ruc = strings.TrimSpace(ruc)
	periodo = strings.TrimSpace(periodo)
	if _, _, err := ValidatePeriodo(periodo); err != nil {
		return nil, err
	}

	rucOut, _, usuarioSOL, claveSOL, _, err := s.creds.GetDecryptedCredentials(ruc)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron obtener las credenciales SOL de la empresa: %w", err)
	}

	rawRes, err := s.provider.ConsultarPeriodo(ctx, rucOut, usuarioSOL, claveSOL, periodo)
	if err != nil {
		return nil, err
	}

	var recibos []ReciboHonorario
	if !rawRes.Empty && len(rawRes.RawTXT) > 0 {
		recibos, err = ParseReceptorTXT(rawRes.RawTXT, rucOut, periodo)
		if err != nil {
			return nil, fmt.Errorf("error interpretando reporte TXT de SUNAT: %w", err)
		}
	} else {
		recibos = []ReciboHonorario{}
	}

	// Guardar copia local del archivo TXT oficial en descargas/<ruc>/RHE/<periodo>/
	relPath := filepath.Join(rucOut, "RHE", periodo, fmt.Sprintf("RHE_Receptor_%s_%s.txt", rucOut, periodo))
	absPath := filepath.Join(s.storageDir, relPath)
	if mkErr := os.MkdirAll(filepath.Dir(absPath), 0755); mkErr == nil {
		txtToSave := rawRes.RawTXT
		if len(txtToSave) == 0 {
			txtToSave = BuildRHETXT(recibos)
		}
		_ = os.WriteFile(absPath, txtToSave, 0600)
	}

	if err := s.repo.SavePeriodoResults(rucOut, periodo, rawRes.FechaDesde, rawRes.FechaHasta, relPath, recibos); err != nil {
		return nil, err
	}

	saved, err := s.repo.List(rucOut, RHEListFilter{Periodo: periodo})
	if err != nil {
		return nil, err
	}
	meta, _ := s.repo.GetConsultaMeta(rucOut, periodo)

	msg := fmt.Sprintf("Se consultaron %d Recibo(s) por Honorarios en SUNAT (%s al %s).", len(saved), rawRes.FechaDesde, rawRes.FechaHasta)
	if len(saved) == 0 {
		msg = fmt.Sprintf("Consulta completada (%s al %s): SUNAT no registró Recibos por Honorarios recibidos en este periodo.", rawRes.FechaDesde, rawRes.FechaHasta)
		if rawRes.Notice != "" {
			msg = rawRes.Notice
		}
	}

	return &RHEConsultaResponse{
		EmpresaRUC:     rucOut,
		Periodo:        periodo,
		FechaDesde:     rawRes.FechaDesde,
		FechaHasta:     rawRes.FechaHasta,
		Rol:            "RECEPTOR",
		Modo:           "LOCAL_SOL",
		Mensaje:        msg,
		Total:          len(saved),
		Recibos:        saved,
		Resumen:        ComputeResumen(saved),
		UltimaConsulta: meta,
	}, nil
}

// Exportar genera la descarga organizada en formato "xlsx" (Excel) o "txt" (delimitado por '|')
// respetando los filtros activos (periodo, estado, búsqueda).
func (s *Service) Exportar(ruc string, filter RHEListFilter, formato string) (*ExportedRHEFile, error) {
	ruc = strings.TrimSpace(ruc)
	fmtLower := strings.ToLower(strings.TrimSpace(formato))
	if fmtLower == "" {
		fmtLower = "xlsx"
	}
	if fmtLower != "xlsx" && fmtLower != "txt" {
		return nil, fmt.Errorf("formato de exportación no soportado '%s' (use xlsx o txt)", formato)
	}
	if filter.Periodo != "" {
		if _, _, err := ValidatePeriodo(filter.Periodo); err != nil {
			return nil, err
		}
	}

	items, err := s.repo.List(ruc, filter)
	if err != nil {
		return nil, err
	}
	meta, _ := s.repo.GetConsultaMeta(ruc, filter.Periodo)

	razonSocial := ""
	if s.creds != nil {
		if _, rs, _, _, _, cErr := s.creds.GetDecryptedCredentials(ruc); cErr == nil {
			razonSocial = rs
		}
	}

	baseName := fmt.Sprintf("RHE_Receptor_%s", ruc)
	if filter.Periodo != "" {
		baseName += "_" + filter.Periodo
	} else {
		baseName += "_Todos"
	}

	if fmtLower == "txt" {
		// Si no hay filtros adicionales y existe el TXT oficial guardado para ese periodo, entregar exactamente el oficial
		noExtraFilter := (strings.TrimSpace(filter.Estado) == "" || strings.EqualFold(strings.TrimSpace(filter.Estado), "TODOS")) &&
			strings.TrimSpace(filter.Query) == "" &&
			strings.TrimSpace(filter.Moneda) == ""
		if noExtraFilter && meta != nil && meta.RutaTXT != "" {
			if raw, readErr := s.readSafeStoredTXT(meta.RutaTXT); readErr == nil && len(raw) > 0 {
				return &ExportedRHEFile{
					Data:        raw,
					FileName:    baseName + ".txt",
					ContentType: "text/plain; charset=utf-8",
				}, nil
			}
		}
		return &ExportedRHEFile{
			Data:        BuildRHETXT(items),
			FileName:    baseName + ".txt",
			ContentType: "text/plain; charset=utf-8",
		}, nil
	}

	resumen := ComputeResumen(items)
	xlsxBytes, err := BuildRHEExcelXLSX(items, resumen, ruc, razonSocial, filter.Periodo, meta)
	if err != nil {
		return nil, err
	}

	return &ExportedRHEFile{
		Data:        xlsxBytes,
		FileName:    baseName + ".xlsx",
		ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	}, nil
}

func (s *Service) readSafeStoredTXT(relOrAbs string) ([]byte, error) {
	baseAbs, err := filepath.Abs(s.storageDir)
	if err != nil {
		return nil, err
	}
	candidate := relOrAbs
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(baseAbs, candidate)
	}
	candAbs, err := filepath.Abs(candidate)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(baseAbs, candAbs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil, fmt.Errorf("ruta fuera del directorio de almacenamiento")
	}
	return os.ReadFile(candAbs)
}
