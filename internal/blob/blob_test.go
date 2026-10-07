package blob

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	"github.com/ravenmk2/rune-market/internal/config"
	"github.com/ravenmk2/rune-market/internal/store"
)

func testSetup(t *testing.T) (*Storage, *store.Stores) {
	t.Helper()
	dataDir := t.TempDir()
	cfg := &config.Config{Database: config.Database{
		Driver: store.DialectSQLite, DataDir: t.TempDir(),
	}}
	db, err := store.Open(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db, store.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	return New(dataDir), store.NewStores(db, store.DialectSQLite)
}

func TestPutDedupRelease(t *testing.T) {
	storage, stores := testSetup(t)
	ctx := context.Background()
	content := []byte("fake zip content")
	expectSum := sha256.Sum256(content)

	// first put writes the file and the row
	sum, size, err := storage.Put(ctx, stores.DB, KindArchive, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if sum != hex.EncodeToString(expectSum[:]) || size != int64(len(content)) {
		t.Fatalf("sum=%q size=%d", sum, size)
	}
	p, _ := storage.Path(KindArchive, sum)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("file missing: %v", err)
	}

	// second put of identical content: ref_count+1, same sha
	sum2, _, err := storage.Put(ctx, stores.DB, KindArchive, bytes.NewReader(content))
	if err != nil || sum2 != sum {
		t.Fatalf("dedup put: %v %q", err, sum2)
	}
	var refs int
	if err := stores.DB.QueryRowContext(ctx,
		`SELECT ref_count FROM blob WHERE sha256 = ?`, sum).Scan(&refs); err != nil {
		t.Fatal(err)
	}
	if refs != 2 {
		t.Fatalf("ref_count=%d want 2", refs)
	}

	// release twice: row and file are gone
	if err := storage.Release(ctx, stores.DB, sum); err != nil {
		t.Fatalf("release 1: %v", err)
	}
	if err := storage.Release(ctx, stores.DB, sum); err != nil {
		t.Fatalf("release 2: %v", err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("file should be deleted: %v", err)
	}
	var n int
	_ = stores.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM blob WHERE sha256 = ?`, sum).Scan(&n)
	if n != 0 {
		t.Fatalf("row should be deleted, count=%d", n)
	}

	// releasing a missing blob is ErrNotFound
	if err := storage.Release(ctx, stores.DB, sum); err != store.ErrNotFound {
		t.Fatalf("release missing: %v", err)
	}
}

func TestPutWithinTx(t *testing.T) {
	storage, stores := testSetup(t)
	ctx := context.Background()

	tx, err := stores.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	sum, _, err := storage.Put(ctx, tx, KindArchive, bytes.NewReader([]byte("tx content")))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	f, err := storage.Open(KindArchive, sum)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = f.Close()
}
