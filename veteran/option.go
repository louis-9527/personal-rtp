package veteran

import (
	"github.com/SinonetCPGame/game-kit/domain/entity"
	"github.com/SinonetCPGame/game-kit/domain/service/predefined-result/v5"
)

var (
	defaultMinHistoricalBets   int64   = 500
	defaultHistoricalRtpLimit          = 0.85
	defaultMaxSessionTriggers  int64   = 10
	defaultBetPayoutMultiplier float64 = 15
	defaultPayoutLossRate              = 0.8
	defaultMinimumProbability          = 0.02
	defaultMaximumProbability          = 0.1
	defaultOddsRange                   = []float64{10, 15}
)

// config 是与具体结果类型无关的老玩家策略可配置参数。
type config struct {
	MinHistoricalBets   int64                   // 触发策略所需的历史投注总局数。
	HistoricalRtpLimit  float64                 // 触发策略所需的历史 RTP 上限，包含边界。
	MaxSessionTriggers  int64                   // 单个策略会话内最多成功触发次数。
	BetPayoutMultiplier float64                 // 单笔最大派彩必须大于当前投注的倍数。
	PayoutLossRate      float64                 // 单笔最大派彩占历史亏损的比例。
	MinimumProbability  float64                 // RTP 等于上限时的基础触发概率。
	MaximumProbability  float64                 // RTP 每降低一个百分点递增后的触发概率上限。
	ResultTypes         []predefined.ResultType // 随机结果的结果集类型。
	OddsRange           []float64               // 随机结果倍数区间。
	validator           any
}

// Config 是老玩家策略的可配置参数。
type Config[R entity.IResult] struct {
	config
	Validator predefined.ResultValidator[R] // 随机结果验证器。
}

// Option 用于配置与结果类型无关的老玩家策略参数。
type Option func(*config)

func defaultConfig[R entity.IResult]() Config[R] {
	return Config[R]{config: config{
		MinHistoricalBets:   defaultMinHistoricalBets,
		HistoricalRtpLimit:  defaultHistoricalRtpLimit,
		MaxSessionTriggers:  defaultMaxSessionTriggers,
		BetPayoutMultiplier: defaultBetPayoutMultiplier,
		PayoutLossRate:      defaultPayoutLossRate,
		MinimumProbability:  defaultMinimumProbability,
		MaximumProbability:  defaultMaximumProbability,
		OddsRange:           defaultOddsRange,
	}}
}

// WithMinHistoricalBets 设置触发策略所需的历史投注总局数。
func WithMinHistoricalBets(bets int64) Option {
	return func(c *config) {
		if bets > 0 {
			c.MinHistoricalBets = bets
		}
	}
}

// WithHistoricalRtpLimit 设置历史 RTP 触发上限。
func WithHistoricalRtpLimit(limit float64) Option {
	return func(c *config) {
		if limit > 0 && limit <= 1 {
			c.HistoricalRtpLimit = limit
		}
	}
}

// WithMaxSessionTriggers 设置单个会话内最多成功触发次数。
func WithMaxSessionTriggers(count int64) Option {
	return func(c *config) {
		if count > 0 {
			c.MaxSessionTriggers = count
		}
	}
}

// WithBetPayoutMultiplier 设置单笔投注相对历史亏损的派彩倍数阈值。
func WithBetPayoutMultiplier(multiplier float64) Option {
	return func(c *config) {
		if multiplier > 0 {
			c.BetPayoutMultiplier = multiplier
		}
	}
}

// WithPayoutLossRate 设置单笔最大派彩可占历史亏损的比例。
func WithPayoutLossRate(rate float64) Option {
	return func(c *config) {
		if rate > 0 && rate <= 1 {
			c.PayoutLossRate = rate
		}
	}
}

// WithProbabilityRange 设置 RTP 触发概率的最小值和最大值。
func WithProbabilityRange(minimum, maximum float64) Option {
	return func(c *config) {
		if minimum >= 0 && maximum >= minimum && maximum <= 1 {
			c.MinimumProbability, c.MaximumProbability = minimum, maximum
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

// WithOddsRange 设置随机结果的倍数区间，支持单个精确值或两个区间边界。
func WithOddsRange(oddsRange ...float64) Option {
	return func(c *config) {
		if len(oddsRange) != 1 && len(oddsRange) != 2 {
			return
		}
		for _, odds := range oddsRange {
			if odds < 0 {
				return
			}
		}
		if len(oddsRange) == 2 && oddsRange[1] < oddsRange[0] {
			return
		}
		c.OddsRange = append([]float64(nil), oddsRange...)
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
