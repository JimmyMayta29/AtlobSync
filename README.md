# API-SIRE 🇵🇪

API REST en Go para integración con el **Sistema Integrado de Registros Electrónicos (SIRE)** de SUNAT - Perú.

Permite gestionar los Registros de Ventas (RVIE) y Compras electrónicos mediante los servicios web oficiales de SUNAT.

## 📋 Requisitos Previos

- **Go** 1.20 o superior
- **Credenciales SUNAT**: CLIENT_ID y CLIENT_SECRET del [Portal de Desarrolladores SUNAT](https://api-developers.sunat.gob.pe/)
- **Usuario SOL** con permisos para facturación electrónica

## 🚀 Instalación

```bash
# Clonar el repositorio
git clone https://github.com/SwodLore/API-SIRE.git
cd API-SIRE

# Instalar dependencias
go mod download

# Configurar variables de entorno
cp .env.example .env
# Editar .env con tus credenciales reales
```

## ⚙️ Configuración

Crea un archivo `.env` en la raíz del proyecto con las siguientes variables:

| Variable | Descripción |
|----------|-------------|
| `CLIENT_ID` | ID de cliente de la API SUNAT |
| `CLIENT_SECRET` | Secreto del cliente API SUNAT |
| `RUC` | Número de RUC del contribuyente (11 dígitos) |
| `USUARIO` | Usuario SOL |
| `CLAVE_SOL` | Clave SOL |
| `TOKEN` | Token de acceso (se obtiene del endpoint `/auth/token`) |

## 🏃 Ejecución

```bash
# Modo desarrollo (con hot-reload usando Air)
air

# Modo estándar
go run main.go
```

El servidor se iniciará en `http://localhost:8080`

## 📚 Endpoints Disponibles

### Autenticación

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| POST | `/auth/token` | Obtener token de acceso OAuth2 |

### Registro de Ventas (RVIE)

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/sunat/ventas/periodos/:codLibro` | Consultar periodos disponibles |
| POST | `/sunat/ventas/importar-reemplazo/:perTributario` | Importar reemplazo de propuesta |
| POST | `/sunat/ventas/importar-nuevos/:perTributario` | Importar nuevos comprobantes |
| POST | `/sunat/ventas/registrar-preliminar/:perTributario` | Registrar preliminar |
| POST | `/sunat/ventas/aceptar-propuesta/:perTributario` | Aceptar propuesta RVIE |
| POST | `/sunat/ventas/exclusion-definitiva/:perTributario` | Exclusión definitiva de NC y facturas |
| POST | `/sunat/ventas/tipo-cambio-masivo/:perTributario` | Agregar tipo de cambio masivo |
| PUT | `/sunat/ventas/tipo-cambio/:perTributario` | Editar tipo de cambio individual |
| DELETE | `/sunat/ventas/eliminar-comprobante/:perTributario` | Eliminar comprobante propuesta |
| DELETE | `/sunat/ventas/eliminar-preliminar/:perTributario` | Eliminar comprobante preliminar |
| GET | `/sunat/ventas/estado-ticket` | Consultar estado de ticket |
| GET | `/sunat/ventas/descargar-archivo` | Descargar archivo |
| GET | `/sunat/ventas/descargar-propuesta/:perTributario` | Descargar propuesta |
| GET | `/sunat/ventas/descargar-no-incluidos/:perTributario` | Descargar no incluidos |
| GET | `/sunat/ventas/descargar-resumen/:perTributario` | Descargar resumen |
| GET | `/sunat/ventas/exportar-preliminar/:perTributario` | Exportar preliminar |

### Registro de Compras

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| POST | `/sunat/compras/aceptar-propuesta/:perTributario` | Aceptar propuesta |
| POST | `/sunat/compras/importar-reemplazo/:perTributario` | Importar reemplazo propuesta |
| POST | `/sunat/compras/registrar-preliminar/:perTributario` | Registrar preliminar |
| POST | `/sunat/compras/cargar-no-domiciliados/:perTributario` | Cargar no domiciliados |
| POST | `/sunat/compras/importar-complemento/:perTributario` | Importar datos complementarios |
| POST | `/sunat/compras/tipo-cambio-masivo/:perTributario` | Tipo de cambio masivo |
| PUT | `/sunat/compras/reintegro-credito/:perTributario` | Actualizar reintegro crédito fiscal |
| PUT | `/sunat/compras/credito-fiscal/:perTributario` | Actualizar crédito fiscal especial |
| PUT | `/sunat/compras/coeficiente-prorrata/:perTributario` | Actualizar coeficiente prorrata |
| DELETE | `/sunat/compras/eliminar-propuesta/:perTributario` | Eliminar comprobante propuesta |
| DELETE | `/sunat/compras/eliminar-preliminar/:perTributario` | Eliminar comprobante preliminar |

> **Nota:** `perTributario` tiene formato `YYYYMM` (ej: 202412 para diciembre 2024)

## 📁 Estructura del Proyecto

```
API-SIRE/
├── main.go              # Punto de entrada
├── AUTH/                # Módulo de autenticación OAuth2
│   ├── controller.go
│   ├── routes.go
│   └── service.go
├── VENTAS/              # Módulo Registro de Ventas
│   ├── controllers/
│   ├── models/
│   ├── routes/
│   └── services/
├── COMPRAS/             # Módulo Registro de Compras
│   ├── controllers/
│   ├── models/
│   ├── routes/
│   └── services/
├── .air.toml            # Configuración hot-reload
├── .env.example         # Ejemplo de variables de entorno
├── go.mod               
└── go.sum               
```

## 🔐 Flujo de Autenticación

1. Configura las variables de entorno (`CLIENT_ID`, `CLIENT_SECRET`, `RUC`, `USUARIO`, `CLAVE_SOL`)
2. Llama al endpoint `POST /auth/token`
3. Copia el `access_token` recibido a la variable `TOKEN` en tu `.env`
4. Usa los demás endpoints (el token expira, renuévalo cuando sea necesario)

## 🛠️ Tecnologías

- **[Gin](https://github.com/gin-gonic/gin)** - Framework web
- **[Resty](https://github.com/go-resty/resty)** - Cliente HTTP
- **[godotenv](https://github.com/joho/godotenv)** - Carga de variables de entorno
- **[Air](https://github.com/cosmtrek/air)** - Hot-reload para desarrollo

## 📖 Documentación SUNAT

- [API SIRE - Documentación Oficial](https://api-developers.sunat.gob.pe/)
- [Manual de Desarrolladores SIRE](https://www.sunat.gob.pe/legislacion/supam/sire/)

## ⚠️ Notas Importantes

- Los errores **422** de SUNAT son normales al probar con datos no reales
- El error **401** indica token expirado o inválido
- El formato de `perTributario` es **YYYYMM** (ej: 202412)
- Siempre valida los parámetros obligatorios antes de enviar

## 📄 Licencia

MIT License - Ver [LICENSE](LICENSE) para más detalles.

---

Desarrollado con ❤️ para la comunidad de desarrolladores peruanos.
