package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSubscriptionCacheTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := DB
	oldLogDB := LOG_DB
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldRedisEnabled := common.RedisEnabled
	oldBatchUpdateEnabled := common.BatchUpdateEnabled
	oldInvalidator := subscriptionUserCacheInvalidator

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	DB = db
	LOG_DB = db
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	initCol()
	subscriptionUserCacheInvalidator = invalidateUserCache

	require.NoError(t, db.AutoMigrate(&User{}, &SubscriptionPlan{}, &UserSubscription{}))

	t.Cleanup(func() {
		subscriptionUserCacheInvalidator = oldInvalidator
		common.BatchUpdateEnabled = oldBatchUpdateEnabled
		common.RedisEnabled = oldRedisEnabled
		common.UsingPostgreSQL = oldUsingPostgreSQL
		common.UsingMySQL = oldUsingMySQL
		common.UsingSQLite = oldUsingSQLite
		DB = oldDB
		LOG_DB = oldLogDB

		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func trackSubscriptionCacheInvalidations(t *testing.T) *[]int {
	t.Helper()

	calls := []int{}
	subscriptionUserCacheInvalidator = func(userId int) error {
		calls = append(calls, userId)
		return nil
	}
	return &calls
}

func TestAdminBindSubscriptionInvalidatesUserCache(t *testing.T) {
	db := setupSubscriptionCacheTestDB(t)
	cacheInvalidations := trackSubscriptionCacheInvalidations(t)

	user := &User{Username: "bind_user", Group: "default", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)

	plan := &SubscriptionPlan{
		Title:         "bind-plan",
		Enabled:       true,
		DurationUnit:  "custom",
		CustomSeconds: 600,
		UpgradeGroup:  "vip",
		TotalAmount:   5000,
	}
	require.NoError(t, db.Create(plan).Error)

	msg, err := AdminBindSubscription(user.Id, plan.Id, "test")
	require.NoError(t, err)
	assert.Contains(t, msg, "vip")

	var refreshedUser User
	require.NoError(t, db.First(&refreshedUser, user.Id).Error)
	assert.Equal(t, "vip", refreshedUser.Group)

	var sub UserSubscription
	require.NoError(t, db.Where("user_id = ?", user.Id).First(&sub).Error)
	assert.Equal(t, "active", sub.Status)

	assert.Equal(t, []int{user.Id}, *cacheInvalidations)
}

func TestAdminInvalidateUserSubscriptionInvalidatesUserCache(t *testing.T) {
	db := setupSubscriptionCacheTestDB(t)
	cacheInvalidations := trackSubscriptionCacheInvalidations(t)

	user := &User{Username: "invalidate_user", Group: "vip", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)

	now := GetDBTimestamp()
	sub := &UserSubscription{
		UserId:        user.Id,
		PlanId:        1,
		AmountTotal:   5000,
		Status:        "active",
		StartTime:     now - 60,
		EndTime:       now + 600,
		UpgradeGroup:  "vip",
		PrevUserGroup: "default",
	}
	require.NoError(t, db.Create(sub).Error)

	msg, err := AdminInvalidateUserSubscription(sub.Id)
	require.NoError(t, err)
	assert.NotEmpty(t, msg)

	var refreshedUser User
	require.NoError(t, db.First(&refreshedUser, user.Id).Error)
	assert.Equal(t, "default", refreshedUser.Group)

	var refreshedSub UserSubscription
	require.NoError(t, db.First(&refreshedSub, sub.Id).Error)
	assert.Equal(t, "cancelled", refreshedSub.Status)
	assert.LessOrEqual(t, refreshedSub.EndTime, common.GetTimestamp())

	assert.Equal(t, []int{user.Id}, *cacheInvalidations)
}

func TestAdminDeleteUserSubscriptionInvalidatesUserCache(t *testing.T) {
	db := setupSubscriptionCacheTestDB(t)
	cacheInvalidations := trackSubscriptionCacheInvalidations(t)

	user := &User{Username: "delete_user", Group: "vip", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)

	now := GetDBTimestamp()
	sub := &UserSubscription{
		UserId:        user.Id,
		PlanId:        1,
		AmountTotal:   5000,
		Status:        "active",
		StartTime:     now - 60,
		EndTime:       now + 600,
		UpgradeGroup:  "vip",
		PrevUserGroup: "default",
	}
	require.NoError(t, db.Create(sub).Error)

	msg, err := AdminDeleteUserSubscription(sub.Id)
	require.NoError(t, err)
	assert.NotEmpty(t, msg)

	var refreshedUser User
	require.NoError(t, db.First(&refreshedUser, user.Id).Error)
	assert.Equal(t, "default", refreshedUser.Group)

	var count int64
	require.NoError(t, db.Model(&UserSubscription{}).Where("id = ?", sub.Id).Count(&count).Error)
	assert.Zero(t, count)

	assert.Equal(t, []int{user.Id}, *cacheInvalidations)
}

func TestExpireDueSubscriptionsInvalidatesUserCache(t *testing.T) {
	db := setupSubscriptionCacheTestDB(t)
	cacheInvalidations := trackSubscriptionCacheInvalidations(t)

	user := &User{Username: "expire_user", Group: "vip", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)

	now := GetDBTimestamp()
	sub := &UserSubscription{
		UserId:        user.Id,
		PlanId:        1,
		AmountTotal:   5000,
		Status:        "active",
		StartTime:     now - 600,
		EndTime:       now - 1,
		UpgradeGroup:  "vip",
		PrevUserGroup: "default",
	}
	require.NoError(t, db.Create(sub).Error)

	expiredCount, err := ExpireDueSubscriptions(10)
	require.NoError(t, err)
	assert.Equal(t, 1, expiredCount)

	var refreshedUser User
	require.NoError(t, db.First(&refreshedUser, user.Id).Error)
	assert.Equal(t, "default", refreshedUser.Group)

	var refreshedSub UserSubscription
	require.NoError(t, db.First(&refreshedSub, sub.Id).Error)
	assert.Equal(t, "expired", refreshedSub.Status)

	assert.Equal(t, []int{user.Id}, *cacheInvalidations)
}
