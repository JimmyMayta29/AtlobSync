# Diagnostico_SIRE.ps1
# Prueba INDEPENDIENTE del sistema (sin Go, sin sire_api.exe).
# Hace UN pedido de token y 3 llamadas de prueba a "periodos" con ese mismo token.
# No guarda ni imprime la clave SOL, el client_secret ni el token.
# Texto sin tildes a proposito, para evitar problemas de codificacion en Windows PowerShell 5.1.

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

function Read-Plain([string]$prompt) {
    $s = Read-Host $prompt -AsSecureString
    $b = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($s)
    try { return [Runtime.InteropServices.Marshal]::PtrToStringAuto($b) }
    finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($b) }
}

function Get-ErrorInfo($err) {
    # Devuelve @{ Status; Body; Headers } tanto para PowerShell 5.1 como 7+
    $status = -1; $body = ''; $headers = $null
    $ex = $err.Exception
    if ($ex.Response) {
        try { $status = [int]$ex.Response.StatusCode } catch {}
        try { $headers = $ex.Response.Headers } catch {}
    }
    if ($err.ErrorDetails -and $err.ErrorDetails.Message) {
        $body = $err.ErrorDetails.Message
    } elseif ($ex.Response -and $ex.Response.GetResponseStream) {
        try {
            $sr = New-Object IO.StreamReader($ex.Response.GetResponseStream())
            $body = $sr.ReadToEnd()
        } catch {}
    }
    if (-not $body) { $body = $ex.Message }
    return @{ Status = $status; Body = $body; Headers = $headers }
}

function Strip-Html([string]$t) {
    if (-not $t) { return '' }
    $t = [regex]::Replace($t, '<script[\s\S]*?</script>|<style[\s\S]*?</style>', ' ')
    $t = [regex]::Replace($t, '<[^>]+>', ' ')
    $t = [regex]::Replace($t, '\s+', ' ').Trim()
    if ($t.Length -gt 200) { $t = $t.Substring(0, 200) + '...' }
    return $t
}

function Show-Headers($headers) {
    if (-not $headers) { Write-Host '   (sin cabeceras disponibles)'; return }
    foreach ($name in 'Server', 'WWW-Authenticate', 'Content-Type', 'Date') {
        $val = $null
        try { $val = $headers[$name] } catch {}
        if ($val) { Write-Host ("   {0}: {1}" -f $name, ($val -join ', ')) }
    }
}

Write-Host ''
Write-Host '=== DIAGNOSTICO SIRE (independiente del sistema) ===' -ForegroundColor Cyan
Write-Host 'ATENCION: esto hace UN intento real de login en SUNAT.'
Write-Host 'No lo ejecute varias veces seguidas: los intentos fallidos pueden bloquear el usuario SOL.'
$ok = Read-Host 'Escriba SI para continuar'
if ($ok -ne 'SI') { Write-Host 'Cancelado.'; return }

$ruc          = (Read-Host 'RUC (11 digitos)').Trim()
$usuario      = (Read-Host 'Usuario SOL').Trim()
$clave        = Read-Plain 'Clave SOL (no se muestra)'
$clientId     = (Read-Host 'Client ID').Trim()
$clientSecret = Read-Plain 'Client Secret (no se muestra)'

# ---------- PASO 1: token ----------
Write-Host ''
Write-Host '--- PASO 1: pedir token (1 intento) ---' -ForegroundColor Cyan
$tokenUrl = "https://api-seguridad.sunat.gob.pe/v1/clientessol/$clientId/oauth2/token/"
$pairs = @(
    @('grant_type',    'password'),
    @('scope',         'https://api-sire.sunat.gob.pe'),
    @('client_id',     $clientId),
    @('client_secret', $clientSecret),
    @('username',      "$ruc$usuario"),
    @('password',      $clave)
)
$bodyStr = ($pairs | ForEach-Object { '{0}={1}' -f $_[0], [uri]::EscapeDataString($_[1]) }) -join '&'

