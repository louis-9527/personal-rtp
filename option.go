package personalrtp

import "time"

const (
	defaultRecentListMaxLen                      = 200                 // 最近普通投注列表最大长度
	defaultRecentStatsTTL                        = 24 * time.Hour      // 最近局数 RTP 缓存过期时间
	defaultRecoveryRecentSpins             int64 = 100                 // 触发 recovery 所需最近局数
	defaultRecoveryRecentRtpUpperLimit           = 0.6                 // 触发 recovery 的最近局数 RTP 上限
	defaultRecoveryRecentCvUpperLimit            = 1.0                 // 触发 recovery 的最近局数下注 CV 上限
	defaultRecoveryMaxSpins                int64 = 50                  // recovery 策略最多执行局数
	defaultRecoveryExitRtp                       = 0.9                 // 退出 recovery 的总 RTP 下限（最近局数快照 + 策略普通投注合并计算）
	defaultRecoveryExitMedianBetMultiplier       = 5.0                 // 退出 recovery 的当前下注 / 触发时中位下注倍数
	defaultRecoveryActiveTTL                     = 30 * 24 * time.Hour // recovery 事件最长持续时间
	defaultRecoveryCooldownTTL                   = 4 * time.Hour       // recovery 结束后冷却时间
	defaultRecoverySelectedRtpMode               = 0                   // recovery 事件使用的 RTP mode

	defaultNewcomerMaxMonthlySpins int64 = 200                 // 新手事件允许的近一个月普通投注最大局数 M
	defaultNewcomerTargetMinSpins  int64 = 100                 // 新手事件随机判定局数 N 下限
	defaultNewcomerTargetMaxSpins  int64 = 200                 // 新手事件随机判定局数 N 上限
	defaultNewcomerFixedRtpWeight        = 0.37                // 新手事件固定 RTP 权重
	defaultNewcomerSelectedRtpMode       = 0                   // 新手事件使用的 RTP mode
	defaultNewcomerActiveTTL             = 30 * 24 * time.Hour // 新手事件最长持续时间
	defaultNewcomerCooldownTTL           = 30 * 24 * time.Hour // 新手事件结束后冷却时间

	defaultColdRecentSpins         int64 = 100                 // 触发冷水事件所需最近普通投注局数
	defaultColdRecentRtpLowerLimit       = 1.3                 // 触发冷水事件的最近局数 RTP 下限
	defaultColdMaxSpins            int64 = 50                  // 冷水事件最多执行局数
	defaultColdSelectedRtpMode           = 0                   // 冷水事件使用的 RTP mode
	defaultColdExitRtp                   = 1.0                 // 冷水事件退出的总 RTP 上限
	defaultColdActiveTTL                 = 30 * 24 * time.Hour // 冷水事件最长持续时间
	defaultColdCooldownTTL               = 4 * time.Hour       // 冷水事件结束后冷却时间
)

// Option 用于配置个人 RTP 事件服务。
type Option func(*Config)

type Config struct {
	recentListMaxLen int64         // 最近普通投注列表最大长度
	recentStatsTTL   time.Duration // 最近局数 RTP 缓存过期时间

	recoveryRecentSpins             int64         // 触发 recovery 所需最近局数
	recoveryRecentRtpUpperLimit     float64       // 触发 recovery 的最近局数 RTP 上限
	recoveryRecentCvUpperLimit      float64       // 触发 recovery 的最近局数下注 CV 上限
	recoveryMaxSpins                int64         // recovery 事件最多执行局数
	recoveryExitRtp                 float64       // 退出 recovery 的总 RTP 下限
	recoveryExitMedianBetMultiplier float64       // 退出 recovery 的当前下注 / 触发时中位下注倍数
	recoveryActiveTTL               time.Duration // recovery 事件最长持续时间
	recoveryCooldownTTL             time.Duration // recovery 结束后冷却时间
	recoverySelectedRtpMode         int           // recovery 事件使用的 RTP mode

	newcomerMaxMonthlySpins int64         // 新手事件允许的近一个月普通投注最大局数 M
	newcomerTargetMinSpins  int64         // 新手事件随机判定局数 N 下限
	newcomerTargetMaxSpins  int64         // 新手事件随机判定局数 N 上限
	newcomerFixedRtpWeight  float64       // 新手事件固定 RTP 权重
	newcomerSelectedRtpMode int           // 新手事件使用的 RTP mode
	newcomerActiveTTL       time.Duration // 新手事件最长持续时间
	newcomerCooldownTTL     time.Duration // 新手事件结束后冷却时间

	coldRecentSpins         int64         // 触发冷水事件所需最近普通投注局数
	coldRecentRtpLowerLimit float64       // 触发冷水事件的最近局数 RTP 下限
	coldMaxSpins            int64         // 冷水事件最多执行局数
	coldSelectedRtpMode     int           // 冷水事件使用的 RTP mode
	coldExitRtp             float64       // 冷水事件退出的总 RTP 上限
	coldActiveTTL           time.Duration // 冷水事件最长持续时间
	coldCooldownTTL         time.Duration // 冷水事件结束后冷却时间
}

