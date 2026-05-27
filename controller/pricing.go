package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func filterPricingByUsableGroups(pricing []model.Pricing, usableGroup map[string]string) []model.Pricing {
	if len(pricing) == 0 {
		return pricing
	}

	filtered := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		if common.StringsContains(item.EnableGroup, "all") {
			filtered = append(filtered, item)
			continue
		}
		for _, group := range item.EnableGroup {
			if _, ok := usableGroup[group]; ok {
				filtered = append(filtered, item)
				break
			}
		}
	}
	return filtered
}

func publicPricingUsableGroups(groupRatio map[string]float64) map[string]string {
	usableGroup := setting.GetUserUsableGroupsCopy()
	for group := range groupRatio {
		if group == "" || group == "auto" {
			continue
		}
		if _, ok := usableGroup[group]; !ok {
			usableGroup[group] = setting.GetUsableGroupDescription(group)
		}
	}
	return usableGroup
}

func pricingAutoGroups(usableGroup map[string]string) []string {
	autoGroups := make([]string, 0)
	for _, group := range setting.GetAutoGroups() {
		if _, ok := usableGroup[group]; ok {
			autoGroups = append(autoGroups, group)
		}
	}
	return autoGroups
}

func GetPricing(c *gin.Context) {
	pricing := model.GetPricing()
	userId, exists := c.Get("id")
	usableGroup := map[string]string{}
	groupRatio := map[string]float64{}
	for s, f := range ratio_setting.GetGroupRatioCopy() {
		groupRatio[s] = f
	}
	var group string
	if exists {
		user, err := model.GetUserCache(userId.(int))
		if err == nil {
			group = user.Group
			for g := range groupRatio {
				ratio, ok := ratio_setting.GetGroupGroupRatio(group, g)
				if ok {
					groupRatio[g] = ratio
				}
			}
		}
	}

	if exists {
		usableGroup = service.GetUserUsableGroups(group)
	} else {
		usableGroup = publicPricingUsableGroups(groupRatio)
	}
	pricing = filterPricingByUsableGroups(pricing, usableGroup)
	// check groupRatio contains usableGroup
	for group := range ratio_setting.GetGroupRatioCopy() {
		if _, ok := usableGroup[group]; !ok {
			delete(groupRatio, group)
		}
	}

	c.JSON(200, gin.H{
		"success":            true,
		"data":               pricing,
		"vendors":            model.GetVendors(),
		"group_ratio":        groupRatio,
		"usable_group":       usableGroup,
		"supported_endpoint": model.GetSupportedEndpointMap(),
		"auto_groups":        pricingAutoGroups(usableGroup),
		"pricing_version":    "a42d372ccf0b5dd13ecf71203521f9d2",
	})
}

func ResetModelRatio(c *gin.Context) {
	defaultStr := ratio_setting.DefaultModelRatio2JSONString()
	err := model.UpdateOption("ModelRatio", defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	err = ratio_setting.UpdateModelRatioByJSONString(defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "重置模型倍率成功",
	})
}
