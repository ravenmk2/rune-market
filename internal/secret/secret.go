// Package secret manages the master key at ./data/secret: 32 random bytes,
// hex encoded, file mode 0600, generated on first start.
package secret

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const fileName = "secret"

// LoadOrCreate returns the master key, generating it on first run.
func LoadOrCreate(dataDir string) ([]byte, error) {
	p := filepath.Join(dataDir, fileName)
	raw, err := os.ReadFile(p)
	if err == nil {
		key, decErr := hex.DecodeString(string(trimSpace(raw)))
		if decErr != nil || len(key) != 32 {
			return nil, fmt.Errorf("secret: %s is corrupt", p)
		}
		return key, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("secret: read: %w", err)
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("secret: rand: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("secret: create data dir: %w", err)
	}
	if err := os.WriteFile(p, []byte(hex.EncodeToString(key)), 0o600); err != nil {
		return nil, fmt.Errorf("secret: write: %w", err)
	}
	return key, nil
}

func trimSpace(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r' || b[len(b)-1] == ' ') {
		b = b[:len(b)-1]
	}
	return b
}
