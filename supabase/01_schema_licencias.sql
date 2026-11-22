-- ============================================================================
-- ATLOB Sync · SUNAT v2.0 — Sistema Comercial de Licencias en Supabase
-- Script idempotente listo para ejecutar en Supabase SQL Editor.
-- ============================================================================

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- 1. Tabla principal de licencias
CREATE TABLE IF NOT EXISTS public.licenses (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    license_key     text UNIQUE NOT NULL,
    plan_type       text NOT NULL DEFAULT 'demo'
                    CHECK (plan_type IN ('demo', 'mensual', 'anual', 'permanente')),
    plan_label      text NOT NULL DEFAULT 'Demo 7 días',
    client_name     text NOT NULL DEFAULT '',
    client_contact  text NOT NULL DEFAULT '',
    status          text NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'expired', 'revoked', 'suspended')),
    duration_days   integer,
    max_companies   integer NOT NULL DEFAULT 3,
    machine_hash    text,
    machine_name    text NOT NULL DEFAULT '',
    activated_at    timestamptz,
    expires_at      timestamptz,
    last_seen_at    timestamptz,
    app_version     text NOT NULL DEFAULT '',
    companies_count integer NOT NULL DEFAULT 0,
    activation_count integer NOT NULL DEFAULT 0,
    heartbeat_count integer NOT NULL DEFAULT 0,
    notes           text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_license_key_normalized CHECK (license_key = upper(btrim(license_key)) AND length(license_key) >= 8),
    CONSTRAINT chk_max_companies_non_negative CHECK (max_companies >= 0)
);

