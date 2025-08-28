package routes

import (
	"github.com/SwodLore/sire-api/VENTAS/controllers"

	"github.com/gin-gonic/gin"
)

func RegisterSunatRoutesVentas(r *gin.Engine) {
	sunat := r.Group("/api/sunat/ventas")
	{
		// 2 Servicio Web Api consultar año y mes
		sunat.GET("/periodos/:codLibro", controllers.GetPeriodos)
		// 3 Servicio Web Api importar reemplazo de la propuesta
		sunat.POST("/importar-reemplazo-propuesta/:perTributario", controllers.ImportarReemplazoPropuesta)
		// 4 Servicio Web Api importar nuevos comprobantes propuesta
		sunat.POST("/importar-nuevos-comprobantes/:perTributario", controllers.ImportarNuevosComprobantes)
		// 5 Servicio Web Api importar nuevos comprobantes preliminar
		sunat.POST("/importar-comprobantes-preliminar/:perTributario", controllers.ImportarComprobantesPreliminar)
		// 6 Servicio Web Api importar ajustes posteriores
		sunat.POST("/importar-ajustes-posteriores/:perTributario", controllers.ImportarAjustesPosteriores)
		// 7 Servicio Web Api importar ajustes posteriores de periodos anteriores
		sunat.POST("/importar-ajustes-periodos-anteriores/:perTributario", controllers.ImportarAjustesPeriodosAnteriores)
		// 8 Servicio Web Api aceptar propuesta del RVIE
		sunat.POST("/aceptar-propuesta/:perTributario", controllers.AceptarPropuestaRVIE)
		// 9 Servicio Web Api registrar preliminar
		sunat.POST("/registrar-preliminar/:perTributario", controllers.RegistrarPreliminarController)
		// 10 Servicio Web Api exclusión definitiva de notas de crédito y facturas
		sunat.POST("/exclusion-definitiva/:perTributario", controllers.ExclusionDefinitivaController)
		// 11 Servicio Web Api agregar tipo de cambio masivo
		sunat.POST("/tipo-cambio-masivo/:perTributario", controllers.TipoCambioMasivoController)
		// 12 Servicio Web Api editar tipo de cambio individual
		sunat.PUT("/editar-tipo-cambio/:perTributario", controllers.EditarTipoCambioController)
		// 13 Servicio Web Api eliminar comprobante propuesta
		sunat.DELETE("/eliminar-comprobante/:perTributario", controllers.EliminarComprobanteController)
		// 14 Servicio Web Api eliminar comprobante preliminar
		sunat.PUT("/eliminar-comprobante-preliminar/:perTributario", controllers.EliminarComprobantePreliminar)
		// 15 Servicio Web Api eliminar reemplazo propuesta
		sunat.PUT("/eliminar-reemplazo/:perTributario", controllers.EliminarReemplazoPropuesta)
		// 16 Servicio Web Api consultar estado de envío de ticket
		sunat.GET("/consultar-estado-ticket", controllers.ConsultarEstadoTicket)
		// 17 Servicio Web Api descargar archivo
		sunat.GET("/descargar-archivo", controllers.DescargarArchivo)
		// 18 Servicio Web Api descargar propuesta
		sunat.GET("/descargar-propuesta/:perTributario", controllers.DescargarPropuesta)
		// 19 Servicio Web Api descargar no incluidos
		sunat.GET("/descargar-no-incluidos/:perTributario", controllers.DescargarNoIncluidos)
		// 20 Servicio Web Api descargar resumen
		sunat.GET("/descargar-resumen/:perTributario/:codTipoResumen/:codTipoArchivo", controllers.DescargarResumen)
		// 21 Servicio Web Api descargar resumen inconsistencias
		sunat.GET("/descargar-resumen-inconsistencias/:perPeriodoTributario", controllers.DescargarResumenInconsistencias)
		// 22 Servicio Web Api exportar preliminar de registro de Ventas
		sunat.GET("/exportar-preliminar-ventas/:perTributario", controllers.ExportarPreliminarVentas)
		// 23 Servicio Web Api descargar reporte de casillas
		sunat.GET("/descargar-reporte-casillas/:perTributario/:tipoReporte/:tipoDescarga", controllers.DescargarReporteCasillas)
		// 24 Servicio Web Api descargar inconsistencias en registros preliminar registrado
		sunat.GET("/descargar-inconsistencias-preliminar/:perTributario/:numCas", controllers.DescargarInconsistenciasPreliminar)
		// 25 Servicio Web Api descargar inconsistencias por comprobante pago
		sunat.GET("/inconsistencias-comprobantes/:perTributario", controllers.DescargarInconsistenciasComprobantes)
		// 26 Servicio Web Api descargar constancia de recepción
		sunat.GET("/constancia-recepcion", controllers.DescargarConstanciaRecepcion)
		// 27 Servicio Web Api descargar RVIE por periodo
		sunat.GET("/rvie-periodo/:perTributario", controllers.DescargarRVIEPorPeriodo)
		// 28 Servicio Web Api descargar reporte consolidado por periodo
		sunat.GET("/reporte-consolidado-periodo/:perTributario", controllers.DescargarReporteConsolidadoPorPeriodo)
		// 29 Servicio Web Api descargar ajustes posteriores
		sunat.GET("/ajustes-posteriores/:perTributario", controllers.DescargarAjustesPosteriores)
		// 30 Servicio Web Api descargar ajustes posteriores de periodos anteriores
		sunat.GET("/ajustes-posteriores-periodos-anteriores/:perTributario", controllers.DescargarAjustesPosterioresPeriodosAnteriores)
		// 31 Servicio Web Api descargar reporte inconsistencias por periodo
		sunat.GET("/inconsistencias-periodo/:codRegistroLibro", controllers.DescargarInconsistenciasPorPeriodo)
		// 32 Servicio Web Api descargar reporte CAR
		sunat.GET("/reporte-car/:perTributario", controllers.DescargarReporteCAR)
		// 33 Servicio Web Api descargar reporte estadístico
		sunat.GET("/reporte-estadistico/:perTributario", controllers.DescargarReporteEstadistico)
		// 34 Servicio Web Api descargar reporte de cumplimiento
		sunat.GET("/reporte-cumplimiento/:perTributario/:codLibro", controllers.DescargarReporteCumplimiento)
		// 35 Servicio Web Api reporte de exportadores
		sunat.GET("/reporte-exportadores/:perTributario", controllers.ReporteExportadores)
		// 36 Servicio Web Api eliminar preliminar registrado
		sunat.PUT("/eliminapreliminar/:perTributario", controllers.EliminarPreliminarController)
	}
}
