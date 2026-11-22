-- ============================================================================
-- ATLOB Sync · SUNAT — Comandos rápidos para Supabase SQL Editor
-- (También puedes hacer todo esto sin tocar SQL usando gestionar_licencias.bat)
-- ============================================================================

-- 1. Crear una DEMO de 7 días (máximo 3 empresas) para un nuevo prospecto
SELECT license_key, client_name, plan_label, duration_days, max_companies
FROM public.create_demo_license(
    'Estudio Contable Ejemplo SAC', -- Nombre del prospecto / empresa
    '999888777',                    -- Teléfono o correo (opcional)
    7,                              -- Días de prueba (inician cuando el cliente la activa)
    3,                              -- Máximo de RUCs permitidos en demo (0 = ilimitado)
    'Solicitó demo desde video promocional'
);

-- 2. Crear una licencia ANUAL comercial (RUCs ilimitados = 0)
SELECT license_key, client_name, plan_label, max_companies
FROM public.create_commercial_license(
    'Cliente Pagado SAC',
    'anual',
    365,
    0,
    '999888777',
    'Pago confirmado'
);

-- 3. Crear una licencia MENSUAL comercial (30 días, RUCs ilimitados)
SELECT license_key, client_name, plan_label
FROM public.create_commercial_license('Cliente Mensual SAC', 'mensual', 30, 0, '', '');

-- 4. Crear una licencia PERMANENTE
SELECT license_key, client_name, plan_label
FROM public.create_commercial_license('Cliente Permanente SAC', 'permanente', NULL, 0, '', '');

-- 5. Ver tablero completo de clientes, estado y cuándo fue su última conexión
SELECT * FROM public.v_licenses_dashboard;

-- 6. Extender 5 días adicionales a un prospecto que pidió más tiempo
SELECT license_key, status, expires_at
FROM public.extend_license('ATLOB-DEMO-XXXX-XXXX', 5);

-- 7. Liberar equipo si el cliente cambió de computadora
SELECT license_key, machine_hash, machine_name, expires_at
FROM public.reset_license_machine('ATLOB-PRO-XXXX-XXXX');

-- 8. Revocar acceso de inmediato
SELECT license_key, status, notes
FROM public.revoke_license('ATLOB-DEMO-XXXX-XXXX', 'Periodo finalizado / cancelado');

-- 9. Ver últimos 30 eventos (activaciones, intentos desde otra PC, expiraciones)
SELECT created_at, license_key, event_type, machine_name, detail
FROM public.license_events
ORDER BY created_at DESC
LIMIT 30;
