package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestModelFingerprintRestrictedAdminKeysDenied(t *testing.T) {
	for _, route := range []struct{ method, path string }{{"POST", "/api/v1/admin/model-fingerprint/verify"}, {"GET", "/api/v1/admin/model-fingerprint/records"}, {"GET", "/api/v1/admin/model-fingerprint/reference"}, {"POST", "/api/v1/admin/model-fingerprint/reference"}} {
		if adminaccess.RouteAllowed(adminaccess.Policy{}, route.method, route.path) {
			t.Fatalf("new fingerprint route implicitly granted: %s", route.path)
		}
		if !adminaccess.RouteAllowed(adminaccess.Policy{FullAccess: true}, route.method, route.path) {
			t.Fatalf("full admin should access %s", route.path)
		}
	}
}

func TestModelFingerprintRoutesBehindAdminAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := &handler.Handlers{Admin: &handler.AdminHandlers{ModelFingerprint: admin.NewModelFingerprintHandler(nil)}}
	auth := middleware.AdminAuthMiddleware(func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
		} else {
			c.AbortWithStatus(http.StatusForbidden)
		}
	})
	RegisterAdminRoutes(router.Group("/api/v1"), h, auth, middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }), middleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() }), nil, nil)
	for _, route := range []struct{ method, path string }{{"POST", "/verify"}, {"GET", "/records"}, {"GET", "/reference"}, {"POST", "/reference"}} {
		for _, authenticated := range []bool{false, true} {
			req := httptest.NewRequest(route.method, "/api/v1/admin/model-fingerprint"+route.path, nil)
			want := http.StatusUnauthorized
			if authenticated {
				req.Header.Set("Authorization", "Bearer non-admin")
				want = http.StatusForbidden
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != want {
				t.Fatalf("%s %s: got %d want %d", route.method, route.path, w.Code, want)
			}
		}
	}
}
