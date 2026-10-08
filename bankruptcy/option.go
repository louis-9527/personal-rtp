package bankruptcy

import (
	"github.com/SinonetCPGame/game-kit/domain/entity"
	"github.com/SinonetCPGame/game-kit/domain/service/predefined-result/v5"
)

// 破产策略默认参数。
const (
	defaultMinNormalSpins        int64 = 30
	defaultMinSessionBets        int64 = 30
	defaultBetAbbLowerMultiplier       = 0.1
	defaultBetAbbUpperMultiplier       = 10.0
)

// Stage 是破产策略 24 小时触发序列中的一个阶段。
// 返还系数采用左开右闭区间；随机值连续分布时端点差异不影响实际概率。
type Stage struct {
	EventID                     string  // 阶段对应的破产策略事件 ID。
	RrlLowerLimit               float64 // RRL（余额 / ABB）触发区间下限，包含边界。
	RrlUpperLimit               float64 // RRL（余额 / ABB）触发区间上限，包含边界。
	SessionRtpUpperLimit        float64 // 会话 RTP 触发上限，不包含边界。
	TriggerProbability          float64 // 满足其他条件时，本局触发策略的概率。
	RefundRateLowerLimit        float64 // 会话亏损返还系数随机区间下限。
	RefundRateUpperLimit        float64 // 会话亏损返还系数随机区间上限。
	PrincipalLossRateLowerLimit float64 // 会话本金亏损率下限。
}

// config 是与具体结果类型无关的破产策略可配置参数。
type config struct {
	MinNormalSpins        int64                   // 计算 ABB 所需的最近普通投注局数。
	MinSessionBets        int64                   // 触发策略所需的会话投注总笔数，普通投注和 buybonus 都计入。
	BetAbbLowerMultiplier float64                 // 当前投注 / ABB 触发区间下限。
	BetAbbUpperMultiplier float64                 // 当前投注 / ABB 触发区间上限。
	Stages                []Stage                 // 以触发顺序排列的 24 小时破产策略阶段。
	ResultTypes           []predefined.ResultType // 用于随机结果的结果集类型。
	validator             any
}

// Config 是破产策略的可配置参数。
type Config[R entity.IResult] struct {
	config
	Validator predefined.ResultValidator[R] // 随机结果验证器。
}

// Option 用于配置与结果类型无关的破产策略参数。
type Option func(*config)

// defaultConfig 返回破产策略的默认配置。
func defaultConfig[R entity.IResult]() Config[R] {
	return Config[R]{config: config{
		MinNormalSpins: defaultMinNormalSpins, MinSessionBets: defaultMinSessionBets,
		BetAbbLowerMultiplier: defaultBetAbbLowerMultiplier, BetAbbUpperMultiplier: defaultBetAbbUpperMultiplier,
		Stages: []Stage{
			{EventID: "B1", RrlLowerLimit: 3, RrlUpperLimit: 10000, SessionRtpUpperLimit: 0.5, TriggerProbability: 0.1,
				RefundRateLowerLimit: 0.7, RefundRateUpperLimit: 0.75, PrincipalLossRateLowerLimit: 0.5},
			{EventID: "B2", RrlLowerLimit: 3, RrlUpperLimit: 12, SessionRtpUpperLimit: 0.4, TriggerProbability: 0.2,
				RefundRateLowerLimit: 0.7, RefundRateUpperLimit: 0.8, PrincipalLossRateLowerLimit: 0.6},
			{EventID: "B3", RrlLowerLimit: 2, RrlUpperLimit: 8, SessionRtpUpperLimit: 0.4, TriggerProbability: 0.15,
				RefundRateLowerLimit: 0.45, RefundRateUpperLimit: 0.5, PrincipalLossRateLowerLimit: 1.5},
		},
	}}
}

// WithMinNormalSpins 设置计算 ABB 所需的最近普通投注局数。
func WithMinNormalSpins(spins int64) Option {
	return func(c *config) {
		if spins > 0 {
			c.MinNormalSpins = spins
		}
	}
}

// WithMinSessionBets 设置触发策略所需的会话投注总笔数，普通投注和 buybonus 都计入。
func WithMinSessionBets(bets int64) Option {
	return func(c *config) {
		if bets > 0 {
			c.MinSessionBets = bets
		}
	}
}

// WithBetAbbMultiplierRange 设置当前投注 / ABB 的触发倍数区间，边界值包含在内。
func WithBetAbbMultiplierRange(lower, upper float64) Option {
	return func(c *config) {
		if lower >= 0 && upper >= lower {
			c.BetAbbLowerMultiplier, c.BetAbbUpperMultiplier = lower, upper
		}
	}
}

// WithStages 设置滚动 24 小时内按顺序触发的阶段。
// 仅当所有阶段配置有效时才应用，避免产生无法触发或无事件 ID 的阶段。
func WithStages(stages ...Stage) Option {
	return func(c *config) {
		if len(stages) == 0 {
			return
		}
		configured := make([]Stage, len(stages))
		for i, stage := range stages {
			if stage.EventID == "" ||
				stage.RrlLowerLimit < 0 || stage.RrlUpperLimit < stage.RrlLowerLimit ||
				stage.SessionRtpUpperLimit <= 0 ||
				stage.TriggerProbability < 0 || stage.TriggerProbability > 1 ||
				stage.RefundRateLowerLimit < 0 || stage.RefundRateUpperLimit < stage.RefundRateLowerLimit ||
				stage.PrincipalLossRateLowerLimit < 0 {
				return
			}
			configured[i] = stage
		}
		c.Stages = configured
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
