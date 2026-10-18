package crypto

import (
	"testing"
)

func TestCryptoEncryptDecrypt(t *testing.T) {
	masterKey := "mi_clave_maestra_super_segura_12345_mas_de_32_caracteres"
	service, err := NewCryptoService(masterKey)
	if err != nil {
		t.Fatalf("Error al instanciar CryptoService: %v", err)
	}

	secret := "MiClaveSOL2026*"
	encrypted, err := service.Encrypt(secret)
	if err != nil {
		t.Fatalf("Error al cifrar: %v", err)
	}

	if encrypted == secret {
		t.Fatalf("El texto cifrado no puede ser igual al texto original")
	}

	decrypted, err := service.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Error al descifrar: %v", err)
	}

	if decrypted != secret {
		t.Fatalf("Se esperaba '%s' pero se obtuvo '%s'", secret, decrypted)
	}
}

func TestCryptoShortKey(t *testing.T) {
	_, err := NewCryptoService("corta")
	if err == nil {
		t.Fatalf("Se esperaba error con clave menor a 32 caracteres")
	}
}

func TestCryptoInvalidKey(t *testing.T) {
	service1, _ := NewCryptoService("clave1_maestra_valida_32_caracteres_minimo_a")
	service2, _ := NewCryptoService("clave2_maestra_valida_32_caracteres_minimo_b")

	encrypted, err := service1.Encrypt("secreto")
	if err != nil {
		t.Fatalf("Error al cifrar: %v", err)
	}

	_, err = service2.Decrypt(encrypted)
	if err == nil {
		t.Fatalf("Se esperaba un error al descifrar con una clave diferente, pero no ocurrió")
	}
}

func TestCryptoEmpty(t *testing.T) {
	service, _ := NewCryptoService("clave_maestra_valida_32_caracteres_minimo_c")
	enc, err := service.Encrypt("")
	if err != nil || enc != "" {
		t.Fatalf("Texto vacío debe resultar en cadena vacía")
	}

	dec, err := service.Decrypt("")
	if err != nil || dec != "" {
		t.Fatalf("Cadena vacía debe descifrarse como vacía")
	}
}
