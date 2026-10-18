package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// ErrUnknownKeyVersion is returned when a ciphertext references a key version
// that is not configured.
var ErrUnknownKeyVersion = errors.New("unknown encryption key version")

// Service encrypts message content with AES-256-GCM. Multiple key versions are
// supported so keys can be rotated without downtime.
type Service struct {
	keys    map[int][]byte
	current int
}

// New validates every key (32 bytes) and builds the cipher service.
func New(keys map[int][]byte, current int) (*Service, error) {
	if len(keys) == 0 {
		return nil, errors.New("crypto: no encryption keys configured")
	}
	if _, ok := keys[current]; !ok {
		return nil, fmt.Errorf("crypto: no key for current version %d", current)
	}
	cp := make(map[int][]byte, len(keys))
	for v, k := range keys {
		if len(k) != 32 {
			return nil, fmt.Errorf("crypto: key version %d must be 32 bytes, got %d", v, len(k))
		}
		cp[v] = k
	}
	return &Service{keys: cp, current: current}, nil
}

// CurrentVersion returns the key version used for new ciphertexts.
func (s *Service) CurrentVersion() int { return s.current }

// Encrypt returns base64 ciphertext, base64 nonce and the key version.
func (s *Service) Encrypt(plain string) (ct string, iv string, version int, err error) {
	key, ok := s.keys[s.current]
	if !ok {
		return "", "", 0, ErrUnknownKeyVersion
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", 0, fmt.Errorf("crypto: creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", 0, fmt.Errorf("crypto: creating gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", 0, fmt.Errorf("crypto: reading nonce: %w", err)
	}
	sealed := gcm.Seal(nil, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(sealed),
		base64.StdEncoding.EncodeToString(nonce),
		s.current, nil
}

// Decrypt opens a ciphertext produced by Encrypt.
func (s *Service) Decrypt(ct, iv string, version int) (string, error) {
	key, ok := s.keys[version]
	if !ok {
		return "", ErrUnknownKeyVersion
	}
	raw, err := base64.StdEncoding.DecodeString(ct)
	if err != nil {
		return "", fmt.Errorf("crypto: decoding ciphertext: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(iv)
	if err != nil {
		return "", fmt.Errorf("crypto: decoding nonce: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypto: creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: creating gcm: %w", err)
	}
	if len(nonce) != gcm.NonceSize() {
		return "", errors.New("crypto: invalid nonce size")
	}
	plain, err := gcm.Open(nil, nonce, raw, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: authenticating ciphertext: %w", err)
	}
	return string(plain), nil
}

// ReEncrypt decrypts with the old version and encrypts with the current one.
// Used by the key-rotation job.
func (s *Service) ReEncrypt(ct, iv string, oldVer int) (string, string, int, error) {
	plain, err := s.Decrypt(ct, iv, oldVer)
	if err != nil {
		return "", "", 0, err
	}
	return s.Encrypt(plain)
}
