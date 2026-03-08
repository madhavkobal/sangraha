package storage

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"
)

func TestInitSSERejectsWrongKeySize(t *testing.T) {
	if err := InitSSE([]byte("short")); err == nil {
		t.Error("expected error for short key")
	}
}

func TestInitSSEAccepts32Bytes(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	if err := InitSSE(key); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGenerateObjectKey(t *testing.T) {
	k1, err := GenerateObjectKey()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(k1) != sseKeySize {
		t.Errorf("key len = %d; want %d", len(k1), sseKeySize)
	}
	k2, err := GenerateObjectKey()
	if err != nil {
		t.Fatalf("generate second: %v", err)
	}
	if bytes.Equal(k1, k2) {
		t.Error("two generated keys are equal; RNG likely broken")
	}
}

func TestEncryptDecryptKeyRoundTrip(t *testing.T) {
	master := make([]byte, 32)
	if _, err := rand.Read(master); err != nil {
		t.Fatal(err)
	}
	if err := InitSSE(master); err != nil {
		t.Fatal(err)
	}

	objectKey, err := GenerateObjectKey()
	if err != nil {
		t.Fatal(err)
	}

	wrapped, err := EncryptKey(objectKey)
	if err != nil {
		t.Fatalf("EncryptKey: %v", err)
	}
	if bytes.Equal(wrapped, objectKey) {
		t.Error("wrapped key should differ from plaintext key")
	}

	unwrapped, err := DecryptKey(wrapped)
	if err != nil {
		t.Fatalf("DecryptKey: %v", err)
	}
	if !bytes.Equal(unwrapped, objectKey) {
		t.Error("round-trip: decrypted key does not match original")
	}
}

func TestEncryptingWriterDecryptingReaderRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		plaintext []byte
	}{
		{"empty", []byte{}},
		{"small", []byte("hello, world!")},
		{"1KB", makeBytes(1024)},
		{"64KB", makeBytes(65536)},
	}

	key := make([]byte, sseKeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Encrypt.
			var encBuf bytes.Buffer
			ew, err := NewEncryptingWriter(&encBuf, key)
			if err != nil {
				t.Fatalf("NewEncryptingWriter: %v", err)
			}
			if _, werr := ew.Write(tc.plaintext); werr != nil {
				t.Fatalf("Write: %v", werr)
			}
			if cerr := ew.Close(); cerr != nil {
				t.Fatalf("Close: %v", cerr)
			}

			// Verify ciphertext differs from plaintext (except empty case).
			if len(tc.plaintext) > 0 && bytes.Equal(encBuf.Bytes(), tc.plaintext) {
				t.Error("ciphertext equals plaintext; encryption not applied")
			}

			// Decrypt.
			dr, err := NewDecryptingReader(&encBuf, key)
			if err != nil {
				t.Fatalf("NewDecryptingReader: %v", err)
			}
			got, err := io.ReadAll(dr)
			if err != nil {
				t.Fatalf("ReadAll: %v", err)
			}

			if !bytes.Equal(got, tc.plaintext) {
				t.Errorf("decrypted content mismatch: got %q want %q", got, tc.plaintext)
			}
		})
	}
}

func TestDecryptingReaderWrongKeyFails(t *testing.T) {
	key1 := make([]byte, sseKeySize)
	key2 := make([]byte, sseKeySize)
	if _, err := rand.Read(key1); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(key2); err != nil {
		t.Fatal(err)
	}

	var encBuf bytes.Buffer
	ew, err := NewEncryptingWriter(&encBuf, key1)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = ew.Write([]byte("secret"))
	if err := ew.Close(); err != nil {
		t.Fatal(err)
	}

	// Decrypting with wrong key must fail.
	if _, err := NewDecryptingReader(&encBuf, key2); err == nil {
		t.Error("expected decryption to fail with wrong key")
	}
}

func makeBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i % 256)
	}
	return b
}
