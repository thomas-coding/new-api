package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTopUpControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.User{}, &model.Redemption{}, &model.Log{}); err != nil {
		t.Fatalf("failed to migrate test tables: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestTopUpAllowsRedeemWithSessionOnly(t *testing.T) {
	db := setupTopUpControllerTestDB(t)

	user := &model.User{
		Id:       9,
		Username: "member",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	redemption := &model.Redemption{
		Name:   "quota-code",
		Key:    "TEST-REDEEM-001",
		Status: common.RedemptionCodeStatusEnabled,
		Quota:  500000,
	}
	if err := db.Create(redemption).Error; err != nil {
		t.Fatalf("failed to create redemption: %v", err)
	}

	router := gin.New()
	store := cookie.NewStore([]byte("test-secret"))
	router.Use(sessions.Sessions("session", store))
	router.Use(func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", int64(user.Id))
		session.Set("username", user.Username)
		session.Set("role", int64(user.Role))
		session.Set("status", int64(user.Status))
		session.Set("group", user.Group)
		c.Next()
	})
	router.POST("/api/user/topup", middleware.UserAuth(), TopUp)

	body := bytes.NewBufferString(`{"key":"TEST-REDEEM-001"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/user/topup", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	response := decodeAPIResponse(t, rec)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s body=%s", response.Message, rec.Body.String())
	}

	var updatedUser model.User
	if err := db.First(&updatedUser, user.Id).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if updatedUser.Quota != redemption.Quota {
		t.Fatalf("expected user quota %d, got %d", redemption.Quota, updatedUser.Quota)
	}

	var updatedRedemption model.Redemption
	if err := db.First(&updatedRedemption, redemption.Id).Error; err != nil {
		t.Fatalf("failed to reload redemption: %v", err)
	}
	if updatedRedemption.Status != common.RedemptionCodeStatusUsed {
		t.Fatalf("expected redemption status %d, got %d", common.RedemptionCodeStatusUsed, updatedRedemption.Status)
	}
	if updatedRedemption.UsedUserId != user.Id {
		t.Fatalf("expected redemption used user %d, got %d", user.Id, updatedRedemption.UsedUserId)
	}
}
