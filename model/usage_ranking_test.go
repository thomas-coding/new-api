package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedRankingUser(t *testing.T, id int) {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id:       id,
		Username: fmt.Sprintf("user%d", id),
		Password: "password",
		AffCode:  fmt.Sprintf("aff%d", id),
	}).Error)
}

func seedConsumeLog(t *testing.T, userId int, createdAt int64, quota int, promptTokens int, completionTokens int) {
	t.Helper()
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           userId,
		Username:         fmt.Sprintf("user%d", userId),
		CreatedAt:        createdAt,
		Type:             LogTypeConsume,
		Quota:            quota,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
	}).Error)
}

func TestGetBeijingDayRange(t *testing.T) {
	now := time.Date(2026, 5, 2, 18, 30, 0, 0, time.UTC)
	start, end := GetBeijingDayRange(now)

	assert.Equal(t, time.Date(2026, 5, 3, 0, 0, 0, 0, beijingLocation).Unix(), start)
	assert.Equal(t, time.Date(2026, 5, 4, 0, 0, 0, 0, beijingLocation).Unix(), end)
}

func TestGetDailyUsageRankingReturnsTopAndCurrentUser(t *testing.T) {
	truncateTables(t)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, beijingLocation)
	start, end := GetBeijingDayRange(now)

	for i := 1; i <= 12; i++ {
		seedRankingUser(t, i)
		seedConsumeLog(t, i, start+int64(i), i*100, 13-i, 0)
	}
	seedConsumeLog(t, 1, start-1, 100000, 1, 1)
	seedConsumeLog(t, 1, end, 100000, 1, 1)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    1,
		Username:  "user1",
		CreatedAt: start + 10,
		Type:      LogTypeError,
		Quota:     100000,
	}).Error)

	ranking, err := getDailyUsageRanking(12, 10, now)
	require.NoError(t, err)

	require.Len(t, ranking.Top, 10)
	assert.Equal(t, "Asia/Shanghai", ranking.Timezone)
	assert.Equal(t, 1, ranking.Top[0].Rank)
	assert.Equal(t, 1, ranking.Top[0].UserId)
	assert.Equal(t, 100, ranking.Top[0].Quota)
	assert.Equal(t, 1, ranking.Top[0].RequestCount)
	assert.Equal(t, 12, ranking.Top[0].TokenCount)
	require.NotNil(t, ranking.Me)
	assert.Equal(t, 12, ranking.Me.UserId)
	assert.Equal(t, 12, ranking.Me.Rank)
	assert.True(t, ranking.Me.IsMe)
	assert.Equal(t, 1200, ranking.Me.Quota)
	assert.Equal(t, 1, ranking.Me.TokenCount)
}

func TestGetDailyUsageRankingCurrentUserWithoutUsage(t *testing.T) {
	truncateTables(t)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, beijingLocation)
	start, _ := GetBeijingDayRange(now)
	seedRankingUser(t, 1)
	seedRankingUser(t, 2)
	seedConsumeLog(t, 1, start+1, 100, 5, 6)

	ranking, err := getDailyUsageRanking(2, 10, now)
	require.NoError(t, err)

	require.Len(t, ranking.Top, 1)
	require.NotNil(t, ranking.Me)
	assert.Equal(t, 0, ranking.Me.Rank)
	assert.Equal(t, "user2", ranking.Me.Username)
	assert.True(t, ranking.Me.IsMe)
}
