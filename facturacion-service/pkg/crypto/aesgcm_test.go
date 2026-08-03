package crypto

import (
	"bytes"
	"testing"
)

const testKey = "01234567890123456789012345678901"

func TestCipherRoundTrip(t *testing.T) {
	c, err := NewCipher(testKey)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}

	original := []byte("-----BEGIN PRIVATE KEY-----\nMIIEv...\n")

	encrypted, err := c.Encrypt(original)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Contains(encrypted, original) {
		t.Fatal("el texto plano aparece en el cifrado")
	}

	decrypted, err := c.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(decrypted, original) {
		t.Fatalf("round trip fallido: %q", decrypted)
	}
}

func TestCipherNonceUnico(t *testing.T) {
	c, _ := NewCipher(testKey)

	a, _ := c.Encrypt([]byte("mismo dato"))
	b, _ := c.Encrypt([]byte("mismo dato"))

	if bytes.Equal(a, b) {
		t.Fatal("dos cifrados del mismo dato son identicos: nonce reutilizado")
	}
}

func TestCipherRechazaAlterado(t *testing.T) {
	c, _ := NewCipher(testKey)

	encrypted, _ := c.Encrypt([]byte("dato sensible"))
	encrypted[len(encrypted)-1] ^= 0xFF

	if _, err := c.Decrypt(encrypted); err == nil {
		t.Fatal("acepto un texto cifrado alterado")
	}
}

func TestCipherClaveInvalida(t *testing.T) {
	if _, err := NewCipher("corta"); err == nil {
		t.Fatal("acepto una clave de longitud invalida")
	}
}
