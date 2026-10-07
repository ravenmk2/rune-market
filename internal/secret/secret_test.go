package secret

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadOrCreate(t *testing.T) {
	dir := t.TempDir()

	key, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("key length: %d", len(key))
	}

	// Windows ignores Unix file modes; 0600 is only enforceable elsewhere
	if runtime.GOOS != "windows" {
		st, err := os.Stat(filepath.Join(dir, "secret"))
		if err != nil {
			t.Fatal(err)
		}
		if perm := st.Mode().Perm(); perm&0o077 != 0 {
			t.Fatalf("secret permissions too open: %o", perm)
		}
	}

	// file content is the hex encoding of the key
	raw, err := os.ReadFile(filepath.Join(dir, "secret"))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := hex.DecodeString(string(raw))
	if err != nil || !bytes.Equal(decoded, key) {
		t.Fatalf("file content mismatch: %v", err)
	}

	// second call loads the same key
	again, err := LoadOrCreate(dir)
	if err != nil || !bytes.Equal(again, key) {
		t.Fatalf("reload: %v", err)
	}
}

func TestLoadCorrupt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "secret"), []byte("not-hex"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreate(dir); err == nil {
		t.Fatal("expected error for corrupt secret")
	}
}
