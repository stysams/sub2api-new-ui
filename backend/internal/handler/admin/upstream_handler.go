package admin

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type UpstreamHandler struct{ upstreamService service.UpstreamService }

func NewUpstreamHandler(upstreamService service.UpstreamService) *UpstreamHandler {
	return &UpstreamHandler{upstreamService: upstreamService}
}

func (h *UpstreamHandler) List(c *gin.Context) {
	if c.Query("page") != "" {
		page, pageSize := response.ParsePagination(c)
		search := c.Query("search")
		items, total, err := h.upstreamService.ListPaginated(c.Request.Context(), page, pageSize, search)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		response.Paginated(c, items, total, page, pageSize)
		return
	}
	items, err := h.upstreamService.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *UpstreamHandler) GetByID(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	item, err := h.upstreamService.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *UpstreamHandler) Create(c *gin.Context) {
	var req service.CreateUpstreamInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	executeAdminIdempotentJSON(c, "admin.upstreams.create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) { return h.upstreamService.Create(ctx, &req) })
}

func (h *UpstreamHandler) Update(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	var req service.UpdateUpstreamInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	executeAdminIdempotentJSON(c, "admin.upstreams.update", map[string]any{"id": id, "request": req}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) { return h.upstreamService.Update(ctx, id, &req) })
}

func (h *UpstreamHandler) Delete(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	executeAdminIdempotentJSON(c, "admin.upstreams.delete", map[string]any{"id": id}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) { return nil, h.upstreamService.Delete(ctx, id) })
}

func (h *UpstreamHandler) TestConnection(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	result, err := h.upstreamService.TestConnection(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UpstreamHandler) RefreshBalance(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	result, err := h.upstreamService.RefreshBalance(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UpstreamHandler) GetBalanceNotifySettings(c *gin.Context) {
	result, err := h.upstreamService.GetBalanceNotifySettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UpstreamHandler) UpdateBalanceNotifySettings(c *gin.Context) {
	var req service.UpstreamBalanceNotifySettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if err := h.upstreamService.UpdateBalanceNotifySettings(c.Request.Context(), &req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, &req)
}

func (h *UpstreamHandler) RefreshAllBalances(c *gin.Context) {
	result, err := h.upstreamService.RefreshAllBalances(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UpstreamHandler) FetchGroups(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	refresh := c.Query("refresh") == "1" || strings.EqualFold(c.Query("refresh"), "true")
	result, err := h.upstreamService.FetchGroups(c.Request.Context(), id, refresh)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UpstreamHandler) ListResources(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	result, err := h.upstreamService.ListResources(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UpstreamHandler) CreateKey(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	var req struct {
		Group string `json:"group" binding:"required"`
		Name  string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	executeAdminIdempotentJSON(c, "admin.upstreams.keys.create", map[string]any{"upstream_id": id, "request": req}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.upstreamService.CreateKey(ctx, id, req.Group, req.Name)
	})
}

func (h *UpstreamHandler) RefreshModels(c *gin.Context) {
	id, ok := parseUpstreamIDParam(c, "rid")
	if !ok {
		return
	}
	result, err := h.upstreamService.FetchModels(c.Request.Context(), id, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UpstreamHandler) ChatCompletion(c *gin.Context) {
	id, ok := parseUpstreamIDParam(c, "rid")
	if !ok {
		return
	}
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	body["stream"] = true
	resp, err := h.upstreamService.ChatCompletion(c.Request.Context(), id, body)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer resp.Body.Close()
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, resp.Body)
}

func (h *UpstreamHandler) SyncToAccount(c *gin.Context) {
	id, ok := parseUpstreamIDParam(c, "rid")
	if !ok {
		return
	}
	var req struct {
		GroupIDs []int64 `json:"group_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	executeAdminIdempotentJSON(c, "admin.upstreams.resources.sync", map[string]any{"resource_id": id, "request": req}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) { return h.upstreamService.SyncToAccount(ctx, id, req.GroupIDs) })
}

func parseUpstreamID(c *gin.Context) (int64, bool) { return parseUpstreamIDParam(c, "id") }
func parseUpstreamIDParam(c *gin.Context, param string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(param), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid upstream ID")
		return 0, false
	}
	return id, true
}
