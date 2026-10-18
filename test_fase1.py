import subprocess
import time
import httpx
import sys

BASE_URL = "http://127.0.0.1:8080"

def log(msg):
    print(f"[*] {msg}", flush=True)

def kill_process():
    try:
        subprocess.run(["powershell", "-Command", "Get-Process -Name 'sire_api' -ErrorAction SilentlyContinue | Stop-Process -Force"], capture_output=True)
        time.sleep(1)
    except Exception as e:
        log(f"Error cerrando procesos previos: {e}")

def main():
    log("=== INICIANDO PRUEBAS DE FASE 1: MULTIEMPRESA, SQLITE Y CIFRADO ===")
    kill_process()
    
    log("Iniciando sire_api.exe...")
    proc = subprocess.Popen([r"C:\PROYECTOS SUNAT\SIRE - SUNAT\API-SIRE\sire_api.exe"], 
                            cwd=r"C:\PROYECTOS SUNAT\SIRE - SUNAT\API-SIRE",
                            stdout=subprocess.PIPE, 
                            stderr=subprocess.PIPE)
    time.sleep(2)

    try:
        # 1. Probar Listado de Empresas (Sembrado automático desde .env)
        log("\n--- TEST 1: GET /api/empresas (Listar empresas) ---")
        res = httpx.get(f"{BASE_URL}/api/empresas", timeout=10.0)
        log(f"Status: {res.status_code}")
        assert res.status_code == 200, f"Error listando empresas: {res.text}"
        data = res.json()["data"]
        log(f"Empresas registradas: {len(data)}")
        for emp in data:
            log(f" -> RUC: {emp['ruc']}, Razon Social: {emp['razon_social']}, ClaveSOL: {emp['clave_sol']}, ClientSecret: {emp['client_secret']}")
            assert emp['clave_sol'] == "******", "Clave SOL debe estar enmascarada"
            assert emp['client_secret'] == "******", "Client secret debe estar enmascarado"

        # 2. Probar Conexión con SUNAT para la empresa sembrada
        if len(data) > 0:
            ruc = data[0]['ruc']
            log(f"\n--- TEST 2: POST /api/empresas/{ruc}/probar-conexion (Probar credenciales SUNAT) ---")
            res = httpx.post(f"{BASE_URL}/api/empresas/{ruc}/probar-conexion", timeout=25.0)
            log(f"Status: {res.status_code}")
            log(f"Respuesta del endpoint: {res.text}")
            resp_json = res.json()
            if res.status_code == 200:
                log(f"[OK] SUNAT aceptó credenciales: {resp_json.get('mensaje')} (Token: {resp_json.get('token_preview')})")
            elif res.status_code == 502:
                log(f"[AVISO] SUNAT respondió con rechazo de credenciales (correcto manejo de error): {resp_json.get('error')}")
                assert "SUNAT" in resp_json.get("error", ""), "El mensaje de error debe provenir de SUNAT"
            else:
                raise AssertionError(f"Status inesperado en probar-conexion: {res.status_code}")

        # 3. Probar Creación de una Segunda Empresa
        log("\n--- TEST 3: POST /api/empresas (Registrar segunda empresa) ---")
        nueva_emp = {
            "ruc": "20555555551",
            "razon_social": "EMPRESA DE PRUEBA SECUNDARIA S.A.C.",
            "usuario_sol": "MODDATOS",
            "clave_sol": "mock_clave_sol",
            "client_id": "mock_client_id",
            "client_secret": "mock_client_secret"
        }
        res = httpx.post(f"{BASE_URL}/api/empresas", json=nueva_emp, timeout=10.0)
        log(f"Status: {res.status_code}")
        log(f"Respuesta: {res.text}")
        assert res.status_code == 201, f"Fallo creando segunda empresa: {res.text}"

        # 4. Probar Obtención por RUC
        log("\n--- TEST 4: GET /api/empresas/20555555551 (Detalle de empresa) ---")
        res = httpx.get(f"{BASE_URL}/api/empresas/20555555551", timeout=10.0)
        log(f"Status: {res.status_code}")
        assert res.status_code == 200, f"Fallo obteniendo detalle: {res.text}"
        detail = res.json()["data"]
        assert detail["ruc"] == "20555555551"
        assert detail["clave_sol"] == "******"

        # 5. Probar Actualización de Empresa
        log("\n--- TEST 5: PUT /api/empresas/20555555551 (Actualizar datos) ---")
        upd = {
            "razon_social": "EMPRESA DE PRUEBA SECUNDARIA EDITADA S.A.C.",
            "usuario_sol": "MODDATOS2",
            "client_id": "client-id-secundario-v2",
            "clave_sol": "",      # Dejar vacía para conservar la anterior cifrada
            "client_secret": ""
        }
        res = httpx.put(f"{BASE_URL}/api/empresas/20555555551", json=upd, timeout=10.0)
        log(f"Status: {res.status_code}")
        assert res.status_code == 200, f"Fallo actualizando empresa: {res.text}"
        assert res.json()["data"]["razon_social"] == "EMPRESA DE PRUEBA SECUNDARIA EDITADA S.A.C."

        # 6. Probar Eliminación (Soft-delete)
        log("\n--- TEST 6: DELETE /api/empresas/20555555551 (Soft delete) ---")
        res = httpx.delete(f"{BASE_URL}/api/empresas/20555555551", timeout=10.0)
        log(f"Status: {res.status_code}")
        assert res.status_code == 200, f"Fallo eliminando empresa: {res.text}"

        # Verificar que ya no aparece
        res = httpx.get(f"{BASE_URL}/api/empresas/20555555551", timeout=10.0)
        assert res.status_code == 404, "La empresa eliminada no debe encontrarse"

        log("\n=======================================================")
        log("  TODAS LAS PRUEBAS DE LA FASE 1 PASARON EXITOSAMENTE! ")
        log("=======================================================")

    finally:
        kill_process()
        log("Servidor detenido.")

if __name__ == "__main__":
    main()
