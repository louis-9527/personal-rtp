package session

import (
	"github.com/SinonetCPGame/game-kit/pb/enum"
	"github.com/redis/go-redis/v9"
)

const (
	FieldID           = "id"
	FieldBetCount     = "betCount"
	FieldBetAmount    = "betAmount"
	FieldPayoutAmount = "payoutAmount"
	FieldPrincipal    = "principal"
	FieldEventCount   = "eventCount"
	// 以下字段分别统计普通投注和 buy bonus 类型的投注。
	FieldNormalBetCount       = "normalBetCount"
	FieldNormalBetAmount      = "normalBetAmount"
	FieldNormalPayoutAmount   = "normalPayoutAmount"
	FieldBuyBonusBetCount     = "buyBonusBetCount"
	FieldBuyBonusBetAmount    = "buyBonusBetAmount"
	FieldBuyBonusPayoutAmount = "buyBonusPayoutAmount"

	StrategyBankruptcy = "bankruptcy" // StrategyBankruptcy 保留破产策略原有的事件计数字段，兼容线上已有会话。
	StrategyVeteran    = "veteran"
	StrategyRecovery   = "recovery"
)

// State 是 Redis 中保存的策略会话状态。
type State struct {
	ID                   string // 会话 ID。
	BetCount             int64  // 累计投注次数。
	BetAmount            int64  // 累计投注金额。
	PayoutAmount         int64  // 累计派彩金额。
	Principal            int64  // 本金（会话开始前的玩家余额）。
	NormalBetCount       int64  // 普通投注累计次数。
	NormalBetAmount      int64  // 普通投注累计金额。
	NormalPayoutAmount   int64  // 普通投注累计派彩金额。
	BuyBonusBetCount     int64  // Buy Bonus 累计次数。
	BuyBonusBetAmount    int64  // Buy Bonus 累计金额。
	BuyBonusPayoutAmount int64  // Buy Bonus 累计派彩金额。
}

// Settlement 是一笔已结算投注，金额使用游戏侧放大后的整数。
type Settlement struct {
	BetAmount    int64        // 本笔投注金额，使用游戏侧放大后的整数。
	PayoutAmount int64        // 本笔派彩金额，使用游戏侧放大后的整数。
	BetType      enum.BetType // 投注类型，用于确定是否计入策略会话。
	Balance      int64        // 本笔结算后的玩家余额。
}

// Manager 负责维护可被多个策略复用的玩家会话。
// 当前沿用破产策略的 Redis key，避免影响线上已有会话。
type Manager struct {
	redis  redis.UniversalClient
	config Config
}