-- 2. Auditoría de eventos (activaciones, validaciones, bloqueos, extensiones)
CREATE TABLE IF NOT EXISTS public.license_events (
    id           bigserial PRIMARY KEY,
    license_id   uuid REFERENCES public.licenses(id) ON DELETE CASCADE,
    license_key  text NOT NULL,
    event_type   text NOT NULL,
    machine_hash text NOT NULL DEFAULT '',
    machine_name text NOT NULL DEFAULT '',
    app_version  text NOT NULL DEFAULT '',
    detail       text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_licenses_status ON public.licenses(status);
CREATE INDEX IF NOT EXISTS idx_licenses_last_seen ON public.licenses(last_seen_at DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_license_events_license_id ON public.license_events(license_id, created_at DESC);

-- 3. Seguridad estricta: RLS activado y acceso directo revocado a anon/authenticated
ALTER TABLE public.licenses ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.license_events ENABLE ROW LEVEL SECURITY;

REVOKE ALL ON TABLE public.licenses FROM PUBLIC, anon, authenticated;
REVOKE ALL ON TABLE public.license_events FROM PUBLIC, anon, authenticated;

GRANT ALL ON TABLE public.licenses, public.license_events TO service_role;
GRANT USAGE, SELECT ON SEQUENCE public.license_events_id_seq TO service_role;

-- 4. Helper interno para generar respuesta JSONB uniforme
CREATE OR REPLACE FUNCTION public._build_license_response(
    p_ok boolean,
    p_code text,
    p_message text,
    p_lic public.licenses
)
RETURNS jsonb
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    v_days_rem integer := -1;
    v_hours_rem integer := -1;
BEGIN
    IF p_lic.expires_at IS NOT NULL THEN
        v_days_rem := GREATEST(0, ceil(extract(epoch FROM (p_lic.expires_at - now())) / 86400.0)::int);
        v_hours_rem := GREATEST(0, ceil(extract(epoch FROM (p_lic.expires_at - now())) / 3600.0)::int);
    END IF;

    RETURN jsonb_build_object(
        'ok', p_ok,
        'code', p_code,
        'status', COALESCE(p_lic.status, ''),
        'license_key', COALESCE(p_lic.license_key, ''),
        'plan_type', COALESCE(p_lic.plan_type, ''),
        'plan_label', COALESCE(p_lic.plan_label, ''),
        'client_name', COALESCE(p_lic.client_name, ''),
        'max_companies', COALESCE(p_lic.max_companies, 0),
        'duration_days', p_lic.duration_days,
        'activated_at', p_lic.activated_at,
        'expires_at', p_lic.expires_at,
        'days_remaining', v_days_rem,
        'hours_remaining', v_hours_rem,
        'server_time', now(),
        'message', p_message
    );
END;
$$;

REVOKE ALL ON FUNCTION public._build_license_response(boolean, text, text, public.licenses) FROM PUBLIC, anon, authenticated;

-- 5. RPC Público (cliente desktop): activar licencia y vincular al equipo en el primer uso
CREATE OR REPLACE FUNCTION public.activate_license(
    p_license_key  text,
    p_machine_hash text,
    p_machine_name text DEFAULT '',
    p_app_version  text DEFAULT ''
)
RETURNS jsonb
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    v_key          text := upper(btrim(COALESCE(p_license_key, '')));
    v_machine_hash text := lower(btrim(COALESCE(p_machine_hash, '')));
    v_machine_name text := left(btrim(COALESCE(p_machine_name, '')), 64);
    v_app_version  text := left(btrim(COALESCE(p_app_version, '')), 32);
    v_lic          public.licenses%ROWTYPE;
    v_now          timestamptz := now();
    v_new_expires  timestamptz;
BEGIN
    IF v_key = '' OR length(v_key) < 6 THEN
        RETURN jsonb_build_object(
            'ok', false,
            'code', 'NOT_FOUND',
            'status', '',
            'server_time', v_now,
            'message', 'Ingresa un código de licencia válido.'
        );
    END IF;

    IF v_machine_hash = '' OR length(v_machine_hash) < 16 THEN
        RETURN jsonb_build_object(
            'ok', false,
            'code', 'INVALID_MACHINE',
            'status', '',
            'server_time', v_now,
            'message', 'No se pudo identificar este equipo de forma segura.'
        );
    END IF;

    SELECT * INTO v_lic
    FROM public.licenses
    WHERE license_key = v_key
    FOR UPDATE;

    IF NOT FOUND THEN
        INSERT INTO public.license_events (license_id, license_key, event_type, machine_hash, machine_name, app_version, detail)
        VALUES (NULL, v_key, 'not_found', v_machine_hash, v_machine_name, v_app_version, 'Intento de activación con código inexistente');

        RETURN jsonb_build_object(
            'ok', false,
            'code', 'NOT_FOUND',
            'status', '',
            'license_key', v_key,
            'server_time', v_now,
            'message', 'El código de licencia no existe. Verifica que esté bien escrito.'
        );
    END IF;

    IF v_lic.status IN ('revoked', 'suspended') THEN
        INSERT INTO public.license_events (license_id, license_key, event_type, machine_hash, machine_name, app_version, detail)
        VALUES (v_lic.id, v_lic.license_key, v_lic.status, v_machine_hash, v_machine_name, v_app_version, 'Intento de activación en licencia ' || v_lic.status);

        RETURN public._build_license_response(
            false,
            upper(v_lic.status),
            CASE
                WHEN v_lic.status = 'revoked' THEN 'Esta licencia fue revocada. Contáctanos al 940 781 298 para soporte o renovación.'
                ELSE 'Esta licencia se encuentra suspendida temporalmente. Contáctanos al 940 781 298.'
            END,
            v_lic
        );
    END IF;

    IF v_lic.expires_at IS NOT NULL AND v_now >= v_lic.expires_at THEN
        UPDATE public.licenses
        SET status = 'expired',
            updated_at = v_now
        WHERE id = v_lic.id
        RETURNING * INTO v_lic;

        INSERT INTO public.license_events (license_id, license_key, event_type, machine_hash, machine_name, app_version, detail)
        VALUES (v_lic.id, v_lic.license_key, 'expired', v_machine_hash, v_machine_name, v_app_version, 'Licencia expirada al intentar activar');

        RETURN public._build_license_response(
            false,
            'EXPIRED',
            'Tu periodo de prueba o licencia ha finalizado. Tus datos siguen intactos en tu PC. Escríbenos al 940 781 298 para activar un plan.',
            v_lic
        );
    END IF;

    IF v_lic.machine_hash IS NULL THEN
        IF v_lic.expires_at IS NOT NULL THEN
            v_new_expires := v_lic.expires_at;
        ELSIF v_lic.duration_days IS NOT NULL AND v_lic.duration_days > 0 THEN
            v_new_expires := v_now + make_interval(days => v_lic.duration_days);
        ELSE
            v_new_expires := NULL;
        END IF;

        UPDATE public.licenses
        SET machine_hash = v_machine_hash,
            machine_name = CASE WHEN v_machine_name <> '' THEN v_machine_name ELSE machine_name END,
            app_version = CASE WHEN v_app_version <> '' THEN v_app_version ELSE app_version END,
            activated_at = COALESCE(activated_at, v_now),
            expires_at = v_new_expires,
            last_seen_at = v_now,
            activation_count = activation_count + 1,
            updated_at = v_now
        WHERE id = v_lic.id
        RETURNING * INTO v_lic;

        INSERT INTO public.license_events (license_id, license_key, event_type, machine_hash, machine_name, app_version, detail)
        VALUES (v_lic.id, v_lic.license_key, 'activated', v_machine_hash, v_machine_name, v_app_version, 'Primera activación completada');

        RETURN public._build_license_response(
            true,
            'ACTIVE',
            'Licencia activada correctamente en este equipo.',
            v_lic
        );
    END IF;

    IF v_lic.machine_hash <> v_machine_hash THEN
        INSERT INTO public.license_events (license_id, license_key, event_type, machine_hash, machine_name, app_version, detail)
        VALUES (
            v_lic.id,
            v_lic.license_key,
            'machine_mismatch',
            v_machine_hash,
            v_machine_name,
            v_app_version,
            'Bloqueado: licencia vinculada a otro equipo (' || COALESCE(NULLIF(v_lic.machine_name, ''), left(v_lic.machine_hash, 8)) || ')'
        );

        RETURN public._build_license_response(
            false,
            'MACHINE_MISMATCH',
            'Esta licencia ya está activada en otro equipo (' || COALESCE(NULLIF(v_lic.machine_name, ''), 'PC registrada') || '). Si cambiaste de PC escríbenos al 940 781 298.',
            v_lic
        );
    END IF;

    -- Mismo equipo: reactivación limpia
    UPDATE public.licenses
    SET machine_name = CASE WHEN v_machine_name <> '' THEN v_machine_name ELSE machine_name END,
        app_version = CASE WHEN v_app_version <> '' THEN v_app_version ELSE app_version END,
        last_seen_at = v_now,
        heartbeat_count = heartbeat_count + 1,
        updated_at = v_now
    WHERE id = v_lic.id
    RETURNING * INTO v_lic;

    INSERT INTO public.license_events (license_id, license_key, event_type, machine_hash, machine_name, app_version, detail)
    VALUES (v_lic.id, v_lic.license_key, 'reactivated', v_machine_hash, v_machine_name, v_app_version, 'Sesión reactivada en el equipo autorizado');

    RETURN public._build_license_response(
        true,
        'ACTIVE',
        'Licencia verificada correctamente en este equipo.',
        v_lic
    );
END;
$$;

-- 6. RPC Público (cliente desktop): validar vigencia periódica y registrar actividad
CREATE OR REPLACE FUNCTION public.validate_license(
    p_license_key     text,
    p_machine_hash    text,
    p_machine_name    text DEFAULT '',
    p_app_version     text DEFAULT '',
    p_companies_count integer DEFAULT 0
)
RETURNS jsonb
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    v_key          text := upper(btrim(COALESCE(p_license_key, '')));
    v_machine_hash text := lower(btrim(COALESCE(p_machine_hash, '')));
    v_machine_name text := left(btrim(COALESCE(p_machine_name, '')), 64);
    v_app_version  text := left(btrim(COALESCE(p_app_version, '')), 32);
    v_companies    integer := GREATEST(0, COALESCE(p_companies_count, 0));
    v_lic          public.licenses%ROWTYPE;
    v_now          timestamptz := now();
BEGIN
    SELECT * INTO v_lic
    FROM public.licenses
    WHERE license_key = v_key
    FOR UPDATE;

    IF NOT FOUND THEN
        RETURN jsonb_build_object(
            'ok', false,
            'code', 'NOT_FOUND',
            'status', '',
            'license_key', v_key,
            'server_time', v_now,
            'message', 'La licencia ya no existe en el servidor.'
        );
    END IF;

    IF v_lic.status IN ('revoked', 'suspended') THEN
        RETURN public._build_license_response(
            false,
            upper(v_lic.status),
            CASE
                WHEN v_lic.status = 'revoked' THEN 'Tu licencia fue revocada. Contáctanos al 940 781 298.'
                ELSE 'Tu licencia está suspendida temporalmente. Contáctanos al 940 781 298.'
            END,
            v_lic
        );
    END IF;

    IF v_lic.machine_hash IS NULL THEN
        RETURN public._build_license_response(
            false,
            'NOT_ACTIVATED',
            'Licencia pendiente de activación en este equipo.',
            v_lic
        );
    END IF;

    IF v_lic.machine_hash <> v_machine_hash THEN
        INSERT INTO public.license_events (license_id, license_key, event_type, machine_hash, machine_name, app_version, detail)
        VALUES (v_lic.id, v_lic.license_key, 'machine_mismatch', v_machine_hash, v_machine_name, v_app_version, 'Validación rechazada por cambio de equipo');

        RETURN public._build_license_response(
            false,
            'MACHINE_MISMATCH',
            'Esta licencia corresponde a otro equipo (' || COALESCE(NULLIF(v_lic.machine_name, ''), 'PC registrada') || '). Contáctanos al 940 781 298.',
            v_lic
        );
    END IF;

    IF v_lic.expires_at IS NOT NULL AND v_now >= v_lic.expires_at THEN
        UPDATE public.licenses
        SET status = 'expired',
            last_seen_at = v_now,
            companies_count = v_companies,
            updated_at = v_now
        WHERE id = v_lic.id
        RETURNING * INTO v_lic;

        INSERT INTO public.license_events (license_id, license_key, event_type, machine_hash, machine_name, app_version, detail)
        VALUES (v_lic.id, v_lic.license_key, 'expired', v_machine_hash, v_machine_name, v_app_version, 'Licencia expirada detectada durante validación');

        RETURN public._build_license_response(
            false,
            'EXPIRED',
            'Tu periodo de prueba o licencia ha finalizado. Tus datos siguen guardados en tu PC. Escríbenos al 940 781 298 para contratar o renovar.',
            v_lic
        );
    END IF;

    UPDATE public.licenses
    SET status = 'active',
        last_seen_at = v_now,
        companies_count = v_companies,
        machine_name = CASE WHEN v_machine_name <> '' THEN v_machine_name ELSE machine_name END,
        app_version = CASE WHEN v_app_version <> '' THEN v_app_version ELSE app_version END,
        heartbeat_count = heartbeat_count + 1,
        updated_at = v_now
    WHERE id = v_lic.id
    RETURNING * INTO v_lic;

    RETURN public._build_license_response(
        true,
        'ACTIVE',
        'Licencia vigente.',
        v_lic
    );
END;
$$;

REVOKE ALL ON FUNCTION public.activate_license(text, text, text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.activate_license(text, text, text, text) TO anon, authenticated;

REVOKE ALL ON FUNCTION public.validate_license(text, text, text, text, integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.validate_license(text, text, text, text, integer) TO anon, authenticated;

-- 7. Funciones administrativas (uso exclusivo del dueño vía service_role o SQL Editor)
CREATE OR REPLACE FUNCTION public.generate_readable_key(p_prefix text DEFAULT 'ATLOB-DEMO')
RETURNS text
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, extensions
AS $$
DECLARE
    v_alphabet text := '23456789ABCDEFGHJKMNPQRSTUVWXYZ';
    v_prefix   text := upper(btrim(COALESCE(NULLIF(p_prefix, ''), 'ATLOB-DEMO')));
    v_bytes    bytea;
    v_part1    text;
    v_part2    text;
    v_candidate text;
    i          integer;
BEGIN
    LOOP
        v_bytes := gen_random_bytes(8);
        v_part1 := '';
        v_part2 := '';
        FOR i IN 0..3 LOOP
            v_part1 := v_part1 || substr(v_alphabet, (get_byte(v_bytes, i) % length(v_alphabet)) + 1, 1);
            v_part2 := v_part2 || substr(v_alphabet, (get_byte(v_bytes, i + 4) % length(v_alphabet)) + 1, 1);
        END LOOP;
        v_candidate := v_prefix || '-' || v_part1 || '-' || v_part2;
        EXIT WHEN NOT EXISTS (SELECT 1 FROM public.licenses WHERE license_key = v_candidate);
    END LOOP;
    RETURN v_candidate;
END;
$$;

CREATE OR REPLACE FUNCTION public.create_demo_license(
    p_client_name    text,
    p_client_contact text DEFAULT '',
    p_days           integer DEFAULT 7,
    p_max_companies  integer DEFAULT 3,
    p_notes          text DEFAULT ''
)
RETURNS public.licenses
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    v_days integer := GREATEST(1, COALESCE(p_days, 7));
    v_max  integer := GREATEST(0, COALESCE(p_max_companies, 3));
    v_key  text;
    v_row  public.licenses%ROWTYPE;
BEGIN
    v_key := public.generate_readable_key('ATLOB-DEMO');
    INSERT INTO public.licenses (
        license_key, plan_type, plan_label, client_name, client_contact,
        status, duration_days, max_companies, notes
    )
    VALUES (
        v_key, 'demo', format('Demo %s días', v_days),
        btrim(COALESCE(p_client_name, '')), btrim(COALESCE(p_client_contact, '')),
        'active', v_days, v_max, btrim(COALESCE(p_notes, ''))
    )
    RETURNING * INTO v_row;

    INSERT INTO public.license_events (license_id, license_key, event_type, detail)
    VALUES (v_row.id, v_row.license_key, 'created', format('Demo creada (%s días, máx %s empresas)', v_days, v_max));

    RETURN v_row;
END;
$$;

CREATE OR REPLACE FUNCTION public.create_commercial_license(
    p_client_name    text,
    p_plan_type      text DEFAULT 'anual',
    p_days           integer DEFAULT 365,
    p_max_companies  integer DEFAULT 0,
    p_client_contact text DEFAULT '',
    p_notes          text DEFAULT ''
)
RETURNS public.licenses
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    v_plan  text := lower(btrim(COALESCE(p_plan_type, 'anual')));
    v_days  integer := p_days;
    v_max   integer := GREATEST(0, COALESCE(p_max_companies, 0));
    v_label text;
    v_key   text;
    v_row   public.licenses%ROWTYPE;
BEGIN
    IF v_plan NOT IN ('mensual', 'anual', 'permanente') THEN
        RAISE EXCEPTION 'Plan inválido: %. Use mensual, anual o permanente.', v_plan;
    END IF;

    IF v_plan = 'permanente' THEN
        v_days := NULL;
        v_label := 'Licencia Permanente';
    ELSIF v_plan = 'mensual' THEN
        v_days := GREATEST(1, COALESCE(v_days, 30));
        v_label := format('Plan Mensual (%s días)', v_days);
    ELSE
        v_days := GREATEST(1, COALESCE(v_days, 365));
        v_label := format('Plan Anual (%s días)', v_days);
    END IF;

    v_key := public.generate_readable_key('ATLOB-PRO');
    INSERT INTO public.licenses (
        license_key, plan_type, plan_label, client_name, client_contact,
        status, duration_days, max_companies, notes
    )
    VALUES (
        v_key, v_plan, v_label,
        btrim(COALESCE(p_client_name, '')), btrim(COALESCE(p_client_contact, '')),
        'active', v_days, v_max, btrim(COALESCE(p_notes, ''))
    )
    RETURNING * INTO v_row;

    INSERT INTO public.license_events (license_id, license_key, event_type, detail)
    VALUES (v_row.id, v_row.license_key, 'created', format('Licencia comercial %s creada', v_label));

    RETURN v_row;
END;
$$;

CREATE OR REPLACE FUNCTION public.extend_license(
    p_license_key text,
    p_extra_days  integer
)
RETURNS public.licenses
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    v_key text := upper(btrim(COALESCE(p_license_key, '')));
    v_row public.licenses%ROWTYPE;
BEGIN
    IF p_extra_days IS NULL OR p_extra_days <= 0 THEN
        RAISE EXCEPTION 'p_extra_days debe ser mayor a 0';
    END IF;

    UPDATE public.licenses
    SET expires_at = GREATEST(COALESCE(expires_at, now()), now()) + make_interval(days => p_extra_days),
        duration_days = COALESCE(duration_days, 0) + p_extra_days,
        status = 'active',
        updated_at = now()
    WHERE license_key = v_key
    RETURNING * INTO v_row;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Licencia no encontrada: %', v_key;
    END IF;

    INSERT INTO public.license_events (license_id, license_key, event_type, detail)
    VALUES (v_row.id, v_row.license_key, 'extended', format('Extendida +%s días hasta %s', p_extra_days, v_row.expires_at));

    RETURN v_row;
END;
$$;

CREATE OR REPLACE FUNCTION public.revoke_license(
    p_license_key text,
    p_reason      text DEFAULT ''
)
RETURNS public.licenses
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    v_key    text := upper(btrim(COALESCE(p_license_key, '')));
    v_reason text := btrim(COALESCE(p_reason, ''));
    v_row    public.licenses%ROWTYPE;
BEGIN
    UPDATE public.licenses
    SET status = 'revoked',
        notes = CASE
            WHEN v_reason <> '' THEN concat_ws(' | ', NULLIF(notes, ''), 'Revocada: ' || v_reason)
            ELSE notes
        END,
        updated_at = now()
    WHERE license_key = v_key
    RETURNING * INTO v_row;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Licencia no encontrada: %', v_key;
    END IF;

    INSERT INTO public.license_events (license_id, license_key, event_type, detail)
    VALUES (v_row.id, v_row.license_key, 'revoked', COALESCE(NULLIF(v_reason, ''), 'Revocada por administrador'));

    RETURN v_row;
END;
$$;

CREATE OR REPLACE FUNCTION public.reset_license_machine(
    p_license_key text
)
RETURNS public.licenses
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    v_key text := upper(btrim(COALESCE(p_license_key, '')));
    v_row public.licenses%ROWTYPE;
BEGIN
    UPDATE public.licenses
    SET machine_hash = NULL,
        machine_name = '',
        updated_at = now()
    WHERE license_key = v_key
    RETURNING * INTO v_row;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Licencia no encontrada: %', v_key;
    END IF;

    INSERT INTO public.license_events (license_id, license_key, event_type, detail)
    VALUES (v_row.id, v_row.license_key, 'machine_reset', 'Equipo desvinculado para re-activación en otra PC');

    RETURN v_row;
END;
$$;

REVOKE ALL ON FUNCTION public.generate_readable_key(text) FROM PUBLIC, anon, authenticated;
REVOKE ALL ON FUNCTION public.create_demo_license(text, text, integer, integer, text) FROM PUBLIC, anon, authenticated;
REVOKE ALL ON FUNCTION public.create_commercial_license(text, text, integer, integer, text, text) FROM PUBLIC, anon, authenticated;
REVOKE ALL ON FUNCTION public.extend_license(text, integer) FROM PUBLIC, anon, authenticated;
REVOKE ALL ON FUNCTION public.revoke_license(text, text) FROM PUBLIC, anon, authenticated;
REVOKE ALL ON FUNCTION public.reset_license_machine(text) FROM PUBLIC, anon, authenticated;

GRANT EXECUTE ON FUNCTION public.generate_readable_key(text) TO service_role;
GRANT EXECUTE ON FUNCTION public.create_demo_license(text, text, integer, integer, text) TO service_role;
GRANT EXECUTE ON FUNCTION public.create_commercial_license(text, text, integer, integer, text, text) TO service_role;
GRANT EXECUTE ON FUNCTION public.extend_license(text, integer) TO service_role;
GRANT EXECUTE ON FUNCTION public.revoke_license(text, text) TO service_role;
GRANT EXECUTE ON FUNCTION public.reset_license_machine(text) TO service_role;

-- 8. Vista resumen para consulta rápida en SQL Editor
CREATE OR REPLACE VIEW public.v_licenses_dashboard
WITH (security_invoker = on) AS
SELECT
    license_key,
    client_name,
    client_contact,
    plan_type,
    plan_label,
    status,
    CASE
        WHEN expires_at IS NULL THEN NULL
        ELSE GREATEST(0, ceil(extract(epoch FROM (expires_at - now())) / 86400.0)::int)
    END AS dias_restantes,
    max_companies,
    companies_count,
    machine_name,
    activated_at,
    expires_at,
    last_seen_at,
    heartbeat_count,
    app_version,
    notes,
    created_at
FROM public.licenses
ORDER BY created_at DESC;

REVOKE ALL ON public.v_licenses_dashboard FROM PUBLIC, anon, authenticated;
GRANT SELECT ON public.v_licenses_dashboard TO service_role;
