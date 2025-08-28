package main

import (
	"log"

	AUTH "github.com/SwodLore/sire-api/AUTH"
	comprasRoutes "github.com/SwodLore/sire-api/COMPRAS/routes"
	ventasRoutes "github.com/SwodLore/sire-api/VENTAS/routes"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No se pudo cargar .env, usando variables de entorno del sistema")
	}
	r := gin.Default()

	// Registrar todas las rutas de SUNAT
	AUTH.RegisterAuthRoutes(r) // Asumiendo que existe esta función en el paquete AUTH
	comprasRoutes.RegisterSunatRoutesCompras(r)
	ventasRoutes.RegisterSunatRoutesVentas(r)

	r.Run(":8080")
}
