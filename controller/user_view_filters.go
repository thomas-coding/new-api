package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

const userFacingDefaultGroup = "default"

func isUserFacingGPTModel(name string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	return strings.HasPrefix(trimmed, "gpt")
}

func filterUserFacingModelNames(models []string) []string {
	filtered := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, modelName := range models {
		if !isUserFacingGPTModel(modelName) {
			continue
		}
		if _, ok := seen[modelName]; ok {
			continue
		}
		seen[modelName] = struct{}{}
		filtered = append(filtered, modelName)
	}
	return filtered
}

func filterUserFacingOpenAIModels(models []dto.OpenAIModels) []dto.OpenAIModels {
	filtered := make([]dto.OpenAIModels, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, modelItem := range models {
		if !isUserFacingGPTModel(modelItem.Id) {
			continue
		}
		if _, ok := seen[modelItem.Id]; ok {
			continue
		}
		seen[modelItem.Id] = struct{}{}
		filtered = append(filtered, modelItem)
	}
	return filtered
}

func pickPrimaryUserFacingGroup(usableGroups map[string]string, fallback string) string {
	if _, ok := usableGroups[userFacingDefaultGroup]; ok {
		return userFacingDefaultGroup
	}
	if fallback != "" {
		if _, ok := usableGroups[fallback]; ok {
			return fallback
		}
	}
	for groupName := range usableGroups {
		return groupName
	}
	return userFacingDefaultGroup
}

func filterUserFacingUsableGroups(usableGroups map[string]string, fallback string) map[string]string {
	primaryGroup := pickPrimaryUserFacingGroup(usableGroups, fallback)
	if desc, ok := usableGroups[primaryGroup]; ok {
		return map[string]string{primaryGroup: desc}
	}
	return map[string]string{}
}

func filterUserFacingGroupRatio(groupRatio map[string]float64, usableGroups map[string]string, fallback string) map[string]float64 {
	primaryGroup := pickPrimaryUserFacingGroup(usableGroups, fallback)
	if ratio, ok := groupRatio[primaryGroup]; ok {
		return map[string]float64{primaryGroup: ratio}
	}
	return map[string]float64{}
}

func filterUserFacingPricing(pricing []model.Pricing, usableGroups map[string]string, fallback string) []model.Pricing {
	primaryGroup := pickPrimaryUserFacingGroup(usableGroups, fallback)
	filtered := make([]model.Pricing, 0, len(pricing))
	for _, pricingItem := range pricing {
		if !isUserFacingGPTModel(pricingItem.ModelName) {
			continue
		}
		enabledGroups := make([]string, 0, 1)
		for _, groupName := range pricingItem.EnableGroup {
			if groupName == primaryGroup {
				enabledGroups = append(enabledGroups, groupName)
				break
			}
		}
		if len(enabledGroups) == 0 {
			continue
		}
		cloned := pricingItem
		cloned.EnableGroup = enabledGroups
		filtered = append(filtered, cloned)
	}
	return filtered
}

func filterUserFacingVendors(pricing []model.Pricing, vendors []model.PricingVendor) []model.PricingVendor {
	allowedVendorIDs := make(map[int]struct{})
	for _, pricingItem := range pricing {
		if pricingItem.VendorID > 0 {
			allowedVendorIDs[pricingItem.VendorID] = struct{}{}
		}
	}
	if len(allowedVendorIDs) == 0 {
		return []model.PricingVendor{}
	}
	filtered := make([]model.PricingVendor, 0, len(vendors))
	for _, vendor := range vendors {
		if _, ok := allowedVendorIDs[vendor.ID]; ok {
			filtered = append(filtered, vendor)
		}
	}
	return filtered
}
