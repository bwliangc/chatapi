package repository

import (
	"context"
	"database/sql"
	"sync"
	"sync/atomic"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	_ "modernc.org/sqlite"
)

func TestSettingCompareAndSwap(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE settings (id INTEGER PRIMARY KEY AUTOINCREMENT, key TEXT NOT NULL UNIQUE, value TEXT NOT NULL, updated_at DATETIME NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	repo := NewSettingRepository(client).(*settingRepository)
	ctx := context.Background()
	// Competing first writes initialize the key without overwriting the winner.
	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := repo.CompareAndSwapValue(ctx, "bank", "", "v1")
			if err != nil {
				t.Error(err)
			}
			if ok {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("expected one winner, got %d", wins.Load())
	}
	ok, err := repo.CompareAndSwapValue(ctx, "bank", "v1", "v2")
	if err != nil || !ok {
		t.Fatalf("valid swap failed %v", err)
	}
	ok, err = repo.CompareAndSwapValue(ctx, "bank", "v1", "old")
	if err != nil || ok {
		t.Fatalf("stale swap accepted %v", err)
	}
	value, err := repo.GetValue(ctx, "bank")
	if err != nil || value != "v2" {
		t.Fatalf("lost update %q %v", value, err)
	}
}
