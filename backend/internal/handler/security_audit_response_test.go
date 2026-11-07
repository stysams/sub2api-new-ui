package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type capturedPromptResponse struct {
	request  securityaudit.Request
	response securityaudit.PromptResponse
}

type promptResponseRecorderStub struct {
	captured         chan capturedPromptResponse
	disabled         bool
	responseDisabled bool
	panicOnResponse  bool
}

func (*promptResponseRecorderStub) RecordPrompt(context.Context, securityaudit.Request) {}
func (r *promptResponseRecorderStub) PromptRecordingEnabled() bool                      { return !r.disabled }
func (r *promptResponseRecorderStub) PromptResponseRecordingEnabled() bool {
	return !r.responseDisabled
}
func (r *promptResponseRecorderStub) GetPromptRecordingConfig() securityaudit.PromptRecordingConfig {
	return securityaudit.PromptRecordingConfig{
		Enabled:         !r.disabled,
		ResponseEnabled: !r.responseDisabled,
	}
}
func (r *promptResponseRecorderStub) RecordResponse(_ context.Context, request securityaudit.Request, response securityaudit.PromptResponse) {
	if r.panicOnResponse {
		panic("response recorder unavailable")
	}
	r.captured <- capturedPromptResponse{request: request, response: response}
}

func TestPromptResponseCaptureMiddlewareBypassesCaptureWhenResponseDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &promptResponseRecorderStub{captured: make(chan capturedPromptResponse, 1), responseDisabled: true}
	coordinator := securityaudit.NewCoordinator(nil, nil, recorder)
	handlers := &Handlers{Gateway: &GatewayHandler{securityAuditCoordinator: coordinator}}
	router := gin.New()
	router.Use(handlers.PromptResponseCaptureMiddleware())
	router.POST("/v1/test", func(c *gin.Context) {
		_, reference := coordinator.PreparePromptRecording(securityaudit.Request{RequestID: "response-disabled", Endpoint: "/v1/chat/completions", Protocol: "openai_chat_completions"})
		c.Set(securityAuditRequestContextKey, reference)
		c.JSON(http.StatusOK, gin.H{"choices": []any{gin.H{"message": gin.H{"content": "must not be extracted"}}}})
		writer := c.Writer.(*securityAuditResponseWriter)
		require.Zero(t, writer.body.Len(), "disabled response recording must not buffer the response")
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/test", nil))
	require.Equal(t, http.StatusOK, response.Code)
	select {
	case <-recorder.captured:
		t.Fatal("disabled response recording must bypass response capture")
	default:
	}
}

func TestPromptResponseCaptureMiddlewareBypassesCaptureWhenRecordingDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &promptResponseRecorderStub{captured: make(chan capturedPromptResponse, 1), disabled: true}
	coordinator := securityaudit.NewCoordinator(nil, nil, recorder)
	handlers := &Handlers{Gateway: &GatewayHandler{securityAuditCoordinator: coordinator}}
	router := gin.New()
	router.Use(handlers.PromptResponseCaptureMiddleware())
	router.POST("/v1/test", func(c *gin.Context) {
		_, reference := coordinator.PreparePromptRecording(securityaudit.Request{RequestID: "req-disabled", Endpoint: "/v1/chat/completions", Protocol: "openai_chat_completions"})
		c.Set(securityAuditRequestContextKey, reference)
		c.JSON(http.StatusOK, gin.H{"choices": []any{gin.H{"message": gin.H{"content": "must not be captured"}}}})
		writer := c.Writer.(*securityAuditResponseWriter)
		require.Zero(t, writer.body.Len(), "disabled prompt recording must not buffer the response")
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/test", nil))
	require.Equal(t, http.StatusOK, response.Code)
	select {
	case <-recorder.captured:
		t.Fatal("disabled prompt recording must bypass response capture")
	default:
	}
}

func TestPromptResponseCaptureMiddlewareRetainsEnabledSnapshotAfterRecordingDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &promptResponseRecorderStub{captured: make(chan capturedPromptResponse, 1)}
	coordinator := securityaudit.NewCoordinator(nil, nil, recorder)
	handlers := &Handlers{Gateway: &GatewayHandler{securityAuditCoordinator: coordinator}}
	router := gin.New()
	router.Use(handlers.PromptResponseCaptureMiddleware())
	router.POST("/v1/test", func(c *gin.Context) {
		_, reference := coordinator.PreparePromptRecording(securityaudit.Request{RequestID: "enabled-snapshot", Endpoint: "/v1/chat/completions", Protocol: "openai_chat_completions"})
		c.Set(securityAuditRequestContextKey, reference)
		recorder.disabled = true
		require.False(t, coordinator.PromptResponseRecordingEnabled())
		require.True(t, securityaudit.ShouldRecordPromptResponse(reference))
		c.JSON(http.StatusOK, gin.H{"choices": []any{gin.H{"message": gin.H{"content": "snapshot response"}}}})
		writer := c.Writer.(*securityAuditResponseWriter)
		require.Positive(t, writer.body.Len(), "the enabled request snapshot must retain response buffering")
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/test", nil))
	require.Equal(t, http.StatusOK, response.Code)
	select {
	case captured := <-recorder.captured:
		require.Equal(t, "enabled-snapshot", captured.request.RequestID)
		require.Equal(t, "snapshot response", captured.response.Text)
	default:
		t.Fatal("disabling recording after preparation must not discard the response")
	}
}

func TestPromptResponseCaptureMiddlewareCapturesWhenEnabledBeforePreparation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &promptResponseRecorderStub{captured: make(chan capturedPromptResponse, 1), disabled: true}
	coordinator := securityaudit.NewCoordinator(nil, nil, recorder)
	handlers := &Handlers{Gateway: &GatewayHandler{securityAuditCoordinator: coordinator}}
	router := gin.New()
	router.Use(handlers.PromptResponseCaptureMiddleware())
	router.POST("/v1/test", func(c *gin.Context) {
		recorder.disabled = false
		_, reference := coordinator.PreparePromptRecording(securityaudit.Request{RequestID: "enabled-before-prepare", Endpoint: "/v1/chat/completions", Protocol: "openai_chat_completions"})
		c.Set(securityAuditRequestContextKey, reference)
		c.JSON(http.StatusOK, gin.H{"choices": []any{gin.H{"message": gin.H{"content": "newly enabled response"}}}})
		writer := c.Writer.(*securityAuditResponseWriter)
		require.Positive(t, writer.body.Len(), "recording enabled during preparation must buffer the response")
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/test", nil))
	require.Equal(t, http.StatusOK, response.Code)
	select {
	case captured := <-recorder.captured:
		require.Equal(t, "enabled-before-prepare", captured.request.RequestID)
		require.Equal(t, "newly enabled response", captured.response.Text)
	default:
		t.Fatal("enabling recording before preparation must capture the response")
	}
}

func TestPromptResponseCaptureMiddlewareExtractsSuccessfulResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &promptResponseRecorderStub{captured: make(chan capturedPromptResponse, 1)}
	coordinator := securityaudit.NewCoordinator(nil, nil, recorder)
	handlers := &Handlers{Gateway: &GatewayHandler{securityAuditCoordinator: coordinator}}
	router := gin.New()
	router.Use(handlers.PromptResponseCaptureMiddleware())
	router.POST("/v1/test", func(c *gin.Context) {
		c.Set(securityAuditRequestContextKey, securityaudit.Request{RequestID: "req-1", APIKeyID: 9, Protocol: "openai_chat_completions", Stage: "http"})
		c.JSON(http.StatusOK, gin.H{"choices": []any{gin.H{"message": gin.H{"content": "captured text"}}}})
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/test", nil))
	require.Equal(t, http.StatusOK, response.Code)
	captured := <-recorder.captured
	require.Equal(t, "req-1", captured.request.RequestID)
	require.Equal(t, "captured text", captured.response.Text)
	require.Equal(t, 13, captured.response.Length)
}

func TestPromptResponseCaptureMiddlewarePreservesResponseWhenRecorderPanics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &promptResponseRecorderStub{captured: make(chan capturedPromptResponse, 1), panicOnResponse: true}
	coordinator := securityaudit.NewCoordinator(nil, nil, recorder)
	handlers := &Handlers{Gateway: &GatewayHandler{securityAuditCoordinator: coordinator}}
	router := gin.New()
	router.Use(handlers.PromptResponseCaptureMiddleware())
	router.POST("/v1/test", func(c *gin.Context) {
		c.Set(securityAuditRequestContextKey, securityaudit.Request{RequestID: "isolated-response"})
		c.JSON(http.StatusOK, gin.H{"choices": []any{gin.H{"message": gin.H{"content": "upstream response"}}}})
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/test", nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"choices":[{"message":{"content":"upstream response"}}]}`, response.Body.String())
}

func TestPromptResponseCaptureMiddlewareSkipsFailedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &promptResponseRecorderStub{captured: make(chan capturedPromptResponse, 1)}
	coordinator := securityaudit.NewCoordinator(nil, nil, recorder)
	handlers := &Handlers{Gateway: &GatewayHandler{securityAuditCoordinator: coordinator}}
	router := gin.New()
	router.Use(handlers.PromptResponseCaptureMiddleware())
	router.POST("/v1/test", func(c *gin.Context) {
		c.Set(securityAuditRequestContextKey, securityaudit.Request{RequestID: "req-failed", Protocol: "openai_chat_completions"})
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream failed"})
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/test", nil))
	select {
	case <-recorder.captured:
		t.Fatal("failed response must not be retained as generated text")
	default:
	}
}

func TestPromptResponseCaptureMiddlewareRetainsLargeResponsePrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &promptResponseRecorderStub{captured: make(chan capturedPromptResponse, 1)}
	coordinator := securityaudit.NewCoordinator(nil, nil, recorder)
	handlers := &Handlers{Gateway: &GatewayHandler{securityAuditCoordinator: coordinator}}
	router := gin.New()
	router.Use(handlers.PromptResponseCaptureMiddleware())
	text := strings.Repeat("a", 3*1024*1024)
	router.POST("/v1/test", func(c *gin.Context) {
		c.Set(securityAuditRequestContextKey, securityaudit.Request{RequestID: "large-response", Stage: "http"})
		c.JSON(http.StatusOK, gin.H{"choices": []any{gin.H{"message": gin.H{"content": text}}}})
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/test", nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), text, "client response must remain complete")
	select {
	case captured := <-recorder.captured:
		require.True(t, captured.response.Truncated)
		require.NotEmpty(t, captured.response.Text)
		require.LessOrEqual(t, len(captured.response.Text), securityaudit.PromptResponseTextLimit)
		require.NotEmpty(t, captured.response.Document)
		require.LessOrEqual(t, len(captured.response.Document), securityaudit.PromptResponseTextLimit)
	default:
		t.Fatal("large successful JSON response was lost")
	}
}
