// ============================================================================
// ATLOB Sync · SUNAT v2.0 — Worker Comercial de Licencias (Cloudflare + D1)
// ============================================================================

const KEY_ALPHABET = "23456789ABCDEFGHJKMNPQRSTUVWXYZ";
const SUPPORT_PHONE = "940 781 298";
const MS_PER_DAY = 86400 * 1000;
const MS_PER_HOUR = 3600 * 1000;

export default {
  async fetch(request, env, ctx) {
    try {
      const url = new URL(request.url);
      const path = url.pathname.replace(/\/+$/, "") || "/";
      const method = request.method.toUpperCase();

      if (method === "GET" && path === "/health") {
        return jsonResponse({
          ok: true,
          service: "atlob-sync-licencias",
          server_time: new Date().toISOString(),
        });
      }

      if (!env.DB) {
        return jsonResponse({ message: "Base de datos D1 no enlazada." }, 500);
      }

      const providedToken = extractToken(request);
      if (!providedToken) {
        return jsonResponse({ message: "API key requerida." }, 401);
      }

      const [isClient, isAdmin] = await Promise.all([
        verifySecret(providedToken, env.CLIENT_API_KEY),
        verifySecret(providedToken, env.ADMIN_API_KEY),
      ]);

      if (!isClient && !isAdmin) {
        return jsonResponse({ message: "API key inválida." }, 401);
      }

      // Rutas públicas para la aplicación cliente (solo CLIENT_API_KEY)
      if (method === "POST" && path === "/rest/v1/rpc/activate_license") {
        if (!isClient) {
          return jsonResponse({ message: "Se requiere CLIENT_API_KEY para activar." }, 403);
        }
        const body = await readJsonBody(request);
        return await handleActivateLicense(env.DB, body);
      }

      if (method === "POST" && path === "/rest/v1/rpc/validate_license") {
        if (!isClient) {
          return jsonResponse({ message: "Se requiere CLIENT_API_KEY para validar." }, 403);
        }
        const body = await readJsonBody(request);
        return await handleValidateLicense(env.DB, body);
      }

      // Rutas administrativas (exclusivas para ADMIN_API_KEY)
      const isAdminRoute =
        (method === "POST" &&
          [
            "/rest/v1/rpc/create_demo_license",
            "/rest/v1/rpc/create_commercial_license",
            "/rest/v1/rpc/extend_license",
            "/rest/v1/rpc/revoke_license",
            "/rest/v1/rpc/reset_license_machine",
          ].includes(path)) ||
        ((method === "GET" || method === "DELETE") && path === "/rest/v1/licenses");

      if (!isAdminRoute) {
        return jsonResponse({ message: "Ruta no encontrada." }, 404);
      }

      if (!isAdmin) {
        return jsonResponse({ message: "Acceso restringido a operaciones administrativas." }, 403);
      }

      if (method === "GET" && path === "/rest/v1/licenses") {
        return await handleListLicenses(env.DB, url);
      }

      if (method === "DELETE" && path === "/rest/v1/licenses") {
        return await handleDeleteLicense(env.DB, url);
      }

      const body = await readJsonBody(request);

      if (path === "/rest/v1/rpc/create_demo_license") {
        return await handleCreateDemoLicense(env.DB, body);
      }
      if (path === "/rest/v1/rpc/create_commercial_license") {
        return await handleCreateCommercialLicense(env.DB, body);
      }
      if (path === "/rest/v1/rpc/extend_license") {
        return await handleExtendLicense(env.DB, body);
      }
      if (path === "/rest/v1/rpc/revoke_license") {
        return await handleRevokeLicense(env.DB, body);
      }
      if (path === "/rest/v1/rpc/reset_license_machine") {
        return await handleResetMachine(env.DB, body);
      }

      return jsonResponse({ message: "Ruta no encontrada." }, 404);
    } catch (error) {
      const msg = error instanceof Error ? error.message : "Error interno";
      const status = error && typeof error === "object" && Number.isInteger(error.httpStatus)
        ? error.httpStatus
        : 500;
      if (status >= 500) {
        console.error(JSON.stringify({ event: "unhandled_error", error: msg, url: request.url }));
      }
      return jsonResponse({ message: msg }, status);
    }
  },
};

