package company

import (
	"net/http"

	"github.com/SwodLore/sire-api/internal/sunat"
	"github.com/gin-gonic/gin"
)

type Controller struct {
	service *Service
}

func NewController(service *Service) *Controller {
	return &Controller{service: service}
}

func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	empresas := r.Group("/empresas")
	{
		empresas.GET("", ctrl.List)
		empresas.POST("", ctrl.Create)
		empresas.GET("/:ruc", ctrl.Get)
		empresas.PUT("/:ruc", ctrl.Update)
		empresas.DELETE("/:ruc", ctrl.Delete)
		empresas.POST("/:ruc/probar-conexion", ctrl.TestConnection)
	}
}

func (ctrl *Controller) List(c *gin.Context) {
	empresas, err := ctrl.service.ListEmpresas()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":     false,
			"error":       "Error al listar empresas",
			"detalle":     err.Error(),
			"codigo_http": http.StatusInternalServerError,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": empresas})
}

func (ctrl *Controller) Get(c *gin.Context) {
	ruc := c.Param("ruc")
	empresa, err := ctrl.service.GetEmpresa(ruc)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":     false,
			"error":       "Empresa no encontrada",
			"detalle":     err.Error(),
			"codigo_http": http.StatusNotFound,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": empresa})
}

func (ctrl *Controller) Create(c *gin.Context) {
	var req CreateEmpresaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "Parámetros obligatorios incompletos o inválidos",
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	empresa, err := ctrl.service.CreateEmpresa(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "Error al registrar empresa",
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"mensaje": "Empresa registrada exitosamente",
		"data":    empresa,
	})
}

func (ctrl *Controller) Update(c *gin.Context) {
	ruc := c.Param("ruc")
	var req UpdateEmpresaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "Parámetros inválidos",
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	empresa, err := ctrl.service.UpdateEmpresa(ruc, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "Error al actualizar empresa",
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mensaje": "Empresa actualizada exitosamente",
		"data":    empresa,
	})
}

func (ctrl *Controller) Delete(c *gin.Context) {
	ruc := c.Param("ruc")
	if err := ctrl.service.DeleteEmpresa(ruc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "Error al eliminar empresa",
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mensaje": "Empresa eliminada exitosamente",
	})
}

func (ctrl *Controller) TestConnection(c *gin.Context) {
	ruc := c.Param("ruc")
	resultado, err := ctrl.service.TestConnection(ruc)
	if err != nil {
		sunat.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, resultado)
}
