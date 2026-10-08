package recovery

import (
	"github.com/SinonetCPGame/game-kit/domain/entity"
	"github.com/SinonetCPGame/game-kit/domain/service/predefined-result/v5"
)

const (
	defaultMinSessionBets      int64 = 20
	defaultSessionRtpLimit           = 0.95
	defaultRecoveryRecentBets  int   = 20
	defaultRecentRtpLimit            = 0.5
	defaultMinTriggerInterval  int64 = 30
	defaultMaxSessionTriggers  int64 = 2
	defaultTriggerProbability        = 0.2
	defaultRecoveryMinRate           = 0.5
	defaultRecoveryMaxRate           = 0.8
	defaultPayoutMultiplierMin       = 1
	defaultPayoutMultiplierMax       = 100
)

// config 是与具体结果类型无关的补偿策略可配置参数。
type config struct {
	MinSessionBets      int64                   // 触发补偿所需的会话普通投注最小局数。
	SessionRtpLimit     float64                 // 会话普通投注统计计算的 RTP 触发上限。
	RecentBets          int                     // 计算理论补偿金额时使用的最近普通投注局数。
	RecentRtpLimit      float64                 // 最近普通投注 RTP 的触发上限，包含边界。
	MinTriggerInterval  int64                   // 距离上次成功触发补偿所需间隔的会话内普通投注局数。
	MaxSessionTriggers  int64                   // 单个会话内最多成功触发的补偿次数。
	TriggerProbability  float64                 // 满足其他条件时的补偿触发概率，范围为 (0, 1]。
	RecoveryMinRate     float64                 // 理论补偿金额随机系数的下限。
	RecoveryMaxRate     float64                 // 理论补偿金额随机系数的上限。
	PayoutMultiplierMin float64                 // 理论派彩倍数的左开区间下限；倍数为理论补偿金额除以本局投注金额后加 1。
	PayoutMultiplierMax float64                 // 理论派彩倍数的右闭区间上限；倍数为理论补偿金额除以本局投注金额后加 1。
	ResultTypes         []predefined.ResultType // 随机结果的结果集类型。
	validator           any                     // 随机结果验证器，初始化时转换为具体结果类型。
}

// Config 是补偿策略的可配置参数。
type Config[R entity.IResult] struct {
	config
	Validator predefined.ResultValidator[R]
}

// Option 用于配置与结果类型无关的补偿策略参数。
type Option func(*config)

func defaultConfig[R entity.IResult]() Config[R] {
	return Config[R]{
		config: config{
			MinSessionBets:      defaultMinSessionBets,
			SessionRtpLimit:     defaultSessionRtpLimit,
			RecentBets:          defaultRecoveryRecentBets,
			RecentRtpLimit:      defaultRecentRtpLimit,
			MinTriggerInterval:  defaultMinTriggerInterval,
			MaxSessionTriggers:  defaultMaxSessionTriggers,
			TriggerProbability:  defaultTriggerProbability,
			RecoveryMinRate:     defaultRecoveryMinRate,
			RecoveryMaxRate:     defaultRecoveryMaxRate,
			PayoutMultiplierMin: defaultPayoutMultiplierMin,
			PayoutMultiplierMax: defaultPayoutMultiplierMax,
		},
	}
}

// WithMinSessionBets 设置触发补偿所需的会话普通投注最小局数。
func WithMinSessionBets(bets int64) Option {
	return func(c *config) {
		if bets > 0 {
			c.MinSessionBets = bets
		}
	}
}

// WithSessionRtpLimit 设置会话普通投注 RTP 的触发上限。
func WithSessionRtpLimit(limit float64) Option {
	return func(c *config) {
		if limit > 0 {
			c.SessionRtpLimit = limit
		}
	}
}

// WithRecentBets 设置计算理论补偿金额使用的最近普通投注局数。
func WithRecentBets(bets int) Option {
	return func(c *config) {
		if bets > 0 {
			c.RecentBets = bets
		}
	}
}

// WithRecentRtpLimit 设置最近普通投注 RTP 的触发上限。
func WithRecentRtpLimit(limit float64) Option {
	return func(c *config) {
		if limit > 0 {
			c.RecentRtpLimit = limit
		}
	}
}

// WithMinTriggerInterval 设置两次成功触发补偿间所需的最小普通投注局数。
func WithMinTriggerInterval(interval int64) Option {
	return func(c *config) {
		if interval >= 0 {
			c.MinTriggerInterval = interval
		}
	}
}

// WithMaxSessionTriggers 设置单个会话内最多成功触发补偿次数。
func WithMaxSessionTriggers(count int64) Option {
	return func(c *config) {
		if count > 0 {
			c.MaxSessionTriggers = count
		}
	}
}

// WithTriggerProbability 设置满足其他条件时的补偿触发概率。
func WithTriggerProbability(probability float64) Option {
	return func(c *config) {
		if probability > 0 && probability <= 1 {
			c.TriggerProbability = probability
		}
	}
}

// WithRecoveryRateRange 设置理论补偿金额随机系数范围。
func WithRecoveryRateRange(minimum, maximum float64) Option {
	return func(c *config) {
		if minimum > 0 && maximum >= minimum {
			c.RecoveryMinRate, c.RecoveryMaxRate = minimum, maximum
		}
	}
}

// WithPayoutMultiplierRange 设置理论派彩倍数范围。
func WithPayoutMultiplierRange(minimum, maximum float64) Option {
	return func(c *config) {
		if minimum >= 0 && maximum > minimum {
			c.PayoutMultiplierMin, c.PayoutMultiplierMax = minimum, maximum
		}
	}
}

// WithResultTypes 设置用于随机结果的结果集类型。
func WithResultTypes(resultTypes []predefined.ResultType) Option {
	return func(c *config) {
		if len(resultTypes) > 0 {
			c.ResultTypes = append([]predefined.ResultType(nil), resultTypes...)
		}
	}
}

// WithValidator 设置随机结果的自定义验证器；R 会由 validator 自动推断。
func WithValidator[R entity.IResult](validator predefined.ResultValidator[R]) Option {
	return func(c *config) {
		if validator != nil {
			c.validator = validator
		}
	}
}
