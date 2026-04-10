package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type LotteryTierSetting struct {
	Name        string  `json:"name"`
	Amount      int     `json:"amount"`
	Probability float64 `json:"probability"`
}

type LotterySetting struct {
	WeeklyDay            int                  `json:"weekly_day"`
	MythBroadcastEnabled bool                 `json:"myth_broadcast_enabled"`
	Tiers                []LotteryTierSetting `json:"tiers"`
}

var lotterySetting = LotterySetting{
	WeeklyDay:            0,
	MythBroadcastEnabled: true,
	Tiers: []LotteryTierSetting{
		{Name: "普通", Amount: 3, Probability: 82},
		{Name: "稀有", Amount: 8, Probability: 13},
		{Name: "史诗", Amount: 20, Probability: 4},
		{Name: "传说", Amount: 50, Probability: 0.8},
		{Name: "神话", Amount: 200, Probability: 0.2},
	},
}

func init() {
	config.GlobalConfig.Register("lottery_setting", &lotterySetting)
}

func GetLotterySetting() *LotterySetting {
	return &lotterySetting
}

func GetNormalizedLotterySetting() LotterySetting {
	normalized := lotterySetting
	normalized.WeeklyDay = normalizeLotteryWeeklyDay(normalized.WeeklyDay)
	normalized.Tiers = normalizeLotteryTiers(normalized.Tiers)
	return normalized
}

func normalizeLotteryWeeklyDay(day int) int {
	if day < 0 || day > 7 {
		return 0
	}
	return day
}

func normalizeLotteryTiers(tiers []LotteryTierSetting) []LotteryTierSetting {
	defaults := []LotteryTierSetting{
		{Name: "普通", Amount: 3, Probability: 82},
		{Name: "稀有", Amount: 8, Probability: 13},
		{Name: "史诗", Amount: 20, Probability: 4},
		{Name: "传说", Amount: 50, Probability: 0.8},
		{Name: "神话", Amount: 200, Probability: 0.2},
	}
	if len(tiers) != len(defaults) {
		return defaults
	}
	normalized := make([]LotteryTierSetting, len(defaults))
	for i := range defaults {
		normalized[i] = defaults[i]
		if tiers[i].Name != "" {
			normalized[i].Name = tiers[i].Name
		}
		if tiers[i].Amount > 0 {
			normalized[i].Amount = tiers[i].Amount
		}
		if tiers[i].Probability >= 0 {
			normalized[i].Probability = tiers[i].Probability
		}
	}
	return normalized
}
