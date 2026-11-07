package securityaudit

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Embedding only this interface deliberately hides the repository's optional
// expiry cleanup capability. These regression tests append and retain records.
type promptContentPostgresAppendOnlyRepository struct{ PromptRecordRepository }

func TestPromptContentPolicyPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("PROMPT_CONTENT_TEST_DSN"))
	if dsn == "" {
		t.Skip("requires PostgreSQL via PROMPT_CONTENT_TEST_DSN (database prompt_content_policy_test)")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	var database string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT current_database()").Scan(&database))
	require.Equal(t, "prompt_content_policy_test", database, "only the dedicated content policy test database is allowed")
	preparePromptContentPostgresSchema(t, ctx, db)

	var initialRows int64
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM prompt_records").Scan(&initialRows))
	repository := NewPostgreSQLRepository(db)
	appendOnly := promptContentPostgresAppendOnlyRepository{PromptRecordRepository: repository}
	_, exposesCleanup := any(appendOnly).(promptRecordExpiryRepository)
	require.False(t, exposesCleanup, "the test worker must never start data cleanup")
	worker := newPromptRecordService(appendOnly, 8, 8, 1)
	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		require.NoError(t, worker.Shutdown(shutdownCtx))
	})
	manager := NewConfigManager(nil, &promptRecordingSettingRepository{}, nil, prefixEncryptor{}, testTotpKeyConfig())
	require.NoError(t, manager.Reload(ctx))
	policy := RecommendedPromptRecordingPolicy()
	require.Equal(t, policy, *manager.PromptRecordingSnapshot().RecordingPolicy, "recording uses the category policy without an opt-in setting")
	service := &PromptService{config: manager, records: worker}
	coordinator := NewCoordinator(nil, nil, service)
	session := "content-policy-" + uuid.NewString()
	userText := "请读取 main.go：" + session
	requestBody, err := json.Marshal(map[string]any{
		"model":        "content-policy-postgres",
		"instructions": "INSTRUCTIONS_POSTGRES_CANARY",
		"input": []any{
			map[string]any{"role": "system", "content": "SYSTEM_POSTGRES_CANARY"},
			map[string]any{"role": "user", "content": userText},
			map[string]any{"type": "function_call", "name": "read_file", "call_id": "read-1", "arguments": `{"path":"main.go"}`},
			map[string]any{"type": "function_call_output", "call_id": "read-1", "output": "读取结果：package main"},
			map[string]any{"role": "assistant", "content": "已读取源文件"},
			map[string]any{"type": "reasoning", "encrypted_content": "CIPHER_POSTGRES_CANARY", "summary": []any{
				map[string]any{"type": "summary_text", "text": "THINKING_POSTGRES_CANARY"},
			}},
		},
		"tools":    []any{map[string]any{"type": "function", "name": "read_file", "description": "TOOL_DEFINITION_POSTGRES_CANARY"}},
		"metadata": map[string]any{"opaque": "METADATA_POSTGRES_CANARY"},
	})
	require.NoError(t, err)
	req, responseReference := coordinator.PreparePromptRecording(Request{
		Endpoint: "/v1/responses", Protocol: "responses", Stage: "http", Body: requestBody,
		Headers: http.Header{"Session-Id": []string{session}},
	})
	require.NotNil(t, req.recordingCorrelation)
	coordinator.Check(ctx, req)
	response, recognized := ExtractRecordedResponse(responseReference, []byte(`{"output":[
		{"type":"reasoning","encrypted_content":"RESPONSE_CIPHER_POSTGRES_CANARY","summary":[{"type":"summary_text","text":"RESPONSE_THINKING_POSTGRES_CANARY"}]},
		{"role":"assistant","content":[{"type":"output_text","text":"源文件读取完成"}]}
	]}`), false)
	require.True(t, recognized)
	// Submit the response immediately: the worker must wait for the request ID
	// before it updates the correct row's response_document.
	coordinator.RecordResponse(ctx, responseReference, response)
	var recordID int64
	var lastReadError error
	require.Eventually(t, func() bool {
		var captured bool
		lastReadError = db.QueryRowContext(ctx,
			"SELECT id, response_captured_at IS NOT NULL FROM prompt_records WHERE session_id=$1", session,
		).Scan(&recordID, &captured)
		return lastReadError == nil && captured
	}, 10*time.Second, 20*time.Millisecond)
	require.NoError(t, lastReadError)
	stored, err := repository.GetPromptRecord(ctx, recordID)
	require.NoError(t, err)
	require.Empty(t, stored.PromptText, "canonical requests do not duplicate retained prompt text in storage")
	require.NotEmpty(t, stored.RecordingPolicy)
	var storedPolicy PromptRecordingPolicy
	require.NoError(t, json.Unmarshal([]byte(stored.RecordingPolicy), &storedPolicy))
	require.Equal(t, policy, storedPolicy)
	var requestDocument PromptContentDocument
	require.NoError(t, json.Unmarshal([]byte(stored.RequestBody), &requestDocument))
	require.Equal(t, 2, requestDocument.SchemaVersion)
	require.Equal(t, 2, PromptContentMessageCount(requestDocument))
	requestText := PromptContentText(requestDocument)
	for _, retained := range []string{userText, "main.go", "读取结果：package main", "已读取源文件"} {
		require.Contains(t, requestText, retained)
	}
	for _, filtered := range []string{"INSTRUCTIONS_POSTGRES_CANARY", "SYSTEM_POSTGRES_CANARY", "CIPHER_POSTGRES_CANARY", "THINKING_POSTGRES_CANARY", "TOOL_DEFINITION_POSTGRES_CANARY", "METADATA_POSTGRES_CANARY"} {
		require.NotContains(t, stored.RequestBody, filtered)
	}
	detail, err := service.GetPromptRecord(ctx, recordID)
	require.NoError(t, err)
	require.Equal(t, stored.RequestBody, detail.ContentDocument)
	require.Equal(t, requestText, detail.PromptText)
	require.Equal(t, utf8.RuneCountInString(requestText), detail.PromptLength)
	require.Equal(t, PromptContentMessageCount(requestDocument), detail.MessageCount)
	var responseDocument PromptContentDocument
	require.NoError(t, json.Unmarshal([]byte(detail.ResponseDocument), &responseDocument))
	require.Equal(t, 2, responseDocument.SchemaVersion)
	require.Equal(t, "源文件读取完成", PromptContentText(responseDocument))
	require.Equal(t, PromptContentText(responseDocument), detail.ResponseText)
	require.Equal(t, utf8.RuneCountInString(detail.ResponseText), detail.ResponseLength)
	require.NotNil(t, detail.ResponseCapturedAt)
	require.NotContains(t, detail.ResponseDocument, "RESPONSE_CIPHER_POSTGRES_CANARY")
	require.NotContains(t, detail.ResponseDocument, "RESPONSE_THINKING_POSTGRES_CANARY")
	require.Equal(t, response.Document, detail.ResponseDocument)

	t.Run("legacy text and raw JSON remain readable", func(t *testing.T) {
		for _, preserveText := range []bool{true, false} {
			legacySession := session + "-legacy-raw"
			if preserveText {
				legacySession = session + "-legacy-text"
			}
			legacyText := "已有文本：" + legacySession
			legacyBody, marshalErr := json.Marshal(map[string]any{"input": legacyText})
			require.NoError(t, marshalErr)
			hash := sha256.Sum256(legacyBody)
			legacy := &PromptRecord{
				SessionID: legacySession, Stage: "http", Protocol: "responses", Model: "legacy-content-postgres",
				RequestBody: string(legacyBody), PromptHash: hex.EncodeToString(hash[:]),
				PromptLength: utf8.RuneCountInString(legacyText), MessageCount: 1, CreatedAt: time.Now().UTC(),
			}
			if preserveText {
				legacy.PromptText = "保留旧记录文本：" + legacySession
			}
			legacyID, insertErr := repository.InsertPromptRecord(ctx, legacy)
			require.NoError(t, insertErr)
			legacyDetail, readErr := service.GetPromptRecord(ctx, legacyID)
			require.NoError(t, readErr)
			if preserveText {
				require.Equal(t, legacy.PromptText, legacyDetail.PromptText)
			} else {
				require.Equal(t, legacyText, legacyDetail.PromptText)
			}
			require.Equal(t, string(legacyBody), legacyDetail.RequestBody)
			require.Empty(t, legacyDetail.RecordingPolicy)
			require.Empty(t, legacyDetail.ContentDocument)
			require.Empty(t, legacyDetail.ResponseDocument)
			// Reading legacy content is a projection and must not rewrite its
			// original body or lazily materialized text into the database.
			persisted, readErr := repository.GetPromptRecord(ctx, legacyID)
			require.NoError(t, readErr)
			require.Equal(t, legacy.PromptText, persisted.PromptText)
			require.Equal(t, legacy.RequestBody, persisted.RequestBody)
		}
	})
	var retainedRows int64
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM prompt_records").Scan(&retainedRows))
	require.GreaterOrEqual(t, retainedRows, initialRows+3)
	require.Zero(t, worker.QueueStats().ExpiredDeleted)
	require.Zero(t, worker.QueueStats().PersistFailed)
	t.Logf("Retained new request/response row %d and two legacy rows for session %s", recordID, session)
}

func preparePromptContentPostgresSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	var exists bool
	require.NoError(t, db.QueryRowContext(ctx, "SELECT to_regclass('public.prompt_records') IS NOT NULL").Scan(&exists))
	for _, name := range []string{
		"239_prompt_records.sql", "240_prompt_record_responses.sql", "241_prompt_record_requests.sql",
		"242_prompt_record_search_extension.sql", "243_prompt_record_search_cleanup_indexes_notx.sql",
		"244_prompt_record_session_id.sql", "245_prompt_record_api_key_name_index_notx.sql",
	} {
		if exists && name == "239_prompt_records.sql" {
			continue
		}
		body, err := os.ReadFile(filepath.Join("../../migrations", name))
		require.NoError(t, err)
		for _, statement := range promptContentMigrationStatements(string(body)) {
			// The compatibility baseline keeps legacy columns/indexes and
			// never changes existing rows. The new migration below runs whole.
			upper := strings.ToUpper(statement)
			if name == "244_prompt_record_session_id.sql" && (strings.Contains(upper, "DROP ") || strings.HasPrefix(upper, "UPDATE ")) {
				continue
			}
			_, err = db.ExecContext(ctx, statement)
			require.NoError(t, err, name)
		}
	}
	body, err := os.ReadFile(filepath.Join("../../migrations", "251_prompt_record_content_policy.sql"))
	require.NoError(t, err)
	for range 2 {
		_, err = db.ExecContext(ctx, string(body))
		require.NoError(t, err, "content policy migration must be additive and repeatable")
	}
	var columns int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.columns
		WHERE table_schema='public' AND table_name='prompt_records'
		AND column_name IN ('recording_policy','response_document') AND data_type='text' AND is_nullable='NO'`).Scan(&columns))
	require.Equal(t, 2, columns)
}

// The baseline SQL has no semicolons inside string literals, but its line
// comments can contain them. Remove those comments before splitting statements.
func promptContentMigrationStatements(body string) []string {
	lines := strings.Split(body, "\n")
	for index, line := range lines {
		if start := strings.Index(line, "--"); start >= 0 {
			lines[index] = line[:start]
		}
	}
	var statements []string
	for _, statement := range strings.Split(strings.Join(lines, "\n"), ";") {
		if statement = strings.TrimSpace(statement); statement != "" {
			statements = append(statements, statement)
		}
	}
	return statements
}

func TestPromptContentMigrationStatementsIgnoreCommentSemicolons(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("../../migrations", "242_prompt_record_search_extension.sql"))
	require.NoError(t, err)
	require.Equal(t, []string{"CREATE EXTENSION IF NOT EXISTS pg_trgm"}, promptContentMigrationStatements(string(body)))
	require.Equal(t, []string{"SELECT 1", "SELECT 2"}, promptContentMigrationStatements("-- comment; fragment\nSELECT 1; -- another; fragment\nSELECT 2;"))
}