function jsonResponse(data, status = 200) {
  return new Response(JSON.stringify(data), {
    status,
    headers: {
      "Content-Type": "application/json; charset=utf-8",
      "Cache-Control": "no-store",
    },
  });
}

function httpError(message, httpStatus = 400) {
  const err = new Error(message);
  err.httpStatus = httpStatus;
  return err;
}

async function readJsonBody(request) {
  const contentLen = Number(request.headers.get("content-length") || "0");
  if (contentLen > 65536) {
    throw httpError("Cuerpo demasiado grande.", 413);
  }
  const text = await request.text();
  if (!text || !text.trim()) {
    return {};
  }
  try {
    const parsed = JSON.parse(text);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      throw httpError("Se esperaba un objeto JSON.", 400);
    }
    return parsed;
  } catch (e) {
    if (e && e.httpStatus) throw e;
    throw httpError("JSON inválido.", 400);
  }
}

function extractToken(request) {
  const apiKey = (request.headers.get("apikey") || "").trim();
  if (apiKey) return apiKey;

  const auth = (request.headers.get("authorization") || "").trim();
  if (auth.toLowerCase().startsWith("bearer ")) {
    return auth.slice(7).trim();
  }
  return "";
}

async function verifySecret(provided, expected) {
  if (!provided || !expected) return false;
  const encoder = new TextEncoder();
  const [providedDigest, expectedDigest] = await Promise.all([
    crypto.subtle.digest("SHA-256", encoder.encode(String(provided))),
    crypto.subtle.digest("SHA-256", encoder.encode(String(expected))),
  ]);
  return crypto.subtle.timingSafeEqual(providedDigest, expectedDigest);
}

function normalizeKey(raw) {
  return String(raw || "").trim().toUpperCase();
}

function normalizeMachineHash(raw) {
  return String(raw || "").trim().toLowerCase();
}

function cleanText(raw, maxLen = 120) {
  return String(raw || "").trim().slice(0, maxLen);
}

function buildRpcResponse(ok, code, message, row, nowDate) {
  const nowMs = nowDate.getTime();
  let daysRemaining = -1;
  let hoursRemaining = -1;

  if (row && row.expires_at) {
    const expMs = Date.parse(row.expires_at);
    if (!Number.isNaN(expMs)) {
      const diff = expMs - nowMs;
      daysRemaining = diff <= 0 ? 0 : Math.ceil(diff / MS_PER_DAY);
      hoursRemaining = diff <= 0 ? 0 : Math.ceil(diff / MS_PER_HOUR);
    }
  }

  return {
    ok: Boolean(ok),
    code,
    status: row?.status || "",
    license_key: row?.license_key || "",
    plan_type: row?.plan_type || "",
    plan_label: row?.plan_label || "",
    client_name: row?.client_name || "",
    max_companies: Number(row?.max_companies ?? 0),
    duration_days: row?.duration_days ?? null,
    activated_at: row?.activated_at ?? null,
    expires_at: row?.expires_at ?? null,
    days_remaining: daysRemaining,
    hours_remaining: hoursRemaining,
    server_time: nowDate.toISOString(),
    message,
  };
}

function formatLicenseRow(row) {
  if (!row) return null;
  return {
    id: row.id,
    license_key: row.license_key,
    plan_type: row.plan_type,
    plan_label: row.plan_label,
    client_name: row.client_name || "",
    client_contact: row.client_contact || "",
    status: row.status,
    duration_days: row.duration_days === null || row.duration_days === undefined ? null : Number(row.duration_days),
    max_companies: Number(row.max_companies ?? 0),
    machine_hash: row.machine_hash ?? null,
    machine_name: row.machine_name || "",
    activated_at: row.activated_at ?? null,
    expires_at: row.expires_at ?? null,
    last_seen_at: row.last_seen_at ?? null,
    companies_count: Number(row.companies_count ?? 0),
    activation_count: Number(row.activation_count ?? 0),
    heartbeat_count: Number(row.heartbeat_count ?? 0),
    notes: row.notes || "",
    created_at: row.created_at,
  };
}

