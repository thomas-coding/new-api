package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func newAuthTestRouter(sessionValues map[string]any) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := cookie.NewStore([]byte("test-secret"))
	router.Use(sessions.Sessions("session", store))
	router.Use(func(c *gin.Context) {
		session := sessions.Default(c)
		for key, value := range sessionValues {
			session.Set(key, value)
		}
		c.Next()
	})
	router.GET("/protected", UserAuth(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	return router
}

func TestUserAuthAllowsSessionWithoutUserHeader(t *testing.T) {
	router := newAuthTestRouter(map[string]any{
		"id":       7,
		"username": "member",
		"role":     common.RoleCommonUser,
		"status":   common.UserStatusEnabled,
		"group":    "default",
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusNoContent, rec.Code, rec.Body.String())
	}
}

func TestUserAuthAllowsEquivalentNumericSessionID(t *testing.T) {
	router := newAuthTestRouter(map[string]any{
		"id":       int64(9),
		"username": "member",
		"role":     int64(common.RoleCommonUser),
		"status":   int64(common.UserStatusEnabled),
		"group":    "default",
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("New-Api-User", "9")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusNoContent, rec.Code, rec.Body.String())
	}
}

func TestUserAuthRejectsMismatchedUserHeader(t *testing.T) {
	router := newAuthTestRouter(map[string]any{
		"id":       7,
		"username": "member",
		"role":     common.RoleCommonUser,
		"status":   common.UserStatusEnabled,
		"group":    "default",
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("New-Api-User", "8")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
}
