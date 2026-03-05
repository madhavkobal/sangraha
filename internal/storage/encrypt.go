package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// sseKeySize is the AES-256 key size in bytes.
const sseKeySize = 32

// masterKey is the server-side master key used to envelope-encrypt per-object keys.
// In a production deployment this would be loaded from a KMS or config; for now
// it is derived at process start and kept in memory.
var masterKey []byte

// InitSSE sets the master key used for AES-256-GCM envelope encryption.
// Must be called once at server startup with a 32-byte key.
func InitSSE(key []byte) error {
	if len(key) != sseKeySize {
		return fmt.Errorf("sse: master key must be %d bytes, got %d", sseKeySize, len(key))
	}
	masterKey = make([]byte, sseKeySize)
	copy(masterKey, key)
	return nil
}

// GenerateObjectKey creates a new random 32-byte per-object encryption key.
func GenerateObjectKey() ([]byte, error) {
	key := make([]byte, sseKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("sse: generate key: %w", err)
	}
	return key, nil
}

// EncryptKey wraps a per-object key with the master key using AES-256-GCM.
// Returns nonce+ciphertext.
func EncryptKey(objectKey []byte) ([]byte, error) {
	if masterKey == nil {
		return nil, fmt.Errorf("sse: master key not initialised")
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("sse: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("sse: gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("sse: nonce: %w", err)
	}
	ct := gcm.Seal(nonce, nonce, objectKey, nil)
	return ct, nil
}

// DecryptKey unwraps a per-object key that was encrypted with EncryptKey.
func DecryptKey(encryptedKey []byte) ([]byte, error) {
	if masterKey == nil {
		return nil, fmt.Errorf("sse: master key not initialised")
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("sse: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("sse: gcm: %w", err)
	}
	if len(encryptedKey) < gcm.NonceSize() {
		return nil, fmt.Errorf("sse: encrypted key too short")
	}
	nonce := encryptedKey[:gcm.NonceSize()]
	ct := encryptedKey[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("sse: decrypt: %w", err)
	}
	return plain, nil
}

// NewEncryptingWriter wraps w with AES-256-GCM streaming encryption.
// The nonce is prepended to the output; key must be 32 bytes.
func NewEncryptingWriter(w io.Writer, key []byte) (io.WriteCloser, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("sse: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("sse: gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("sse: nonce: %w", err)
	}
	if _, err = w.Write(nonce); err != nil {
		return nil, fmt.Errorf("sse: write nonce: %w", err)
	}
	return &encryptingWriter{gcm: gcm, nonce: nonce, dst: w}, nil
}

type encryptingWriter struct {
	gcm   cipher.AEAD
	nonce []byte
	dst   io.Writer
	buf   []byte
}

func (ew *encryptingWriter) Write(p []byte) (int, error) {
	ew.buf = append(ew.buf, p...)
	return len(p), nil
}

func (ew *encryptingWriter) Close() error {
	ct := ew.gcm.Seal(nil, ew.nonce, ew.buf, nil)
	_, err := ew.dst.Write(ct)
	return err
}

// NewDecryptingReader wraps r with AES-256-GCM streaming decryption.
// Reads and verifies the nonce from the stream header; key must be 32 bytes.
func NewDecryptingReader(r io.Reader, key []byte) (io.Reader, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("sse: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("sse: gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(r, nonce); err != nil {
		return nil, fmt.Errorf("sse: read nonce: %w", err)
	}
	ct, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("sse: read ciphertext: %w", err)
	}
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("sse: decrypt: %w", err)
	}
	return newBytesReader(plain), nil
}

// bytesReader wraps a byte slice as an io.Reader.
type bytesReader struct {
	data []byte
	pos  int
}

func newBytesReader(data []byte) *bytesReader { return &bytesReader{data: data} }

func (br *bytesReader) Read(p []byte) (int, error) {
	if br.pos >= len(br.data) {
		return 0, io.EOF
	}
	n := copy(p, br.data[br.pos:])
	br.pos += n
	return n, nil
}
