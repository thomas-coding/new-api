package model

import (
	"time"
)

const usageRankingTimezone = "Asia/Shanghai"
const usageRankingTokenSumExpr = "COALESCE(SUM(COALESCE(prompt_tokens, 0) + COALESCE(completion_tokens, 0)), 0)"

var beijingLocation = time.FixedZone("CST", 8*60*60)

type UsageRankingEntry struct {
	Rank         int    `json:"rank"`
	UserId       int    `json:"user_id"`
	Username     string `json:"username"`
	Quota        int    `json:"quota"`
	RequestCount int    `json:"request_count"`
	TokenCount   int    `json:"token_count"`
	IsMe         bool   `json:"is_me"`
}

type UsageRankingResult struct {
	Top            []UsageRankingEntry `json:"top"`
	Me             *UsageRankingEntry  `json:"me"`
	Limit          int                 `json:"limit"`
	Timezone       string              `json:"timezone"`
	StartTimestamp int64               `json:"start_timestamp"`
	EndTimestamp   int64               `json:"end_timestamp"`
}

func GetBeijingDayRange(now time.Time) (int64, int64) {
	beijingNow := now.In(beijingLocation)
	start := time.Date(beijingNow.Year(), beijingNow.Month(), beijingNow.Day(), 0, 0, 0, 0, beijingLocation)
	return start.Unix(), start.Add(24 * time.Hour).Unix()
}

func GetTodayUsageRanking(currentUserId int, limit int) (*UsageRankingResult, error) {
	return getDailyUsageRanking(currentUserId, limit, time.Now())
}

func getDailyUsageRanking(currentUserId int, limit int, now time.Time) (*UsageRankingResult, error) {
	limit = 10

	startTimestamp, endTimestamp := GetBeijingDayRange(now)
	top := make([]UsageRankingEntry, 0, limit)
	err := LOG_DB.Table("logs").
		Select("user_id, MAX(username) AS username, COALESCE(SUM(quota), 0) AS quota, COUNT(*) AS request_count, "+usageRankingTokenSumExpr+" AS token_count").
		Where("type = ? AND created_at >= ? AND created_at < ? AND user_id > 0", LogTypeConsume, startTimestamp, endTimestamp).
		Group("user_id").
		Order("token_count DESC").
		Order("request_count DESC").
		Order("user_id ASC").
		Limit(limit).
		Scan(&top).Error
	if err != nil {
		return nil, err
	}

	for idx := range top {
		top[idx].Rank = idx + 1
		top[idx].IsMe = top[idx].UserId == currentUserId
	}

	me, err := getTodayUsageRankingEntryForUser(currentUserId, startTimestamp, endTimestamp)
	if err != nil {
		return nil, err
	}
	if me != nil && me.RequestCount > 0 {
		rank, err := countTodayUsageRankingAhead(me, startTimestamp, endTimestamp)
		if err != nil {
			return nil, err
		}
		me.Rank = rank + 1
		me.IsMe = true
	}

	if me == nil && currentUserId > 0 {
		username, _ := GetUsernameById(currentUserId, false)
		me = &UsageRankingEntry{
			Rank:     0,
			UserId:   currentUserId,
			Username: username,
			IsMe:     true,
		}
	}

	return &UsageRankingResult{
		Top:            top,
		Me:             me,
		Limit:          limit,
		Timezone:       usageRankingTimezone,
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
	}, nil
}

func getTodayUsageRankingEntryForUser(userId int, startTimestamp int64, endTimestamp int64) (*UsageRankingEntry, error) {
	if userId <= 0 {
		return nil, nil
	}
	entry := &UsageRankingEntry{}
	err := LOG_DB.Table("logs").
		Select("user_id, MAX(username) AS username, COALESCE(SUM(quota), 0) AS quota, COUNT(*) AS request_count, "+usageRankingTokenSumExpr+" AS token_count").
		Where("type = ? AND created_at >= ? AND created_at < ? AND user_id = ?", LogTypeConsume, startTimestamp, endTimestamp, userId).
		Group("user_id").
		Scan(entry).Error
	if err != nil {
		return nil, err
	}
	if entry.UserId == 0 {
		return nil, nil
	}
	return entry, nil
}

func countTodayUsageRankingAhead(entry *UsageRankingEntry, startTimestamp int64, endTimestamp int64) (int, error) {
	if entry == nil || entry.UserId <= 0 {
		return 0, nil
	}

	rankedUsers := LOG_DB.Table("logs").
		Select("user_id, "+usageRankingTokenSumExpr+" AS token_count, COUNT(*) AS request_count").
		Where("type = ? AND created_at >= ? AND created_at < ? AND user_id > 0", LogTypeConsume, startTimestamp, endTimestamp).
		Group("user_id").
		Having(usageRankingTokenSumExpr+" > ? OR ("+usageRankingTokenSumExpr+" = ? AND COUNT(*) > ?) OR ("+usageRankingTokenSumExpr+" = ? AND COUNT(*) = ? AND user_id < ?)",
			entry.TokenCount,
			entry.TokenCount, entry.RequestCount,
			entry.TokenCount, entry.RequestCount, entry.UserId)

	var count int64
	err := LOG_DB.Table("(?) AS ranked_users", rankedUsers).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int(count), nil
}
