package routes

import (
	"github.com/SwodLore/sire-api/COMPRAS/controllers"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registra los endpoints de Compras en el grupo especificado (ej. /api/empresas/:ruc/compras)
func RegisterRoutes(sunat *gin.RouterGroup) {
	// 2 Servicio Web Api aceptar propuesta
	sunat.POST("/aceptarpropuesta/:perTributario", controllers.AceptarPropuestaController)
	// 3 Servicio Web Api importar reemplazo de la propuesta
	sunat.POST("/reemplazo-propuesta/:perTributario", controllers.ReemplazoPropuestaController)
	// 4 Servicio Web Api registrar preliminar
	sunat.POST("/registrar-preliminar/:perTributario", controllers.RegistrarPreliminarController)
	// 5 Servicio Web Api cargar registro de compra no domiciliados
	sunat.POST("/cargar-no-domiciliados/:perTributario", controllers.CargarNoDomiciliadosController)
	// 6 Servicio Web Api importar datos complementarios de los CP de la propuesta
	sunat.POST("/importar-complemento-propuesta/:perTributario", controllers.ImportarComplementoPropuestaController)
	// 7 Servicio Web Api importar nuevos comprobantes preliminar
	sunat.POST("/importar-comprobantes-preliminar/:perTributario", controllers.ImportarComprobantesPreliminarController)
	// 8 Servicio Web Api incluir-excluir comprobantes de la propuesta
	sunat.POST("/incluir-excluir-propuesta/:perTributario", controllers.IncluirExcluirPropuestaController)
	// 9 Servicio Web Api importar nuevos comprobantes de pago
	sunat.POST("/importar-comprobantes-propuesta/:perTributario", controllers.ImportarComprobantesPropuestaController)
	// 10 Servicio Web Api importar tipo de cambio masivo
	sunat.POST("/importar-tipo-cambio-masivo/:perTributario/:codLibro", controllers.ImportarTipoCambioMasivoController)
	// 11 Servicio Web Api actualizar reintegro del crédito fiscal
	sunat.PUT("/actualizar-reintegro-credito/:perTributario", controllers.ActualizarReintegroCreditoController)
	// 12 Servicio Web Api actualizar crédito fiscal especial
	sunat.PUT("/actualizar-credito-fiscal-especial/:perTributario", controllers.ActualizarCreditoFiscalEspecialController)
	// 13 Servicio Web Api actualizar coeficiente de prorrata
	sunat.PUT("/actualizar-coeficiente-prorrata/:perTributario", controllers.ActualizarCoeficienteProrrataController)
	// 14 Servicio Web Api consultar FV0621
	sunat.GET("/consultar-fv0621", controllers.ConsultarFV0621Controller)
	// 15 Servicio Web Api eliminar comprobante de la propuesta
	sunat.DELETE("/eliminar-comprobante-propuesta/:perTributario", controllers.EliminarComprobantePropuestaController)
	// 16 Servicio Web Api eliminar comprobante del preliminar
	sunat.POST("/eliminar-comprobante-preliminar/:perTributario", controllers.EliminarComprobantePreliminarController)
	// 17 Servicio Web Api eliminar preliminar
	sunat.PUT("/eliminar-preliminar/:perTributario/:indEliminar", controllers.EliminarPreliminar)
	// 18 Servicio Web Api cargar ajustes posteriores
	sunat.POST("/cargar-ajustes-posteriores", controllers.CargarAjustesPosteriores)
	// 19 Servicio Web Api enviar ajustes posteriores
	sunat.POST("/enviar-ajustes-posteriores/:perTributario/:codOrigenEnvio", controllers.EnviarAjustesPosteriores)
	// 20 Servicio Web Api eliminar comprobante de ajustes posteriores
	sunat.DELETE("/eliminar-comprobante-ajustes/:perTributario/:indTipoAjustePosterior", controllers.EliminarComprobanteAjustes)
	// 21 Servicio Web Api cargar ajustes posteriores no domiciliados
	sunat.POST("/cargar-ajustes-no-domiciliados", controllers.CargarAjustesNoDomiciliados)
	// 22 Servicio Web Api enviar ajustes posteriores no domiciliados
	sunat.POST("/enviar-ajustes-no-domiciliados/:perTributario/:numAjustePosterior/:codLibro/:numTicket", controllers.EnviarAjustesNoDomiciliados)
	// 23 Servicio Web Api eliminar comprobante de ajustes posteriores no domiciliados
	sunat.DELETE("/libros/rce/ajustesposteriores/web/comprobantesajuspost/:perTributario/:indTipoAjustePosterior/eliminarcomprobanteaprcnd", controllers.EliminarComprobanteAjustesPosteriorNoDOM)
	// 24 Servicio Web Api cargar ajustes posteriores de periodos anteriores
	sunat.POST("/libros/rvierce/receptorajustesposteriores/web/ajustesposteriores/upload", controllers.CargarAjustesPosterioresPeriodosAnteriores)
	// 25 Servicio Web Api enviar ajustes posteriores de periodos anteriores
	sunat.POST("/libros/rce/ajustesposteriores/web/comprobantesajuspost/:perTributario/:numAjustePosterior/:codLibro/:numTicket/registrarajustesposterioresparc", controllers.EnviarAjustesPosterioresPeriodosAnteriores)
	// 26 Servicio Web Api eliminar comprobante de ajustes posteriores de periodos anteriores
	sunat.DELETE("/libros/rce/ajustesposteriores/web/ajustesposteriores/:indTipoAjustePosterior/:perTributario/eliminarcomprobanteapparc", controllers.EliminarComprobanteAjustesPosterioresPeriodosAnteriores)
	// 27 Servicio Web Api cargar ajustes posteriores de periodos anteriores no domiciliados
	sunat.POST("/libros/rvierce/receptorajustesposteriores/web/ajustesposteriores/upload/nodomiciliados", controllers.CargarAjustesPosterioresPeriodosAnterioresNoDOM)
	// 28 Servicio Web Api enviar ajustes posteriores
	sunat.POST("/ajustes-posteriores/:perTributario/:numAjustePosterior/:codLibro/:numTicket", controllers.EnviarAjustesPosterioresController)
	// 29 Servicio Web Api eliminar comprobante ajustes posteriores
	sunat.DELETE("/libros/rce/ajustes/:numAjustePosterior/:perTributario/eliminar", controllers.EliminarComprobanteAjustesRCE)
	// 30 Servicio Web Api consultar tickets
	sunat.GET("/libros/rce/tickets/consultar", controllers.ConsultarEstadoTickets)
	// 31 Servicio Web Api descargar archivo reporte
	sunat.GET("/libros/rce/archivo/descargar", controllers.DescargarArchivoReporte)
	// 32 Servicio Web Api consultar periodos
	sunat.GET("/libros/rce/periodos/:codLibro", controllers.ConsultarPeriodosRCE)
	sunat.GET("/periodos/:codLibro", controllers.ConsultarPeriodosRCE)
	sunat.GET("/periodos", controllers.ConsultarPeriodosRCE)
	// 33 Servicio Web Api descargar propuesta
	sunat.GET("/libros/rce/propuesta/:perTributario/descargar", controllers.DescargarPropuestaRCE)
	// 34 Servicio Web Api descargar resumen
	sunat.GET("/libros/rce/resumen/:perTributario/:codTipoResumen/:codTipoArchivo/descargar", controllers.DescargarResumenRCE)
	// 35 Servicio Web Api descargar excluidos
	sunat.GET("/libros/rce/excluidos/:perTributario/descargar", controllers.DescargarExcluidosRCE)
	// 36 Servicio Web Api eliminar comprobante no domiciliado
	sunat.PUT("/eliminar-comprobante-no-domiciliado/:perTributario", controllers.EliminarComprobanteNoDomiciliado)
	// 37 Servicio Web Api exportar preliminar no domiciliados
	sunat.GET("/exportar-preliminar-no-domiciliados/:perTributario", controllers.ExportarPreliminarNoDomiciliados)
	// 38 Servicio Web Api exportar preliminar registro compras
	sunat.GET("/exportar-preliminar-registro-compras/:perTributario", controllers.ExportarPreliminarRegistroCompras)
	// 39 Servicio Web Api descargar reporte casillas
	sunat.GET("/descargar-reporte-casillas/:perTributario/:tipoReporte/:tipoDescarga", controllers.DescargarReporteCasillas)
	// 40 Servicio Web Api descargar inconsistencias preliminar
	sunat.GET("/descargar-inconsistencias-preliminar/:perTributario/:codTipoArchivo", controllers.DescargarInconsistenciasPreliminar)
	// 41 Servicio Web Api descargar inconsistencias montos totales
	sunat.GET("/descargar-inconsistencias-montos-totales/:perTributario", controllers.DescargarInconsistenciasMontosTotales)
	// 42 Servicio Web Api descargar inconsistencias comprobantes
	sunat.GET("/descargar-inconsistencias-comprobantes/:perTributario/:codLibro", controllers.DescargarInconsistenciasComprobantes)
	// 43 Servicio Web Api descargar ajustes posteriores
	sunat.GET("/descargar-ajustes-posteriores/:perTributario", controllers.DescargarAjustesPosteriores)
	// 44 Servicio Web Api descargar ajustes posteriores no domiciliados
	sunat.POST("/descargar-ajustes-posteriores-no-domiciliados/:perTributario", controllers.DescargarAjustesPosterioresNoDomiciliados)
	// 45 Servicio Web Api exportar ajustes posteriores
	sunat.GET("/ajustes-posteriores/:perTributario/exportar", controllers.ExportarAjustesPosteriores)
	// 46 Servicio Web Api exportar ajustes posteriores ND
	sunat.GET("/ajustes-posteriores-nd/:perTributario/exportar", controllers.ExportarAjustesPosterioresND)
	// 47 Servicio Web Api constancia recepcion
	sunat.GET("/constancia-recepcion", controllers.DescargarConstanciaRecepcion)
	// 48 Servicio Web Api reporte consolidado
	sunat.GET("/reporte-consolidado/:perTributario/descargar", controllers.DescargarReporteConsolidado)
	// 49 Servicio Web Api RCE periodo
	sunat.GET("/rce-periodo/:perTributario/descargar", controllers.DescargarRCEPorPeriodo)
	// 50 Servicio Web Api inconsistencias periodo
	sunat.GET("/inconsistencias-periodo/descargar", controllers.DescargarInconsistenciasPeriodo)
	// 51 Servicio Web Api reporte CAR
	sunat.GET("/reporte-car/:perTributario", controllers.DescargarReporteCARPorPeriodo)
	// 52 Servicio Web Api reporte estadistico compras
	sunat.GET("/reporte-estadistico-compras", controllers.DescargarReporteEstadisticoCompras)
	// 53 Servicio Web Api reporte estadistico NC ND
	sunat.GET("/reporte-estadistico-nc-nd", controllers.DescargarReporteEstadisticoNCND)
	// 54 Servicio Web Api reporte estadistico compras dia
	sunat.GET("/reporte-estadistico-compras-dia", controllers.DescargarReporteEstadisticoComprasDia)
	// 55 Servicio Web Api reporte estadistico compras CIIU
	sunat.GET("/reporte-estadistico-compras-ciiu", controllers.DescargarReporteEstadisticoComprasCIIU)
	// 56 Servicio Web Api reporte cumplimiento
	sunat.GET("/reporte-cumplimiento/:perTributario/:codLibro", controllers.DescargarReporteCumplimiento)
	// 57 Servicio Web Api consultar ajustes posteriores RCE
	sunat.PUT("/ajustes-posteriores-rce/:periodoSeleccionado", controllers.ConsultarAjustesPosterioresRCE)
	// 58 Servicio Web Api listar comprobantes ajustes post
	sunat.GET("/comprobantes-ajustes-post/:periodoSeleccionado", controllers.ListarComprobantesAjustesPost)
}