$token = $null
try {
    $r = Invoke-WebRequest -Uri $tokenUrl -Method Post -Body $bodyStr `
         -ContentType 'application/x-www-form-urlencoded' -UseBasicParsing
    $j = $r.Content | ConvertFrom-Json
    $token = $j.access_token
    Write-Host ("   HTTP {0}. Token recibido. expires_in={1}s" -f [int]$r.StatusCode, $j.expires_in) -ForegroundColor Green
} catch {
    $e = Get-ErrorInfo $_
    Write-Host ("   FALLO al pedir token. HTTP {0}" -f $e.Status) -ForegroundColor Red
    Write-Host ("   Respuesta de SUNAT: {0}" -f (Strip-Html $e.Body))
    Write-Host '   El problema esta en la AUTENTICACION (usuario/clave/client_id/client_secret), no en SIRE.'
    $clave = $null; $clientSecret = $null; $bodyStr = $null
    return
}
$clave = $null; $clientSecret = $null; $bodyStr = $null

# ---------- Claims del token (sin valores personales) ----------
Write-Host ''
Write-Host '--- Datos del token (sin valores personales) ---' -ForegroundColor Cyan
try {
    $p = $token.Split('.')[1].Replace('-', '+').Replace('_', '/')
    switch ($p.Length % 4) { 2 { $p += '==' } 3 { $p += '=' } }
    $claims = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($p)) | ConvertFrom-Json
    Write-Host ("   iss: {0}" -f $claims.iss)
    Write-Host ("   aud: {0}" -f ($claims.aud | ConvertTo-Json -Compress -Depth 6))
    $exp = [DateTimeOffset]::FromUnixTimeSeconds([int64]$claims.exp).ToLocalTime()
    Write-Host ("   vence: {0}" -f $exp)
    if ($claims.userdata) {
        Write-Host ("   claves de userdata: {0}" -f (($claims.userdata.PSObject.Properties.Name) -join ', '))
    }
} catch {
    Write-Host '   (no se pudo decodificar el token; no es critico)'
}

# ---------- PASO 2: tres llamadas con el MISMO token ----------
$url = 'https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/padron/web/omisos/140000/periodos'
$resultados = @()

function Test-Call([string]$label, [hashtable]$headers) {
    Write-Host ''
    Write-Host ("--- {0} ---" -f $label) -ForegroundColor Cyan
    $status = -1; $body = ''; $hdrs = $null
    try {
        $r = Invoke-WebRequest -Uri $url -Method Get -Headers $headers -UseBasicParsing
        $status = [int]$r.StatusCode; $body = $r.Content; $hdrs = $r.Headers
    } catch {
        $e = Get-ErrorInfo $_
        $status = $e.Status; $body = $e.Body; $hdrs = $e.Headers
    }
    $color = 'Red'; if ($status -eq 200) { $color = 'Green' }
    Write-Host ("   HTTP {0}" -f $status) -ForegroundColor $color
    Show-Headers $hdrs
    Write-Host ("   Cuerpo: {0}" -f (Strip-Html $body))
    return $status
}

Write-Host ''
Write-Host '--- PASO 2: llamar a "periodos" (mismo token, sin nuevo login) ---' -ForegroundColor Cyan

$hA = @{ 'Authorization' = "Bearer $token"; 'Accept' = 'application/json'; 'Content-Type' = 'application/json' }
$resultados += ,@('A (cabeceras del manual, UA de PowerShell)', (Test-Call 'A: cabeceras exactas del manual' $hA))

$hB = @{ 'Authorization' = "Bearer $token"; 'Accept' = 'application/json'; 'Content-Type' = 'application/json'; 'User-Agent' = 'go-resty/2.16.5 (https://github.com/go-resty/resty)' }
$resultados += ,@('B (igual que A, UA tipo Go)', (Test-Call 'B: igual que A pero con User-Agent tipo Go (como el sistema)' $hB))

$hC = @{ 'Authorization' = "Bearer $token"; 'Accept' = 'application/json' }
$resultados += ,@('C (sin Content-Type)', (Test-Call 'C: sin Content-Type (solo Accept y Authorization)' $hC))

$token = $null

# ---------- Resumen ----------
Write-Host ''
Write-Host '=== RESUMEN (copie y pegue esto; no contiene secretos) ===' -ForegroundColor Cyan
foreach ($x in $resultados) { Write-Host ("   {0}: HTTP {1}" -f $x[0], $x[1]) }
Write-Host ''
Write-Host ('Hora local: ' + (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'))
Write-Host 'Interpretacion:'
Write-Host '  - A=200 y B=401  -> el gateway rechaza el User-Agent del sistema; se corrige en el cliente Go.'
Write-Host '  - A=200 y B=200  -> el problema esta en otra diferencia del cliente Go; se compara peticion a peticion.'
Write-Host '  - A, B y C = 401 -> el problema NO es del sistema: es de la cuenta/aplicacion ante SUNAT. Escalar a SUNAT.'
