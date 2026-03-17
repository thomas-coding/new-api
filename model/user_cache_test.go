package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestUpdateUserCache_SkipsWhenRedisClientNil(t *testing.T) {
	originalEnabled := common.RedisEnabled
	originalRDB := common.RDB
	common.RedisEnabled = true
	common.RDB = nil
	t.Cleanup(func() {
		common.RedisEnabled = originalEnabled
		common.RDB = originalRDB
	})

	err := updateUserCache(User{Id: 1, Username: "demo", Group: "default"})
	if err != nil {
		t.Fatalf("updateUserCache() error = %v, want nil", err)
	}
}

func TestShouldUpdateRedis_RequiresClient(t *testing.T) {
	originalEnabled := common.RedisEnabled
	originalRDB := common.RDB
	common.RedisEnabled = true
	common.RDB = nil
	t.Cleanup(func() {
		common.RedisEnabled = originalEnabled
		common.RDB = originalRDB
	})

	if shouldUpdateRedis(true, nil) {
		t.Fatal("shouldUpdateRedis() = true, want false when Redis client is nil")
	}
}
