package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ModelFingerprintHandler struct {
	svc *service.ModelFingerprintService
}

func NewModelFingerprintHandler(svc *service.ModelFingerprintService) *ModelFingerprintHandler {
	return &ModelFingerprintHandler{svc: svc}
}
func (h *ModelFingerprintHandler) Verify(c *gin.Context) {
	var req struct {
		AccountID int64  `json:"account_id" binding:"required,gt=0"`
		Model     string `json:"model" binding:"required,max=200"`
		API       string `json:"api"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := h.svc.Verify(c.Request.Context(), req.AccountID, req.Model, req.API)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}
func (h *ModelFingerprintHandler) List(c *gin.Context) {
	page, size := response.ParsePagination(c)
	if size > 100 {
		size = 100
	}
	id, err := strconv.ParseInt(c.DefaultQuery("account_id", "0"), 10, 64)
	if err != nil || id < 0 {
		response.BadRequest(c, "invalid account_id")
		return
	}
	items, total, err := h.svc.List(c.Request.Context(), service.FingerprintFilter{Page: page, PageSize: size, AccountID: id, Model: c.Query("model")})
	if err != nil {
		response.InternalError(c, "failed to load fingerprint records")
		return
	}
	response.Paginated(c, items, total, page, size)
}
func (h *ModelFingerprintHandler) Reference(c *gin.Context) {
	snapshot, err := h.svc.Reference(c.Request.Context())
	if err != nil {
		response.Success(c, gin.H{"available": false})
		return
	}
	response.Success(c, gin.H{"available": true, "built_at": snapshot.Bank.BuiltAt, "sha256": snapshot.SHA256, "models": snapshot.Bank.Models})
}
func (h *ModelFingerprintHandler) RefreshReference(c *gin.Context) {
	snapshot, err := h.svc.RefreshReference(c.Request.Context())
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"available": true, "built_at": snapshot.Bank.BuiltAt, "sha256": snapshot.SHA256, "models": snapshot.Bank.Models})
}