async function recordEvent(db, { licenseId, licenseKey, eventType, machineHash = "", machineName = "", appVersion = "", detail = "", nowISO }) {
  await db
    .prepare(
      `INSERT INTO license_events (license_id, license_key, event_type, machine_hash, machine_name, app_version, detail, created_at)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
    )
    .bind(licenseId ?? null, licenseKey, eventType, machineHash, machineName, appVersion, detail, nowISO)
    .run();
}

async function handleActivateLicense(db, body) {
  const key = normalizeKey(body.p_license_key);
  const machineHash = normalizeMachineHash(body.p_machine_hash);
  const machineName = cleanText(body.p_machine_name, 64);
  const appVersion = cleanText(body.p_app_version, 32);
  const nowDate = new Date();
  const nowISO = nowDate.toISOString();

  if (!key || key.length < 6) {
    return jsonResponse({
      ok: false,
      code: "NOT_FOUND",
      status: "",
      server_time: nowISO,
      message: "Ingresa un código de licencia válido.",
    });
  }

  if (!machineHash || machineHash.length < 16) {
    return jsonResponse({
      ok: false,
      code: "INVALID_MACHINE",
      status: "",
      server_time: nowISO,
      message: "No se pudo identificar este equipo de forma segura.",
    });
  }

  let row = await db.prepare("SELECT * FROM licenses WHERE license_key = ? LIMIT 1").bind(key).first();
  if (!row) {
    await recordEvent(db, {
      licenseId: null,
      licenseKey: key,
      eventType: "not_found",
      machineHash,
      machineName,
      appVersion,
      detail: "Intento de activación con código inexistente",
      nowISO,
    });
    return jsonResponse({
      ok: false,
      code: "NOT_FOUND",
      status: "",
      license_key: key,
      server_time: nowISO,
      message: "El código de licencia no existe. Verifica que esté bien escrito.",
    });
  }

  if (row.status === "revoked" || row.status === "suspended") {
    await recordEvent(db, {
      licenseId: row.id,
      licenseKey: row.license_key,
      eventType: row.status,
      machineHash,
      machineName,
      appVersion,
      detail: `Intento de activación en licencia ${row.status}`,
      nowISO,
    });
    const msg =
      row.status === "revoked"
        ? `Esta licencia fue revocada. Contáctanos al ${SUPPORT_PHONE} para soporte o renovación.`
        : `Esta licencia se encuentra suspendida temporalmente. Contáctanos al ${SUPPORT_PHONE}.`;
    return jsonResponse(buildRpcResponse(false, row.status.toUpperCase(), msg, row, nowDate));
  }

  if (row.expires_at && nowDate.getTime() >= Date.parse(row.expires_at)) {
    await db.prepare("UPDATE licenses SET status = 'expired', updated_at = ? WHERE id = ?").bind(nowISO, row.id).run();
    row = await db.prepare("SELECT * FROM licenses WHERE id = ?").bind(row.id).first();
    await recordEvent(db, {
      licenseId: row.id,
      licenseKey: row.license_key,
      eventType: "expired",
      machineHash,
      machineName,
      appVersion,
      detail: "Licencia expirada al intentar activar",
      nowISO,
    });
    return jsonResponse(
      buildRpcResponse(
        false,
        "EXPIRED",
        `Tu periodo de prueba o licencia ha finalizado. Tus datos siguen intactos en tu PC. Escríbenos al ${SUPPORT_PHONE} para activar un plan.`,
        row,
        nowDate
      )
    );
  }

  if (!row.machine_hash) {
    let newExpiresAt = row.expires_at ?? null;
    if (!newExpiresAt && row.duration_days !== null && row.duration_days !== undefined && Number(row.duration_days) > 0) {
      newExpiresAt = new Date(nowDate.getTime() + Number(row.duration_days) * MS_PER_DAY).toISOString();
    }

    const activatedAt = row.activated_at || nowISO;
    const res = await db
      .prepare(
        `UPDATE licenses
         SET machine_hash = ?,
             machine_name = CASE WHEN ? <> '' THEN ? ELSE machine_name END,
             app_version = CASE WHEN ? <> '' THEN ? ELSE app_version END,
             activated_at = ?,
             expires_at = ?,
             last_seen_at = ?,
             activation_count = activation_count + 1,
             updated_at = ?
         WHERE id = ? AND machine_hash IS NULL`
      )
      .bind(machineHash, machineName, machineName, appVersion, appVersion, activatedAt, newExpiresAt, nowISO, nowISO, row.id)
      .run();

    if (res.meta && res.meta.changes === 1) {
      row = await db.prepare("SELECT * FROM licenses WHERE id = ?").bind(row.id).first();
      await recordEvent(db, {
        licenseId: row.id,
        licenseKey: row.license_key,
        eventType: "activated",
        machineHash,
        machineName,
        appVersion,
        detail: "Primera activación completada",
        nowISO,
      });
      return jsonResponse(buildRpcResponse(true, "ACTIVE", "Licencia activada correctamente en este equipo.", row, nowDate));
    }

    // Otra solicitud concurrente ganó la primera activación
    row = await db.prepare("SELECT * FROM licenses WHERE id = ?").bind(row.id).first();
  }

  if (row.machine_hash !== machineHash) {
    await recordEvent(db, {
      licenseId: row.id,
      licenseKey: row.license_key,
      eventType: "machine_mismatch",
      machineHash,
      machineName,
      appVersion,
      detail: `Bloqueado: licencia vinculada a otro equipo (${row.machine_name || row.machine_hash.slice(0, 8)})`,
      nowISO,
    });
    return jsonResponse(
      buildRpcResponse(
        false,
        "MACHINE_MISMATCH",
        `Esta licencia ya está activada en otro equipo (${row.machine_name || "PC registrada"}). Si cambiaste de PC escríbenos al ${SUPPORT_PHONE}.`,
        row,
        nowDate
      )
    );
  }

  await db
    .prepare(
      `UPDATE licenses
       SET machine_name = CASE WHEN ? <> '' THEN ? ELSE machine_name END,
           app_version = CASE WHEN ? <> '' THEN ? ELSE app_version END,
           last_seen_at = ?,
           heartbeat_count = heartbeat_count + 1,
           updated_at = ?
       WHERE id = ?`
    )
    .bind(machineName, machineName, appVersion, appVersion, nowISO, nowISO, row.id)
    .run();

  row = await db.prepare("SELECT * FROM licenses WHERE id = ?").bind(row.id).first();
  await recordEvent(db, {
    licenseId: row.id,
    licenseKey: row.license_key,
    eventType: "reactivated",
    machineHash,
    machineName,
    appVersion,
    detail: "Sesión reactivada en el equipo autorizado",
    nowISO,
  });

  return jsonResponse(buildRpcResponse(true, "ACTIVE", "Licencia verificada correctamente en este equipo.", row, nowDate));
}

async function handleValidateLicense(db, body) {
  const key = normalizeKey(body.p_license_key);
  const machineHash = normalizeMachineHash(body.p_machine_hash);
  const machineName = cleanText(body.p_machine_name, 64);
  const appVersion = cleanText(body.p_app_version, 32);
  const companiesCount = Math.max(0, Number.parseInt(body.p_companies_count ?? 0, 10) || 0);
  const nowDate = new Date();
  const nowISO = nowDate.toISOString();

  let row = await db.prepare("SELECT * FROM licenses WHERE license_key = ? LIMIT 1").bind(key).first();
  if (!row) {
    return jsonResponse({
      ok: false,
      code: "NOT_FOUND",
      status: "",
      license_key: key,
      server_time: nowISO,
      message: "La licencia ya no existe en el servidor.",
    });
  }

  if (row.status === "revoked" || row.status === "suspended") {
    const msg =
      row.status === "revoked"
        ? `Tu licencia fue revocada. Contáctanos al ${SUPPORT_PHONE}.`
        : `Tu licencia está suspendida temporalmente. Contáctanos al ${SUPPORT_PHONE}.`;
    return jsonResponse(buildRpcResponse(false, row.status.toUpperCase(), msg, row, nowDate));
  }

  if (!row.machine_hash) {
    return jsonResponse(
      buildRpcResponse(false, "NOT_ACTIVATED", "Licencia pendiente de activación en este equipo.", row, nowDate)
    );
  }

  if (row.machine_hash !== machineHash) {
    await recordEvent(db, {
      licenseId: row.id,
      licenseKey: row.license_key,
      eventType: "machine_mismatch",
      machineHash,
      machineName,
      appVersion,
      detail: "Validación rechazada por cambio de equipo",
      nowISO,
    });
    return jsonResponse(
      buildRpcResponse(
        false,
        "MACHINE_MISMATCH",
        `Esta licencia corresponde a otro equipo (${row.machine_name || "PC registrada"}). Contáctanos al ${SUPPORT_PHONE}.`,
        row,
        nowDate
      )
    );
  }

  if (row.expires_at && nowDate.getTime() >= Date.parse(row.expires_at)) {
    await db
      .prepare(
        `UPDATE licenses
         SET status = 'expired',
             last_seen_at = ?,
             companies_count = ?,
             updated_at = ?
         WHERE id = ?`
      )
      .bind(nowISO, companiesCount, nowISO, row.id)
      .run();

    row = await db.prepare("SELECT * FROM licenses WHERE id = ?").bind(row.id).first();
    await recordEvent(db, {
      licenseId: row.id,
      licenseKey: row.license_key,
      eventType: "expired",
      machineHash,
      machineName,
      appVersion,
      detail: "Licencia expirada detectada durante validación",
      nowISO,
    });
    return jsonResponse(
      buildRpcResponse(
        false,
        "EXPIRED",
        `Tu periodo de prueba o licencia ha finalizado. Tus datos siguen guardados en tu PC. Escríbenos al ${SUPPORT_PHONE} para contratar o renovar.`,
        row,
        nowDate
      )
    );
  }

  await db
    .prepare(
      `UPDATE licenses
       SET status = 'active',
           last_seen_at = ?,
           companies_count = ?,
           machine_name = CASE WHEN ? <> '' THEN ? ELSE machine_name END,
           app_version = CASE WHEN ? <> '' THEN ? ELSE app_version END,
           heartbeat_count = heartbeat_count + 1,
           updated_at = ?
       WHERE id = ?`
    )
    .bind(nowISO, companiesCount, machineName, machineName, appVersion, appVersion, nowISO, row.id)
    .run();

  row = await db.prepare("SELECT * FROM licenses WHERE id = ?").bind(row.id).first();
  return jsonResponse(buildRpcResponse(true, "ACTIVE", "Licencia vigente.", row, nowDate));
}

function generateReadableKey(prefix) {
  const bytes = new Uint8Array(8);
  crypto.getRandomValues(bytes);
  let p1 = "";
  let p2 = "";
  for (let i = 0; i < 4; i++) {
    p1 += KEY_ALPHABET[bytes[i] % KEY_ALPHABET.length];
    p2 += KEY_ALPHABET[bytes[i + 4] % KEY_ALPHABET.length];
  }
  return `${prefix}-${p1}-${p2}`;
}

async function generateUniqueKey(db, prefix) {
  for (let attempt = 0; attempt < 10; attempt++) {
    const candidate = generateReadableKey(prefix);
    const exists = await db.prepare("SELECT 1 FROM licenses WHERE license_key = ? LIMIT 1").bind(candidate).first();
    if (!exists) return candidate;
  }
  throw new Error("No se pudo generar un código único.");
}

async function handleCreateDemoLicense(db, body) {
  const clientName = cleanText(body.p_client_name, 120);
  if (!clientName) {
    throw httpError("p_client_name es obligatorio.", 400);
  }
  const clientContact = cleanText(body.p_client_contact, 120);
  const days = Math.max(1, Number.parseInt(body.p_days ?? 7, 10) || 7);
  const parsedMax = body.p_max_companies === undefined || body.p_max_companies === null
    ? 3
    : Number.parseInt(body.p_max_companies, 10);
  const maxCompanies = Math.max(0, Number.isNaN(parsedMax) ? 3 : parsedMax);
  const notes = cleanText(body.p_notes, 300);

  const id = crypto.randomUUID();
  const licenseKey = await generateUniqueKey(db, "ATLOB-DEMO");
  const planLabel = `Demo ${days} días`;
  const nowISO = new Date().toISOString();

  await db
    .prepare(
      `INSERT INTO licenses (
        id, license_key, plan_type, plan_label, client_name, client_contact,
        status, duration_days, max_companies, notes, created_at, updated_at
      ) VALUES (?, ?, 'demo', ?, ?, ?, 'active', ?, ?, ?, ?, ?)`
    )
    .bind(id, licenseKey, planLabel, clientName, clientContact, days, maxCompanies, notes, nowISO, nowISO)
    .run();

  await recordEvent(db, {
    licenseId: id,
    licenseKey,
    eventType: "created",
    detail: `Demo creada (${days} días, máx ${maxCompanies} empresas)`,
    nowISO,
  });

  const created = await db.prepare("SELECT * FROM licenses WHERE id = ?").bind(id).first();
  return jsonResponse(formatLicenseRow(created));
}

async function handleCreateCommercialLicense(db, body) {
  const clientName = cleanText(body.p_client_name, 120);
  if (!clientName) {
    throw httpError("p_client_name es obligatorio.", 400);
  }
  const plan = cleanText(body.p_plan_type || "anual", 24).toLowerCase();
  if (!["mensual", "anual", "permanente"].includes(plan)) {
    throw httpError(`Plan inválido: ${plan}. Use mensual, anual o permanente.`, 400);
  }

  let days = null;
  let planLabel = "Licencia Permanente";
  if (plan === "mensual") {
    days = Math.max(1, Number.parseInt(body.p_days ?? 30, 10) || 30);
    planLabel = `Plan Mensual (${days} días)`;
  } else if (plan === "anual") {
    days = Math.max(1, Number.parseInt(body.p_days ?? 365, 10) || 365);
    planLabel = `Plan Anual (${days} días)`;
  }

  const parsedMax = body.p_max_companies === undefined || body.p_max_companies === null
    ? 0
    : Number.parseInt(body.p_max_companies, 10);
  const maxCompanies = Math.max(0, Number.isNaN(parsedMax) ? 0 : parsedMax);
  const clientContact = cleanText(body.p_client_contact, 120);
  const notes = cleanText(body.p_notes, 300);

  const id = crypto.randomUUID();
  const licenseKey = await generateUniqueKey(db, "ATLOB-PRO");
  const nowISO = new Date().toISOString();

  await db
    .prepare(
      `INSERT INTO licenses (
        id, license_key, plan_type, plan_label, client_name, client_contact,
        status, duration_days, max_companies, notes, created_at, updated_at
      ) VALUES (?, ?, ?, ?, ?, ?, 'active', ?, ?, ?, ?, ?)`
    )
    .bind(id, licenseKey, plan, planLabel, clientName, clientContact, days, maxCompanies, notes, nowISO, nowISO)
    .run();

  await recordEvent(db, {
    licenseId: id,
    licenseKey,
    eventType: "created",
    detail: `Licencia comercial ${planLabel} creada`,
    nowISO,
  });

  const created = await db.prepare("SELECT * FROM licenses WHERE id = ?").bind(id).first();
  return jsonResponse(formatLicenseRow(created));
}

async function handleExtendLicense(db, body) {
  const key = normalizeKey(body.p_license_key);
  const extraDays = Number.parseInt(body.p_extra_days, 10);
  if (!key || Number.isNaN(extraDays) || extraDays <= 0) {
    throw httpError("p_license_key y p_extra_days (> 0) son obligatorios.", 400);
  }

  let row = await db.prepare("SELECT * FROM licenses WHERE license_key = ? LIMIT 1").bind(key).first();
  if (!row) {
    throw httpError(`Licencia no encontrada: ${key}`, 404);
  }

  const nowDate = new Date();
  const nowMs = nowDate.getTime();
  const nowISO = nowDate.toISOString();
  const baseMs = row.expires_at ? Math.max(nowMs, Date.parse(row.expires_at) || nowMs) : nowMs;
  const newExpiresISO = new Date(baseMs + extraDays * MS_PER_DAY).toISOString();
  const newDuration = (Number(row.duration_days ?? 0)) + extraDays;

  await db
    .prepare(
      `UPDATE licenses
       SET expires_at = ?,
           duration_days = ?,
           status = 'active',
           updated_at = ?
       WHERE id = ?`
    )
    .bind(newExpiresISO, newDuration, nowISO, row.id)
    .run();

  row = await db.prepare("SELECT * FROM licenses WHERE id = ?").bind(row.id).first();
  await recordEvent(db, {
    licenseId: row.id,
    licenseKey: row.license_key,
    eventType: "extended",
    detail: `Extendida +${extraDays} días hasta ${newExpiresISO}`,
    nowISO,
  });

  return jsonResponse(formatLicenseRow(row));
}

async function handleRevokeLicense(db, body) {
  const key = normalizeKey(body.p_license_key);
  const reason = cleanText(body.p_reason, 200);
  if (!key) {
    throw httpError("p_license_key es obligatorio.", 400);
  }

  let row = await db.prepare("SELECT * FROM licenses WHERE license_key = ? LIMIT 1").bind(key).first();
  if (!row) {
    throw httpError(`Licencia no encontrada: ${key}`, 404);
  }

  const nowISO = new Date().toISOString();
  const newNotes = reason ? [row.notes, `Revocada: ${reason}`].filter(Boolean).join(" | ") : (row.notes || "");

  await db
    .prepare("UPDATE licenses SET status = 'revoked', notes = ?, updated_at = ? WHERE id = ?")
    .bind(newNotes, nowISO, row.id)
    .run();

  row = await db.prepare("SELECT * FROM licenses WHERE id = ?").bind(row.id).first();
  await recordEvent(db, {
    licenseId: row.id,
    licenseKey: row.license_key,
    eventType: "revoked",
    detail: reason || "Revocada por administrador",
    nowISO,
  });

  return jsonResponse(formatLicenseRow(row));
}

async function handleResetMachine(db, body) {
  const key = normalizeKey(body.p_license_key);
  if (!key) {
    throw httpError("p_license_key es obligatorio.", 400);
  }

  let row = await db.prepare("SELECT * FROM licenses WHERE license_key = ? LIMIT 1").bind(key).first();
  if (!row) {
    throw httpError(`Licencia no encontrada: ${key}`, 404);
  }

  const nowISO = new Date().toISOString();
  await db
    .prepare("UPDATE licenses SET machine_hash = NULL, machine_name = '', updated_at = ? WHERE id = ?")
    .bind(nowISO, row.id)
    .run();

  row = await db.prepare("SELECT * FROM licenses WHERE id = ?").bind(row.id).first();
  await recordEvent(db, {
    licenseId: row.id,
    licenseKey: row.license_key,
    eventType: "machine_reset",
    detail: "Equipo desvinculado para re-activación en otra PC",
    nowISO,
  });

  return jsonResponse(formatLicenseRow(row));
}

async function handleListLicenses(db, url) {
  const limitParam = Number.parseInt(url.searchParams.get("limit") || "100", 10);
  const limit = Math.min(200, Math.max(1, Number.isNaN(limitParam) ? 100 : limitParam));
  const res = await db.prepare("SELECT * FROM licenses ORDER BY created_at DESC LIMIT ?").bind(limit).all();
  const rows = (res.results || []).map(formatLicenseRow);
  return jsonResponse(rows);
}

async function handleDeleteLicense(db, url) {
  let rawKey = (url.searchParams.get("license_key") || "").trim();
  if (rawKey.toLowerCase().startsWith("eq.")) {
    rawKey = rawKey.slice(3);
  }
  const key = normalizeKey(rawKey);
  if (!key) {
    throw httpError("Parámetro license_key obligatorio.", 400);
  }

  await db.prepare("DELETE FROM license_events WHERE license_key = ?").bind(key).run();
  const del = await db.prepare("DELETE FROM licenses WHERE license_key = ?").bind(key).run();
  return jsonResponse({ deleted: del.meta?.changes || 0, license_key: key });
}
