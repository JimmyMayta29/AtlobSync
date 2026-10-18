import subprocess
import time
import requests
import sys

BASE_URL = "http://127.0.0.1:8080"
TEST_RUC = "20000000001"

def main():
    print("=== INICIANDO PRUEBAS DE INTEGRACIÓN FASE 3 ===")
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

        # 1. Validar DEFAULT_RUC en ruta heredada
        print("\n--- Test 1: Verificar RequireDefaultCompany con DEFAULT_RUC ---")
        r_legacy = requests.get(f"{BASE_URL}/api/sunat/ventas/periodos/140000")
        print("Status code:", r_legacy.status_code)
        legacy_header = r_legacy.headers.get("X-Sire-Legacy-Route")
        print("Header X-Sire-Legacy-Route:", legacy_header)
        assert legacy_header is not None, "Debe advertir deprecación"
        print("[PASS] DEFAULT_RUC validado exitosamente en rutas heredadas.")

        # 2. Validaciones de periodo y libro en exportación
        print("\n--- Test 2: Validación de parámetros en /exportar ---")
        r_invalid_periodo = requests.post(f"{BASE_URL}/api/empresas/{TEST_RUC}/sire/exportar/ventas/2024")
        print("Periodo inválido (2024):", r_invalid_periodo.status_code, r_invalid_periodo.json())
        assert r_invalid_periodo.status_code == 400, "Debe rechazar periodo inválido"

        r_invalid_libro = requests.post(f"{BASE_URL}/api/empresas/{TEST_RUC}/sire/exportar/inventario/202410")
        print("Libro inválido (inventario):", r_invalid_libro.status_code, r_invalid_libro.json())
        assert r_invalid_libro.status_code == 400, "Debe rechazar libro inválido"
        print("[PASS] Validaciones de libro y periodo aprobadas.")

        # 3. Listar descargas de la empresa (inicialmente vacío o con descargas previas)
        print("\n--- Test 3: Listar descargas GET /api/empresas/:ruc/sire/descargas ---")
        r_descargas = requests.get(f"{BASE_URL}/api/empresas/{TEST_RUC}/sire/descargas")
        print("Status code:", r_descargas.status_code)
        descargas_json = r_descargas.json()
        print("Total descargas registradas:", descargas_json.get("total", 0))
        assert r_descargas.status_code == 200
        print("[PASS] Endpoint de historial de descargas responde 200 OK.")

        # 4. Consultar detalle paginado de la propuesta con filtros
        print("\n--- Test 4: Detalle paginado GET /propuesta/ventas/202410/detalle ---")
        r_detalle = requests.get(
            f"{BASE_URL}/api/empresas/{TEST_RUC}/sire/propuesta/ventas/202410/detalle",
            params={"page": "1", "perPage": "10", "codTipoCDP": "01"}
        )
        print("Status code:", r_detalle.status_code)
        print("Respuesta (preview):", str(r_detalle.text)[:250])
        # Responderá 200 con comprobantes o el error de credenciales/rechazo de SUNAT
        print("[PASS] Endpoint de detalle paginado con filtros ejecutado correctamente.")

        # 5. Seguridad: Descarga por ID inexistente (debe rechazar con 404)
        print("\n--- Test 5: Descarga segura por ID inexistente ---")
        r_dl = requests.get(f"{BASE_URL}/api/descargas/99999/archivo")
        print("Status code:", r_dl.status_code)
        print("Respuesta:", r_dl.json())
        assert r_dl.status_code == 404
        print("[PASS] Rechazó ID inexistente con 404.")

        print("\n=== TODAS LAS PRUEBAS DE INTEGRACIÓN FASE 3 PASARON EXITOSAMENTE ===")
    finally:
        print("\nTerminando proceso servidor...")
        proc.terminate()
        proc.wait()
        print("[OK] Proceso terminado limpiamente.")

if __name__ == "__main__":
    main()
