package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

// Sal fija para derivación de llave con Argon2id
var argonSalt = []byte("SUNAT_SIRE_SALT_ARGON2_2026_V1")

type CryptoService struct {
	key []byte
}

func NewCryptoService(masterKey string) (*CryptoService, error) {
	if len(masterKey) < 32 {
		return nil, errors.New("masterKey debe tener al menos 32 caracteres")
	}

	// Derivar clave de 32 bytes usando Argon2id (memoria: 64MB, iteraciones: 2, hilos: 2)
	derivedKey := argon2.IDKey([]byte(masterKey), argonSalt, 2, 64*1024, 2, 32)
	return &CryptoService{key: derivedKey}, nil
}

// Encrypt cifra un texto en claro usando AES-256-GCM y retorna base64(nonce + ciphertext + tag)
func (c *CryptoService) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("error al crear bloque AES: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("error al inicializar GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("error al generar nonce aleatorio: %w", err)
	}

	// Seal anexa el ciphertext y tag al nonce
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt descifra una cadena en base64 generada por Encrypt
func (c *CryptoService) Decrypt(ciphertextBase64 string) (string, error) {
	if ciphertextBase64 == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return "", fmt.Errorf("error al decodificar base64: %w", err)
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("error al crear bloque AES: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("error al inicializar GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("longitud de datos cifrados inválida")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintextBytes, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("error al descifrar datos (clave inválida o datos manipulados): %w", err)
	}

	return string(plaintextBytes), nil
}
