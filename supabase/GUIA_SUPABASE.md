# Guía de Puesta en Marcha: Licencias Comerciales con Supabase (5 minutos)

Esta guía te explica cómo activar el servidor gratuito de licencias en **Supabase**, compilar tu instalador comercial `ATLOB_Sync_Instalador.exe` y emitir accesos Demo de 7 días o licencias pagadas desde tu PC.

---

## Paso 1: Crear tu proyecto gratuito en Supabase

1. Ingresa a [https://supabase.com](https://supabase.com) e inicia sesión (con GitHub o correo).
2. Haz clic en **New Project**.
3. Elige un nombre (por ejemplo: `atlob-sync-licencias`), una contraseña segura para la base de datos y la región más cercana (por ejemplo `East US` o `South America (São Paulo)`).
4. Espera ~1 minuto a que termine de crearse.

---

## Paso 2: Crear las tablas y funciones de seguridad

1. En el menú lateral izquierdo de Supabase, entra a **SQL Editor** → **New query**.
2. Abre el archivo `API-SIRE\supabase\01_schema_licencias.sql`, copia **todo** su contenido, pégalo en el editor y haz clic en **Run** (`Ctrl+Enter`).
3. Debe mostrarse el mensaje `Success. No rows returned`.

> **Seguridad incluida:** Las tablas `licenses` y `license_events` quedan cerradas con Row Level Security (RLS) sin permisos públicos. La aplicación de escritorio solo puede ejecutar las funciones controladas `activate_license` y `validate_license`, por lo que nadie puede ver la lista de tus clientes ni modificar fechas aunque inspeccione el archivo `.exe`.

---

## Paso 3: Configurar tus credenciales locales (una sola vez)

En Supabase, ve a **Project Settings (ícono de engranaje) → API** (o **Data API / API Keys**):

1. Copia el **Project URL** (ejemplo: `https://abcdefghijklmno.supabase.co`).
2. Copia la llave **`anon` `public`**.
3. Copia la llave **`service_role` `secret`** (⚠️ **NUNCA** la incluyas en el instalador ni la compartas; solo va en tu PC para administrar licencias).

Ahora crea dos archivos en la carpeta `API-SIRE\` (ambos están ignorados por Git):

### A) `API-SIRE\.env.supabase` (se incrusta al compilar el `.exe` para clientes)
Copia `.env.supabase.example` como `.env.supabase` y completa:
```env
SUPABASE_URL=https://tu-proyecto.supabase.co
SUPABASE_ANON_KEY=tu_anon_public_key_aqui
```

### B) `API-SIRE\.env.admin` (solo en TU computadora para crear y gestionar licencias)
Copia `.env.admin.example` como `.env.admin` y completa:
```env
SUPABASE_URL=https://tu-proyecto.supabase.co
SUPABASE_SERVICE_ROLE_KEY=tu_service_role_secret_key_aqui
```

---

## Paso 4: Compilar el instalador comercial

Ejecuta doble clic en `API-SIRE\compilar.bat`.

El script leerá automáticamente `.env.supabase`, incrustará la conexión segura de licencias en `ATLOB_Sync.exe` y generará en la carpeta raíz:

```text
ATLOB_Sync_Instalador.exe
```

*(Si necesitas compilar una versión interna de pruebas sin licencia, puedes ejecutar `compilar.bat --dev-no-license` desde la terminal).*

---

## Paso 5: Generar un Demo de 7 días para tu prospecto

1. Haz doble clic en `API-SIRE\gestionar_licencias.bat`.
2. Elige la opción **1) Crear Demo de 7 días**.
3. Ingresa el nombre del cliente/estudio (ej. `Estudio Contable Pérez`) y su teléfono/correo opcional.
4. La herramienta generará una clave como `ATLOB-DEMO-7K9P-4X2M` e imprimirá un **mensaje listo para copiar y enviar por WhatsApp** junto con el enlace de descarga de `ATLOB_Sync_Instalador.exe`.

### Qué verá tu cliente
- Los **7 días empiezan a contar recién cuando el cliente activa la clave en su PC** (no pierde días si descarga el instalador hoy y lo abre mañana).
- El código se vincula automáticamente a su computadora (evita que compartan la prueba en varias PCs).
- Permite hasta **3 empresas (RUCs)** con todas las funciones activas (SIRE, CPE PDF/XML/CDR y RHE).
- Verás en la opción **3) Ver licencias y actividad** cuándo activó y cuántas veces ha usado el sistema.
- Si termina la prueba o requiere ayuda, en la app aparece tu botón directo de **WhatsApp +51 940 781 298**.

---

## Recomendación importante al enviar el `.exe` (Windows SmartScreen)

Como el instalador aún no cuenta con un certificado comercial EV de firma digital, Windows 10/11 puede mostrar una pantalla azul *"Windows protegió su PC"* la primera vez que el cliente lo abra. Envía junto al archivo esta breve nota:

> *"Al abrir el instalador, si Windows muestra 'Windows protegió su PC', haz clic en **Más información** y luego en **Ejecutar de todas formas**."*
