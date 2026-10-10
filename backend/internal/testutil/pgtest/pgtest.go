// Package pgtest は DB を使うテスト用のヘルパー。テストごとに専用スキーマを作って
// マイグレーションを適用するため、パッケージ・テストを並行実行しても互いに干渉しない。
package pgtest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool は TEST_DATABASE_URL（DB 名が *_test）に専用スキーマを作り、
// db/migrations の *.up.sql を適用した pool を返す。未設定ならテストをスキップする。
func NewPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	if !strings.HasSuffix(strings.SplitN(url, "?", 2)[0], "_test") {
		t.Fatal("refusing to run against a database whose name does not end with _test")
	}
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	schema := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})

	for _, f := range migrationFiles(t) {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(b)); err != nil {
			t.Fatal(fmt.Errorf("%s: %w", f, err))
		}
	}
	return pool
}

func migrationFiles(t *testing.T) []string {
	t.Helper()
	_, here, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(here), "..", "..", "..", "db", "migrations")
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no migrations found in %s: %v", dir, err)
	}
	sort.Strings(files)
	return files
}
