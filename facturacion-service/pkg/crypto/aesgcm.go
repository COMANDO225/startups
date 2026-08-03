package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

var ErrCipherTextCorto = errors.New("texto cifrado invalido")

// Cipher cifra datos que deben poder recuperarse: certificados digitales y
// claves SOL de cada tenant. Argon2 no sirve aca porque es de una sola via.
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher recibe una clave maestra de 32 bytes (AES-256).
func NewCipher(masterKey string) (*Cipher, error) {
	block, err := aes.NewCipher([]byte(masterKey))
	if err != nil {
		return nil, fmt.Errorf("clave maestra invalida: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creando GCM: %w", err)
	}

	return &Cipher{aead: aead}, nil
}

// Encrypt antepone el nonce al texto cifrado.
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generando nonce: %w", err)
	}

	return c.aead.Seal(nonce, nonce, plaintext, nil), nil
}

func (c *Cipher) Decrypt(ciphertext []byte) ([]byte, error) {
	n := c.aead.NonceSize()
	if len(ciphertext) < n {
		return nil, ErrCipherTextCorto
	}

	plaintext, err := c.aead.Open(nil, ciphertext[:n], ciphertext[n:], nil)
	if err != nil {
		return nil, fmt.Errorf("descifrando: %w", err)
	}

	return plaintext, nil
}
