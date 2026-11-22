# Servidor Comercial de Licencias en Cloudflare Workers + D1

El backend de licencias de **ATLOB Sync · SUNAT v2.0** está desplegado en tu cuenta de Cloudflare sobre **Cloudflare Workers** + **Cloudflare D1** (SQLite distribuido, plan gratuito sin pausas por inactividad).

## Recursos desplegados

- **URL del Worker:** `https://atlob-sync-licencias.jimmymayta29.workers.dev`
- **Healthcheck público:** `GET /health`
- **Base de datos D1:** `atlob-sync-licencias` (`d1b03b20-5c15-4b70-85c1-b49db1807dc5`)
- **Esquema SQL:** [schema.sql](schema.sql)
- **Código del Worker:** [worker.js](worker.js)

## Seguridad (Separación de llaves)

El Worker usa dos secretos almacenados de forma cifrada en Cloudflare (`secret_text`):

1. `CLIENT_API_KEY` (guardada en `.env.license`, incrustada al compilar `ATLOB_Sync.exe`):
   - **Solo** autoriza `POST /rest/v1/rpc/activate_license` y `POST /rest/v1/rpc/validate_license`.
   - No puede listar clientes, crear licencias, extender fechas ni borrar registros (el Worker responde `HTTP 403`).
2. `ADMIN_API_KEY` (guardada **únicamente** en `.env.admin` en tu PC):
   - Autoriza `gestionar_licencias.bat` para crear demos de 7 días, crear licencias comerciales, ver actividad, extender, revocar y liberar equipos.

## Uso diario desde tu PC

- **Emitir o administrar licencias:** haz doble clic en `API-SIRE\gestionar_licencias.bat`.
- **Compilar un nuevo instalador comercial:** haz doble clic en `API-SIRE\compilar.bat` (lee `.env.license` automáticamente).
- **Ver la base en la web:** entra a [dash.cloudflare.com](https://dash.cloudflare.com) → **Workers & Pages** → **D1 SQL Database** → `atlob-sync-licencias`.
