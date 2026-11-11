package modelfingerprint

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestReferenceDownloadAndChecksum(t *testing.T) {
	header := []byte(`{"schema":"shared-detector-v2","bank_built_at":"v","model_ids":["a","b"],"response_counts":[1,1]}`)
	raw := make([]byte, (4+len(header)+3)/4*4)
	binary.LittleEndian.PutUint32(raw, uint32(len(header)))
	copy(raw[4:], header)
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer encoder.Close()
	compressed := encoder.EncodeAll(raw, nil)
	badHash := false
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "" || r.Header.Get("x-api-key") != "" {
			t.Error("reference request carried credentials")
		}
		switch r.URL.Path {
		case "/bundle.json":
			digest := hash
			if badHash {
				digest = "bad"
			}
			fmt.Fprintf(w, `{"schema":"lmfpd-data-v1","built_at":"v","models":[{"id":"a","response_count":1},{"id":"b","response_count":1}],"files":{"shared_detector.bin":{"sha256":"%s","parts":["fixture.zst"]}}}`, digest)
		case "/chunks/fixture.zst":
			_, _ = w.Write(compressed)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	original := Sources
	Sources = []string{server.URL + "/"}
	defer func() { Sources = original }()
	snapshot, err := Download(context.Background())
	if err != nil || snapshot.SHA256 != hash || calls != 2 {
		t.Fatalf("download: %+v %v, calls %d", snapshot, err, calls)
	}
	badHash = true
	if _, err = Download(context.Background()); err == nil {
		t.Fatal("corrupt reference accepted")
	}
}
func TestReferenceDownloaderRejectsRedirects(t *testing.T) {
	leaked := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { leaked = true }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer redirect.Close()
	original := Sources
	Sources = []string{redirect.URL + "/"}
	defer func() { Sources = original }()
	if _, err := Download(context.Background()); err == nil || leaked {
		t.Fatal("reference redirect followed")
	}
}
