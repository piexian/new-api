package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestPricingCreatedTime(t *testing.T) {
	const modelName = "pricing-created-middle-tail"
	metadataNames := map[int]string{
		NameRuleExact:    modelName,
		NameRulePrefix:   "pricing-created-",
		NameRuleSuffix:   "-tail",
		NameRuleContains: "middle",
	}
	metadataTimes := map[int]int64{
		NameRuleExact:    1700000001,
		NameRulePrefix:   1700000002,
		NameRuleSuffix:   1700000003,
		NameRuleContains: 1700000004,
	}

	tests := []struct {
		name         string
		rules        []int
		disabled     bool
		wantTime     int64
		wantFiltered bool
	}{
		{
			name:     "exact record takes precedence over rules",
			rules:    []int{NameRuleContains, NameRuleSuffix, NameRulePrefix, NameRuleExact},
			wantTime: metadataTimes[NameRuleExact],
		},
		{
			name:     "prefix rule takes precedence over suffix and contains",
			rules:    []int{NameRuleContains, NameRuleSuffix, NameRulePrefix},
			wantTime: metadataTimes[NameRulePrefix],
		},
		{
			name:     "suffix rule takes precedence over contains",
			rules:    []int{NameRuleContains, NameRuleSuffix},
			wantTime: metadataTimes[NameRuleSuffix],
		},
		{
			name:     "contains rule uses rule record time",
			rules:    []int{NameRuleContains},
			wantTime: metadataTimes[NameRuleContains],
		},
		{
			name: "synthetic metadata without models record has zero time",
		},
		{
			name:         "disabled exact record does not fall back to enabled rule",
			rules:        []int{NameRulePrefix, NameRuleExact},
			disabled:     true,
			wantFiltered: true,
		},
		{
			name:         "disabled prefix record does not fall back to enabled rule",
			rules:        []int{NameRuleSuffix, NameRulePrefix},
			disabled:     true,
			wantFiltered: true,
		},
		{
			name:         "disabled suffix record remains filtered",
			rules:        []int{NameRuleSuffix},
			disabled:     true,
			wantFiltered: true,
		},
		{
			name:         "disabled contains record remains filtered",
			rules:        []int{NameRuleContains},
			disabled:     true,
			wantFiltered: true,
		},
	}

	// These subtests share the package database and global pricing cache.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetPricingEndpointTestTables(t)
			insertPricingEndpointChannel(t, 501, constant.ChannelTypeOpenAI, dto.ChannelOtherSettings{})
			insertPricingEndpointAbility(t, 501, modelName)

			for i, rule := range tt.rules {
				meta := Model{
					ModelName:   metadataNames[rule],
					NameRule:    rule,
					Status:      1,
					CreatedTime: metadataTimes[rule],
				}
				require.NoError(t, DB.Create(&meta).Error)
				if tt.disabled && i == len(tt.rules)-1 {
					// Update explicitly because GORM defaults a zero status to 1 on create.
					require.NoError(t, DB.Model(&meta).Update("status", 0).Error)
				}
			}

			InitChannelCache()
			pricings := GetPricing()

			var persisted []Model
			require.NoError(t, DB.Find(&persisted).Error)
			require.Len(t, persisted, len(tt.rules))
			for _, meta := range persisted {
				require.Equal(t, metadataTimes[meta.NameRule], meta.CreatedTime)
			}

			if tt.wantFiltered {
				require.Empty(t, pricings)
				return
			}
			require.Len(t, pricings, 1)
			require.Equal(t, modelName, pricings[0].ModelName)
			require.Equal(t, tt.wantTime, pricings[0].CreatedTime)

			data, err := common.Marshal(pricings)
			require.NoError(t, err)
			var decoded []struct {
				CreatedTime *int64 `json:"created_time"`
			}
			require.NoError(t, common.Unmarshal(data, &decoded))
			require.Len(t, decoded, 1)
			require.NotNil(t, decoded[0].CreatedTime, "created_time must be present even when zero")
			require.Equal(t, tt.wantTime, *decoded[0].CreatedTime)
		})
	}
}