func defaultConfig() Config {
	return Config{
		recentListMaxLen:                defaultRecentListMaxLen,
		recentStatsTTL:                  defaultRecentStatsTTL,
		recoveryRecentSpins:             defaultRecoveryRecentSpins,
		recoveryRecentRtpUpperLimit:     defaultRecoveryRecentRtpUpperLimit,
		recoveryRecentCvUpperLimit:      defaultRecoveryRecentCvUpperLimit,
		recoveryMaxSpins:                defaultRecoveryMaxSpins,
		recoveryExitRtp:                 defaultRecoveryExitRtp,
		recoveryExitMedianBetMultiplier: defaultRecoveryExitMedianBetMultiplier,
		recoveryActiveTTL:               defaultRecoveryActiveTTL,
		recoveryCooldownTTL:             defaultRecoveryCooldownTTL,
		recoverySelectedRtpMode:         defaultRecoverySelectedRtpMode,

		newcomerMaxMonthlySpins: defaultNewcomerMaxMonthlySpins,
		newcomerTargetMinSpins:  defaultNewcomerTargetMinSpins,
		newcomerTargetMaxSpins:  defaultNewcomerTargetMaxSpins,
		newcomerFixedRtpWeight:  defaultNewcomerFixedRtpWeight,
		newcomerSelectedRtpMode: defaultNewcomerSelectedRtpMode,
		newcomerActiveTTL:       defaultNewcomerActiveTTL,
		newcomerCooldownTTL:     defaultNewcomerCooldownTTL,

		coldRecentSpins:         defaultColdRecentSpins,
		coldRecentRtpLowerLimit: defaultColdRecentRtpLowerLimit,
		coldMaxSpins:            defaultColdMaxSpins,
		coldSelectedRtpMode:     defaultColdSelectedRtpMode,
		coldExitRtp:             defaultColdExitRtp,
		coldActiveTTL:           defaultColdActiveTTL,
		coldCooldownTTL:         defaultColdCooldownTTL,
	}
}

// WithRecentListMaxLen 设置最近普通投注列表最大长度。
func WithRecentListMaxLen(maxLen int64) Option {
	return func(s *Config) {
		if maxLen > 0 {
			s.recentListMaxLen = maxLen
		}
	}
}

// WithRecoveryRecentSpins 设置触发 recovery 所需最近局数。
func WithRecoveryRecentSpins(spins int64) Option {
	return func(s *Config) {
		if spins > 0 {
			s.recoveryRecentSpins = spins
		}
	}
}

// WithRecoveryRecentRtpUpperLimit 设置触发 recovery 的最近局数 RTP 上限。
func WithRecoveryRecentRtpUpperLimit(limit float64) Option {
	return func(s *Config) {
		if limit > 0 {
			s.recoveryRecentRtpUpperLimit = limit
		}
	}
}

// WithRecoveryRecentCvUpperLimit 设置触发 recovery 的最近局数下注 CV 上限。
func WithRecoveryRecentCvUpperLimit(limit float64) Option {
	return func(s *Config) {
		if limit > 0 {
			s.recoveryRecentCvUpperLimit = limit
		}
	}
}

// WithRecentStatsTTL 设置最近局数 RTP 缓存过期时间。
func WithRecentStatsTTL(ttl time.Duration) Option {
	return func(s *Config) {
		if ttl > 0 {
			s.recentStatsTTL = ttl
		}
	}
}

// WithRecoveryMaxSpins 设置 recovery 策略最多执行局数。
func WithRecoveryMaxSpins(spins int64) Option {
	return func(s *Config) {
		if spins > 0 {
			s.recoveryMaxSpins = spins
		}
	}
}

// WithRecoveryExitRtp 设置退出 recovery 的总 RTP 下限。
func WithRecoveryExitRtp(rtp float64) Option {
	return func(s *Config) {
		if rtp > 0 {
			s.recoveryExitRtp = rtp
		}
	}
}

