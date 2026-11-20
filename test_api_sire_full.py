import os
import sys
import time
import subprocess
import json
import httpx

API_DIR = r"e:\PROYECTOS SUNAT\SIRE - SUNAT\API-SIRE"
EXE_PATH = os.path.join(API_DIR, "sire_api.exe")
ENV_PATH = os.path.join(API_DIR, ".env")
BASE_URL = "http://localhost:8080"

def log(msg):
    print(f"[*] {msg}", flush=True)

def kill_existing_sire():
    try:
        subprocess.run(["powershell", "-Command", "Get-Process -Name 'sire_api' -ErrorAction SilentlyContinue | Stop-Process -Force"], capture_output=True)
        time.sleep(1)
    except Exception as e:
        log(f"Error killing existing process: {e}")

def start_server():
    log("Iniciando servidor sire_api.exe...")
    proc = subprocess.Popen([EXE_PATH], cwd=API_DIR, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    time.sleep(2)
    return proc

def update_env_token(token):
    with open(ENV_PATH, "r", encoding="utf-8") as f:
        lines = f.readlines()
    
    new_lines = []
    found = False
    for line in lines:
        if line.startswith("TOKEN="):
            new_lines.append(f"TOKEN={token}\n")
            found = True
        else:
            new_lines.append(line)
    if not found:
        new_lines.append(f"TOKEN={token}\n")
        
    with open(ENV_PATH, "w", encoding="utf-8") as f:
        f.writelines(new_lines)
    log(f".env actualizado con TOKEN de longitud {len(token)}")

def test_endpoints():
    log("Iniciando pruebas de endpoints...")
    
    # 1. Obtener Token
    log("--- 1. TEST AUTENTICACION (GET /api/sunat/auth/obtener-token) ---")
    res = httpx.get(f"{BASE_URL}/api/sunat/auth/obtener-token", timeout=20.0)
    log(f"Status: {res.status_code}")
    if res.status_code != 200:
        log(f"Error en autenticacion: {res.text}")
        return False
    
    data = res.json()
    token = data.get("access_token")
    log(f"Token obtenido con exito! (Prefijo: {token[:25]}...)")
    
    # 2. Guardar token y reiniciar servidor para que tome os.Getenv("TOKEN")
    update_env_token(token)
    log("Reiniciando servidor para cargar el nuevo TOKEN...")
    kill_existing_sire()
    start_server()
    
    # Lista de endpoints para probar
    tests = [
        {
            "name": "VENTAS: Consultar Periodos (140000)",
            "method": "GET",
            "url": f"{BASE_URL}/api/sunat/ventas/periodos/140000",
            "params": {}
        },
        {
            "name": "COMPRAS: Consultar Periodos RCE (080000)",
            "method": "GET",
            "url": f"{BASE_URL}/api/sunat/compras/libros/rce/periodos/080000",
            "params": {}
        },
        {
            "name": "VENTAS: Consultar Estado Ticket",
            "method": "GET",
            "url": f"{BASE_URL}/api/sunat/ventas/consultar-estado-ticket",
            "params": {"perIni": "202410", "perFin": "202412", "page": "1", "perPage": "10"}
        },
        {
            "name": "COMPRAS: Consultar Estado Tickets RCE",
            "method": "GET",
            "url": f"{BASE_URL}/api/sunat/compras/libros/rce/tickets/consultar",
            "params": {"perIni": "202410", "perFin": "202412", "page": "1", "perPage": "10"}
        },
        {
            "name": "VENTAS: Reporte de Cumplimiento (202412)",
            "method": "GET",
            "url": f"{BASE_URL}/api/sunat/ventas/reporte-cumplimiento/202412/140000",
            "params": {}
        },
        {
            "name": "COMPRAS: Reporte de Cumplimiento (202412)",
            "method": "GET",
            "url": f"{BASE_URL}/api/sunat/compras/reporte-cumplimiento/202412/080000",
            "params": {}
        },
        {
            "name": "VENTAS: Descargar Propuesta RVIE (202412)",
            "method": "GET",
            "url": f"{BASE_URL}/api/sunat/ventas/descargar-propuesta/202412",
            "params": {"codTipoArchivo": "0", "codOrigenEnvio": "1"}
        }
    ]
    
    results_summary = []
    
    for t in tests:
        log(f"\n--- Probando {t['name']} ---")
        try:
            if t["method"] == "GET":
                r = httpx.get(t["url"], params=t["params"], timeout=30.0)
            else:
                r = httpx.post(t["url"], json=t.get("json", {}), timeout=30.0)
                
            log(f"URL: {r.url}")
            log(f"HTTP Status: {r.status_code}")
            body_preview = r.text[:400]
            log(f"Response Body: {body_preview}")
            results_summary.append({
                "name": t["name"],
                "status": r.status_code,
                "success": r.status_code in [200, 201, 204],
                "preview": body_preview
            })
        except Exception as ex:
            log(f"Error en llamada: {ex}")
            results_summary.append({
                "name": t["name"],
                "status": "EXCEPTION",
                "success": False,
                "preview": str(ex)
            })

    log("\n================================ RESUMEN DE PRUEBAS ================================")
    for res in results_summary:
        estado = "OK" if res["success"] else "FALLO/ALERTA"
        log(f"[{estado}] {res['name']} -> Status: {res['status']}")
    
    return True

if __name__ == "__main__":
    kill_existing_sire()
    proc = start_server()
    try:
        test_endpoints()
    finally:
        log("Fin de pruebas.")

