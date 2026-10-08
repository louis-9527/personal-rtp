package personalrtp

// StrategySwitchConfig 是业务方可配置的资金池档位策略表。
type StrategySwitchConfig struct {
	Tiers []StrategySwitchTier `json:"tiers"` // 资金池档位策略区间，按顺序匹配首个命中的区间
}

// StrategySwitchOption 用于配置单局档位策略。
type StrategySwitchOption func(*StrategySwitchConfig)

// WithStrategySwitchConfig 使用业务方传入的资金池档位策略表。
func WithStrategySwitchConfig(config StrategySwitchConfig) StrategySwitchOption {
	return func(target *StrategySwitchConfig) {
		*target = config
	}
}

// StrategySwitchTier 定义一个连续资金池档位区间的策略配置。
type StrategySwitchTier struct {
	MinLevel int `json:"minLevel"` // 档位区间下限，包含该值
	MaxLevel int `json:"maxLevel"` // 档位区间上限，包含该值

	ColdEnabled             bool    `json:"coldEnabled"`             // 是否允许触发冷水策略
	ColdMaxSpins            int64   `json:"coldMaxSpins"`            // 冷水策略最多执行局数
	ColdRecentRtpLowerLimit float64 `json:"coldRecentRtpLowerLimit"` // 触发冷水策略的最近局数 RTP 下限
	VeteranEnabled          bool    `json:"veteranEnabled"`          // 是否允许触发老玩家策略
	BankruptcyEnabled       bool    `json:"bankruptcyEnabled"`       // 是否允许触发破产策略
	RecoveryEnabled         bool    `json:"recoveryEnabled"`         // 是否允许触发补偿策略
	NewcomerEnabled         bool    `json:"newcomerEnabled"`         // 是否允许创建新手事件
}

// DefaultStrategySwitchConfig 返回默认的资金池档位策略表。
func DefaultStrategySwitchConfig() StrategySwitchConfig {
	return StrategySwitchConfig{
		Tiers: []StrategySwitchTier{
			{MinLevel: 1, MaxLevel: 5, VeteranEnabled: true, BankruptcyEnabled: true, RecoveryEnabled: true, NewcomerEnabled: true},
			{MinLevel: 6, MaxLevel: 6, ColdEnabled: true, ColdMaxSpins: 80, ColdRecentRtpLowerLimit: 1.25, BankruptcyEnabled: true, RecoveryEnabled: true, NewcomerEnabled: true},
			{MinLevel: 0, MaxLevel: 0, ColdEnabled: true, ColdMaxSpins: 100, ColdRecentRtpLowerLimit: 1.15, NewcomerEnabled: true},
		},
	}
}

// StrategySwitches 是业务方根据资金池档位为单局计算的策略配置。
// FundPoolHealthy 为 false 时，所有新策略均安全关闭。
type StrategySwitches struct {
	FundPoolHealthy bool `json:"fundPoolHealthy"` // 资金池缓存是否正常可用
	FundPoolLevel   int  `json:"fundPoolLevel"`   // 资金池档位，由业务方据此选择基础 RTP

	BankruptcyEnabled       bool    `json:"bankruptcyEnabled"`       // 是否允许触发破产策略
	VeteranEnabled          bool    `json:"veteranEnabled"`          // 是否允许触发老玩家策略
	RecoveryEnabled         bool    `json:"recoveryEnabled"`         // 是否允许触发补偿策略
	NewcomerEnabled         bool    `json:"newcomerEnabled"`         // 是否允许创建新手事件
	ColdEnabled             bool    `json:"coldEnabled"`             // 是否允许创建冷水事件
	ColdMaxSpins            int64   `json:"coldMaxSpins"`            // 冷水策略最多执行局数
	ColdRecentRtpLowerLimit float64 `json:"coldRecentRtpLowerLimit"` // 触发冷水策略的最近局数 RTP 下限
}

// NewStrategySwitches 根据资金池档位生成单局策略配置。
// opts 可由业务方传入覆盖默认档位策略表；未传入时使用 DefaultStrategySwitchConfig。
// fundPoolHealthy 为 false 时不依赖 fundPoolLevel，关闭所有新策略。
func NewStrategySwitches(fundPoolHealthy bool, fundPoolLevel int, opts ...StrategySwitchOption) StrategySwitches {
	switches := StrategySwitches{
		FundPoolHealthy: fundPoolHealthy,
		FundPoolLevel:   fundPoolLevel,
	}
	if !fundPoolHealthy {
		return switches
	}

	config := DefaultStrategySwitchConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&config)
		}
	}
	for _, tier := range config.Tiers {
		if fundPoolLevel < tier.MinLevel || fundPoolLevel > tier.MaxLevel {
			continue
		}
		switches.ColdEnabled = tier.ColdEnabled
		switches.ColdMaxSpins = tier.ColdMaxSpins
		switches.ColdRecentRtpLowerLimit = tier.ColdRecentRtpLowerLimit
		switches.VeteranEnabled = tier.VeteranEnabled
		switches.BankruptcyEnabled = tier.BankruptcyEnabled
		switches.RecoveryEnabled = tier.RecoveryEnabled
		switches.NewcomerEnabled = tier.NewcomerEnabled
		break
	}
	return switches
}
