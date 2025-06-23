package grainfs

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

const (
	// Encryption constants
	NonceSize      = 12  // 96-bit nonce for GCM
	TagSize        = 16  // 128-bit authentication tag for GCM
	HMACSize       = 32  // 256-bit HMAC for filename authentication
	MaxFilenameLen = 200 // Maximum obfuscated filename length
)

// encryptData encrypts data using AES-256-GCM
// Returns: [nonce][encrypted_data][auth_tag]
func encryptData(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and authenticate
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Prepend nonce to ciphertext
	result := make([]byte, NonceSize+len(ciphertext))
	copy(result[:NonceSize], nonce)
	copy(result[NonceSize:], ciphertext)

	return result, nil
}

// decryptData decrypts data encrypted with encryptData
// Expects: [nonce][encrypted_data][auth_tag]
func decryptData(key, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < NonceSize+TagSize {
		return nil, fmt.Errorf("ciphertext too short: %d bytes", len(ciphertext))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce and encrypted data
	nonce := ciphertext[:NonceSize]
	encrypted := ciphertext[NonceSize:]

	// Decrypt and verify
	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// obfuscateFilename encrypts and encodes a filename for storage
// Uses deterministic encryption so the same filename always produces the same obfuscated result
func obfuscateFilename(filenameKey []byte, filename string) (string, error) {
	if filename == "" {
		return "", fmt.Errorf("filename cannot be empty")
	}

	// Use deterministic IV based on filename hash for consistent obfuscation
	h := sha256.New()
	h.Write(filenameKey)
	h.Write([]byte(filename))
	hash := h.Sum(nil)

	// Use first 16 bytes of hash as IV (deterministic)
	iv := hash[:aes.BlockSize]

	// Use AES-CTR for filename encryption
	block, err := aes.NewCipher(filenameKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	stream := cipher.NewCTR(block, iv)

	// Encrypt filename
	plaintext := []byte(filename)
	ciphertext := make([]byte, len(plaintext))
	stream.XORKeyStream(ciphertext, plaintext)

	// Create HMAC for authentication
	hmacHash := hmac.New(sha256.New, filenameKey)
	hmacHash.Write(iv)
	hmacHash.Write(ciphertext)
	mac := hmacHash.Sum(nil)

	// Combine: [iv][ciphertext][hmac]
	combined := make([]byte, len(iv)+len(ciphertext)+len(mac))
	copy(combined[:len(iv)], iv)
	copy(combined[len(iv):len(iv)+len(ciphertext)], ciphertext)
	copy(combined[len(iv)+len(ciphertext):], mac)

	// Base64url encode for filesystem safety
	encoded := base64.URLEncoding.EncodeToString(combined)

	// Ensure length limit
	if len(encoded) > MaxFilenameLen {
		return "", fmt.Errorf("obfuscated filename too long: %d > %d", len(encoded), MaxFilenameLen)
	}

	return encoded, nil
}

// deobfuscateFilename decodes and decrypts an obfuscated filename
func deobfuscateFilename(filenameKey []byte, obfuscated string) (string, error) {
	if obfuscated == "" {
		return "", fmt.Errorf("obfuscated filename cannot be empty")
	}

	// Base64url decode
	combined, err := base64.URLEncoding.DecodeString(obfuscated)
	if err != nil {
		return "", fmt.Errorf("failed to decode filename: %w", err)
	}

	if len(combined) < aes.BlockSize+HMACSize {
		return "", fmt.Errorf("obfuscated filename too short")
	}

	// Extract components: [iv][ciphertext][hmac]
	iv := combined[:aes.BlockSize]
	ciphertext := combined[aes.BlockSize : len(combined)-HMACSize]
	receivedMAC := combined[len(combined)-HMACSize:]

	// Verify HMAC
	h := hmac.New(sha256.New, filenameKey)
	h.Write(iv)
	h.Write(ciphertext)
	expectedMAC := h.Sum(nil)

	if !hmac.Equal(receivedMAC, expectedMAC) {
		return "", fmt.Errorf("filename authentication failed")
	}

	// Decrypt filename
	block, err := aes.NewCipher(filenameKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	stream := cipher.NewCTR(block, iv)
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)

	return string(plaintext), nil
}

// EncryptingWriter wraps an io.Writer to provide transparent encryption
type EncryptingWriter struct {
	writer io.Writer
	gcm    cipher.AEAD
	nonce  []byte
	buffer []byte
}

// NewEncryptingWriter creates a new encrypting writer
func NewEncryptingWriter(w io.Writer, key []byte) (*EncryptingWriter, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate and write nonce
	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	if _, err := w.Write(nonce); err != nil {
		return nil, fmt.Errorf("failed to write nonce: %w", err)
	}

	return &EncryptingWriter{
		writer: w,
		gcm:    gcm,
		nonce:  nonce,
		buffer: make([]byte, 0),
	}, nil
}

// Write encrypts and writes data
func (ew *EncryptingWriter) Write(p []byte) (n int, err error) {
	// Buffer the data for encryption
	ew.buffer = append(ew.buffer, p...)
	return len(p), nil
}

// Close finalizes encryption and writes the result
func (ew *EncryptingWriter) Close() error {
	if len(ew.buffer) == 0 {
		// Write empty encrypted data
		encrypted := ew.gcm.Seal(nil, ew.nonce, nil, nil)
		_, err := ew.writer.Write(encrypted)
		return err
	}

	// Encrypt all buffered data
	encrypted := ew.gcm.Seal(nil, ew.nonce, ew.buffer, nil)
	_, err := ew.writer.Write(encrypted)
	return err
}

// DecryptingReader wraps an io.Reader to provide transparent decryption
type DecryptingReader struct {
	reader      io.Reader
	gcm         cipher.AEAD
	nonce       []byte
	decrypted   []byte
	pos         int
	initialized bool
}

// NewDecryptingReader creates a new decrypting reader
func NewDecryptingReader(r io.Reader, key []byte) (*DecryptingReader, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &DecryptingReader{
		reader: r,
		gcm:    gcm,
	}, nil
}

// Read decrypts and returns data
func (dr *DecryptingReader) Read(p []byte) (n int, err error) {
	if !dr.initialized {
		if err := dr.initialize(); err != nil {
			return 0, err
		}
		dr.initialized = true
	}

	// Copy from decrypted buffer
	available := len(dr.decrypted) - dr.pos
	if available == 0 {
		return 0, io.EOF
	}

	n = len(p)
	if n > available {
		n = available
	}

	copy(p[:n], dr.decrypted[dr.pos:dr.pos+n])
	dr.pos += n

	return n, nil
}

// initialize reads and decrypts all data
func (dr *DecryptingReader) initialize() error {
	// Read nonce
	dr.nonce = make([]byte, NonceSize)
	if _, err := io.ReadFull(dr.reader, dr.nonce); err != nil {
		return fmt.Errorf("failed to read nonce: %w", err)
	}

	// Read all encrypted data
	encrypted, err := io.ReadAll(dr.reader)
	if err != nil {
		return fmt.Errorf("failed to read encrypted data: %w", err)
	}

	// Decrypt
	dr.decrypted, err = dr.gcm.Open(nil, dr.nonce, encrypted, nil)
	if err != nil {
		return fmt.Errorf("failed to decrypt data: %w", err)
	}

	return nil
}
