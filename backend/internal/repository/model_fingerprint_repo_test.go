package repository

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
)

// Runs only against the dedicated Compose database. No records are deleted.
func TestModelFingerprintPostgres(t *testing.T) {
	dsn := os.Getenv("FINGERPRINT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("start deploy/docker-compose.fingerprint-test.yml database and set FINGERPRINT_TEST_DATABASE_URL")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	migration, err := os.ReadFile("../../migrations/252_model_fingerprint.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = db.ExecContext(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewModelFingerprintRepository(db)
	model := "fingerprint-test-" + time.Now().Format("20060102150405.000000000")
	record := &service.ModelFingerprintRecord{AccountID: 7, AccountName: "test-snapshot", Model: model, UpstreamModel: "mapped", Platform: "openai", Endpoint: "https://test.invalid/v1/responses", Status: "running", Samples: []service.FingerprintSample{}}
	if err = repo.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	if record.ID <= 0 || record.CreatedAt.IsZero() {
		t.Fatal("missing generated identity")
	}
	record.Status = "failed"
	record.Samples = append(record.Samples, service.FingerprintSample{Prompt: "probe", Error: "upstream HTTP 401"})
	if err = repo.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	items, total, err := repo.List(ctx, service.FingerprintFilter{Page: 1, PageSize: 20, AccountID: 7, Model: model})
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("list: %v %d %v", items, total, err)
	}
	if items[0].Status != "failed" || items[0].Samples[0].Error != "upstream HTTP 401" || items[0].UpstreamModel != "mapped" {
		t.Fatal("record roundtrip mismatch")
	}
	items, total, err = repo.List(ctx, service.FingerprintFilter{Page: 1, PageSize: 20, AccountID: 8, Model: model})
	if err != nil || total != 0 || len(items) != 0 {
		t.Fatal("account filter ignored")
	}
	if err = repo.SaveReference(ctx, []byte(`{"test":true}`), []byte("detector-test")); err != nil {
		t.Fatal(err)
	}
	bank, detector, err := repo.LoadReference(ctx)
	if err != nil || !strings.Contains(string(bank), "true") || string(detector) != "detector-test" {
		t.Fatal("reference roundtrip mismatch")
	}
}
