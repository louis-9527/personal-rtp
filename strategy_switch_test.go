package personalrtp

import "testing"

func TestNewStrategySwitches(t *testing.T) {
	tests := []struct {
		name                    string
		fundPoolLevel           int
		fundPoolHealthy         bool
		bankruptcyEnabled       bool
		veteranEnabled          bool
		recoveryEnabled         bool
		newcomerEnabled         bool
		coldEnabled             bool
		coldMaxSpins            int64
		coldRecentRtpLowerLimit float64
	}{
		{
			name:              "资金池异常时关闭所有策略",
			fundPoolLevel:     1,
			fundPoolHealthy:   false,
			bankruptcyEnabled: false,
			veteranEnabled:    false,
			recoveryEnabled:   false,
			newcomerEnabled:   false,
			coldEnabled:       false,
		},
		{
			name:                    "零档开启冷水策略",
			fundPoolLevel:           0,
			fundPoolHealthy:         true,
			bankruptcyEnabled:       false,
			veteranEnabled:          false,
			recoveryEnabled:         false,
			newcomerEnabled:         false,
			coldEnabled:             true,
			coldMaxSpins:            80,
			coldRecentRtpLowerLimit: 1.10,
		},
		{
			name:              "一至五档开启破产老玩家补偿和新手策略",
			fundPoolLevel:     5,
			fundPoolHealthy:   true,
			bankruptcyEnabled: true,
			veteranEnabled:    true,
			recoveryEnabled:   true,
			newcomerEnabled:   true,
			coldEnabled:       false,
		},
		{
			name:                    "第六档开启破产冷水补偿和新手策略",
			fundPoolLevel:           6,
			fundPoolHealthy:         true,
			bankruptcyEnabled:       true,
			veteranEnabled:          false,
			recoveryEnabled:         true,
			newcomerEnabled:         true,
			coldEnabled:             true,
			coldMaxSpins:            50,
			coldRecentRtpLowerLimit: 1.30,
		},
		{
			name:              "未配置档位关闭所有策略",
			fundPoolLevel:     7,
			fundPoolHealthy:   true,
			bankruptcyEnabled: false,
			veteranEnabled:    false,
			recoveryEnabled:   false,
			newcomerEnabled:   false,
			coldEnabled:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switches := NewStrategySwitches(tt.fundPoolHealthy, tt.fundPoolLevel)
			if switches.FundPoolHealthy != tt.fundPoolHealthy ||
				switches.FundPoolLevel != tt.fundPoolLevel ||
				switches.BankruptcyEnabled != tt.bankruptcyEnabled ||
				switches.VeteranEnabled != tt.veteranEnabled ||
				switches.RecoveryEnabled != tt.recoveryEnabled ||
				switches.NewcomerEnabled != tt.newcomerEnabled ||
				switches.ColdEnabled != tt.coldEnabled ||
				switches.ColdMaxSpins != tt.coldMaxSpins ||
				switches.ColdRecentRtpLowerLimit != tt.coldRecentRtpLowerLimit {
				t.Fatalf("unexpected switches: %+v", switches)
			}
		})
	}
}

func TestNewStrategySwitchesWithCustomConfig(t *testing.T) {
	config := StrategySwitchConfig{
		Tiers: []StrategySwitchTier{
			{MinLevel: 2, MaxLevel: 3, ColdEnabled: true, ColdMaxSpins: 20, ColdRecentRtpLowerLimit: 1.50},
		},
	}

	switches := NewStrategySwitches(true, 2, WithStrategySwitchConfig(config))
	if switches.FundPoolLevel != 2 ||
		!switches.ColdEnabled ||
		switches.ColdMaxSpins != 20 ||
		switches.ColdRecentRtpLowerLimit != 1.50 {
		t.Fatalf("unexpected switches: %+v", switches)
	}
}
