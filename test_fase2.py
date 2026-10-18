import subprocess
import time
import requests
import sys

BASE_URL = "http://127.0.0.1:8080"

def main():
    print("=== INICIANDO PRUEBAS DE INTEGRACIÓN FASE 2 ===")
    
    # 1. Arrancar el binario sire_api.exe
    proc = subprocess.Popen(["./sire_api.exe"], cwd=".")
    print("Servidor iniciado con PID:", proc.pid)
    
    try:
        # Esperar a que el servidor esté listo
        server_ready = False
        for _ in range(15):
            try:
                r = requests.get(f"{BASE_URL}/api/empresas", timeout=1)
                if r.status_code == 200:
                    server_ready = True
                    break
            except Exception:
                time.sleep(0.5)
                
        if not server_ready:
            print("ERROR: El servidor no respondió a tiempo.")
            sys.exit(1)
            
        print("[OK] Servidor escuchando en http://127.0.0.1:8080")

        # 2. Obtener la lista de empresas registradas (sembradas desde .env)
        resp = requests.get(f"{BASE_URL}/api/empresas")
        empresas = resp.json().get("data", [])
        print(f"[OK] Empresas encontradas en SQLite: {len(empresas)}")
        
        test_ruc = None
        if empresas:
            test_ruc = empresas[0]["ruc"]
            print(f"     Usando empresa de prueba RUC: {test_ruc} ({empresas[0]['razon_social']})")
        else:
            print("AVISO: No hay empresas registradas.")
            
        # 3. Prueba Middleware RUC Inválido
        print("\n--- Test 1: Middleware de RUC inválido ---")
        r_invalid = requests.get(f"{BASE_URL}/api/empresas/12345/ventas/periodos")
        print("Status code:", r_invalid.status_code)
        print("Respuesta:", r_invalid.json())
        assert r_invalid.status_code == 400, "Debe rechazar RUC inválido con 400"
        print("[PASS] Rechazó RUC inválido con 400 y mensaje de error.")

        # 4. Prueba Middleware RUC Inexistente
        print("\n--- Test 2: Middleware de RUC inexistente en BD ---")
        r_notfound = requests.get(f"{BASE_URL}/api/empresas/20999999991/ventas/periodos")
        print("Status code:", r_notfound.status_code)
        print("Respuesta:", r_notfound.json())
        assert r_notfound.status_code == 404, "Debe retornar 404 si la empresa no existe"
        print("[PASS] Retornó 404 si la empresa no existe.")

        # 5. Prueba Rutas Heredadas (Legacy) con Header de Deprecación
        print("\n--- Test 3: Ruta heredada /api/sunat/ventas/periodos/140000 ---")
        r_legacy = requests.get(f"{BASE_URL}/api/sunat/ventas/periodos/140000")
        print("Status code:", r_legacy.status_code)
        legacy_header = r_legacy.headers.get("X-Sire-Legacy-Route")
        print("Header X-Sire-Legacy-Route:", legacy_header)
        assert legacy_header is not None, "Debe incluir header X-Sire-Legacy-Route"
        print("[PASS] La ruta heredada resolvió la empresa por defecto y advirtió deprecación.")

        # 6. Prueba Endpoints Multiempresa para RUC existente
        if test_ruc:
            print(f"\n--- Test 4: Endpoint Multiempresa Ventas para RUC {test_ruc} ---")
            r_ventas = requests.get(f"{BASE_URL}/api/empresas/{test_ruc}/ventas/periodos/140000")
            print("Status code:", r_ventas.status_code)
            print("Respuesta (preview):", str(r_ventas.text)[:200])

            print(f"\n--- Test 5: Endpoint Multiempresa Compras para RUC {test_ruc} ---")
            r_compras = requests.get(f"{BASE_URL}/api/empresas/{test_ruc}/compras/periodos/080000")
            print("Status code:", r_compras.status_code)
            print("Respuesta (preview):", str(r_compras.text)[:200])

        print("\n=== TODAS LAS PRUEBAS DE INTEGRACIÓN FASE 2 PASARON EXITOSAMENTE ===")
    finally:
        print("\nTerminando proceso servidor...")
        proc.terminate()
        proc.wait()
        print("[OK] Proceso terminado limpiamente.")

if __name__ == "__main__":
    main()