// WithRecoveryExitMedianBetMultiplier 设置退出 recovery 的当前下注 / 触发时中位下注倍数。
func WithRecoveryExitMedianBetMultiplier(multiplier float64) Option {
	return func(s *Config) {
		if multiplier > 0 {
			s.recoveryExitMedianBetMultiplier = multiplier
		}
	}
}

// WithRecoveryActiveTTL 设置 recovery 事件最长持续时间。
func WithRecoveryActiveTTL(ttl time.Duration) Option {
	return func(s *Config) {
		if ttl > 0 {
			s.recoveryActiveTTL = ttl
		}
	}
}

// WithRecoveryCooldownTTL 设置 recovery 结束后冷却时间。
func WithRecoveryCooldownTTL(ttl time.Duration) Option {
	return func(s *Config) {
		if ttl > 0 {
			s.recoveryCooldownTTL = ttl
		}
	}
}

// WithRecoverySelectedRtpMode 设置 recovery 事件使用的 RTP mode。
func WithRecoverySelectedRtpMode(mode int) Option {
	return func(s *Config) {
		s.recoverySelectedRtpMode = mode
	}
}

// WithNewcomerMaxMonthlySpins 设置新手事件允许的近一个月普通投注最大局数 M。
func WithNewcomerMaxMonthlySpins(spins int64) Option {
	return func(s *Config) {
		if spins > 0 {
			s.newcomerMaxMonthlySpins = spins
		}
	}
}

// WithNewcomerTargetSpinsRange 设置新手事件随机判定局数 N 的范围。
func WithNewcomerTargetSpinsRange(minSpins, maxSpins int64) Option {
	return func(s *Config) {
		if minSpins > 0 && maxSpins >= minSpins {
			s.newcomerTargetMinSpins = minSpins
			s.newcomerTargetMaxSpins = maxSpins
		}
	}
}

// WithNewcomerFixedRtpWeight 设置新手事件使用选中 RTP mode 的权重。
func WithNewcomerFixedRtpWeight(weight float64) Option {
	return func(s *Config) {
		if weight >= 0 && weight <= 1 {
			s.newcomerFixedRtpWeight = weight
		}
	}
}

// WithNewcomerSelectedRtpMode 设置新手事件使用的 RTP mode。
func WithNewcomerSelectedRtpMode(mode int) Option {
	return func(s *Config) {
		s.newcomerSelectedRtpMode = mode
	}
}

// WithNewcomerActiveTTL 设置新手事件最长持续时间。
func WithNewcomerActiveTTL(ttl time.Duration) Option {
	return func(s *Config) {
		if ttl > 0 {
			s.newcomerActiveTTL = ttl
		}
	}
}

// WithNewcomerCooldownTTL 设置新手事件结束后冷却时间。
func WithNewcomerCooldownTTL(ttl time.Duration) Option {
	return func(s *Config) {
		if ttl > 0 {
			s.newcomerCooldownTTL = ttl
		}
	}
}

// WithColdRecentSpins 设置触发冷水事件所需最近普通投注局数。
func WithColdRecentSpins(spins int64) Option {
	return func(s *Config) {
		if spins > 0 {
			s.coldRecentSpins = spins
		}
	}
}

// WithColdRecentRtpLowerLimit 设置触发冷水事件的最近局数 RTP 下限。
func WithColdRecentRtpLowerLimit(rtp float64) Option {
	return func(s *Config) {
		if rtp > 0 {
			s.coldRecentRtpLowerLimit = rtp
		}
	}
}

// WithColdMaxSpins 设置冷水事件最多执行局数。
func WithColdMaxSpins(spins int64) Option {
	return func(s *Config) {
		if spins > 0 {
			s.coldMaxSpins = spins
		}
	}
}

// WithColdSelectedRtpMode 设置冷水事件使用的 RTP mode。
func WithColdSelectedRtpMode(mode int) Option {
	return func(s *Config) {
		s.coldSelectedRtpMode = mode
	}
}

// WithColdExitRtp 设置冷水事件退出的总 RTP 上限。
func WithColdExitRtp(rtp float64) Option {
	return func(s *Config) {
		if rtp > 0 {
			s.coldExitRtp = rtp
		}
	}
}

// WithColdActiveTTL 设置冷水事件最长持续时间。
func WithColdActiveTTL(ttl time.Duration) Option {
	return func(s *Config) {
		if ttl > 0 {
			s.coldActiveTTL = ttl
		}
	}
}

// WithColdCooldownTTL 设置冷水事件结束后冷却时间。
func WithColdCooldownTTL(ttl time.Duration) Option {
	return func(s *Config) {
		if ttl > 0 {
			s.coldCooldownTTL = ttl
		}
	}
}
