package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/SinonetCPGame/game-kit/log/logx"
	"github.com/SinonetCPGame/game-kit/pb/enum"
	"github.com/SinonetCPGame/game-kit/rds"
	"github.com/SinonetCPGame/game-kit/userinfo"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
)

// New 创建策略会话管理器。
func New(redisClient redis.UniversalClient, opts ...Option) *Manager {
	config := DefaultConfig()
	for _, opt := range opts {
		opt(&config)
	}
	return &Manager{redis: redisClient, config: config}
}

// RecordSettlement 刷新玩家余额；普通投注、buy bonus 和 super bonus 会累计至会话。
// 会话不存在或已过期时自动创建新会话。
func (m *Manager) RecordSettlement(ctx context.Context, user *userinfo.Info, settlement *Settlement) (State, error) {
	if err := m.SetBalance(ctx, user.Uid, settlement.Balance); err != nil {
		return State{}, err
	}
	if (settlement.BetType != enum.BetType_BT_NORMAL &&
		settlement.BetType != enum.BetType_BT_BUY_BONUS &&
		settlement.BetType != enum.BetType_BT_SUPER_BONUS &&
		settlement.BetType != enum.BetType_BT_GOLD_BOOST) ||
		settlement.BetAmount <= 0 {
		return State{}, nil
	}

	state, err := m.Get(ctx, user.Uid)
	if err != nil {
		return State{}, err
	}

	key := rds.GetPersonalRtpSessionKey(user.Uid)
	p := m.redis.Pipeline()

	created := state.ID == ""
	var principal int64
	if created {
		if err = m.Clear(ctx, user.Uid); err != nil {
			return State{}, err
		}
		state.ID = uuid.NewString()
		principal = settlement.Balance + settlement.BetAmount - settlement.PayoutAmount
		p.HSet(ctx, key, FieldPrincipal, principal)
	}

	p.HSet(ctx, key, FieldID, state.ID)
	p.HIncrBy(ctx, key, FieldBetCount, 1)
	p.HIncrBy(ctx, key, FieldBetAmount, settlement.BetAmount)
	p.HIncrBy(ctx, key, FieldPayoutAmount, settlement.PayoutAmount)
	switch settlement.BetType {
	case enum.BetType_BT_NORMAL:
		p.HIncrBy(ctx, key, FieldNormalBetCount, 1)
		p.HIncrBy(ctx, key, FieldNormalBetAmount, settlement.BetAmount)
		p.HIncrBy(ctx, key, FieldNormalPayoutAmount, settlement.PayoutAmount)
	case enum.BetType_BT_BUY_BONUS, enum.BetType_BT_SUPER_BONUS, enum.BetType_BT_GOLD_BOOST:
		p.HIncrBy(ctx, key, FieldBuyBonusBetCount, 1)
		p.HIncrBy(ctx, key, FieldBuyBonusBetAmount, settlement.BetAmount)
		p.HIncrBy(ctx, key, FieldBuyBonusPayoutAmount, settlement.PayoutAmount)
	}
	p.Expire(ctx, key, m.config.IdleTTL)
	if _, err = p.Exec(ctx); err != nil {
		return State{}, err
	}
	if created {
		logx.Infow(ctx,
			"msg", "strategy session start",
			"session_id", state.ID,
			"principal", principal,
			"user_info", user,
		)
	}
	return state, nil
}

// Get 返回当前策略会话状态；会话不存在时返回零值状态。
func (m *Manager) Get(ctx context.Context, uid string) (State, error) {
	values, err := m.redis.HMGet(ctx, rds.GetPersonalRtpSessionKey(uid),
		FieldID, FieldBetCount, FieldBetAmount, FieldPayoutAmount, FieldPrincipal,
		FieldNormalBetCount, FieldNormalBetAmount, FieldNormalPayoutAmount,
		FieldBuyBonusBetCount, FieldBuyBonusBetAmount, FieldBuyBonusPayoutAmount,
	).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return State{}, nil
		}
		return State{}, err
	}
	if len(values) != 11 {
		return State{}, fmt.Errorf("unexpected session field count: %d", len(values))
	}
	return State{
		ID:                   cast.ToString(values[0]),
		BetCount:             cast.ToInt64(values[1]),
		BetAmount:            cast.ToInt64(values[2]),
		PayoutAmount:         cast.ToInt64(values[3]),
		Principal:            cast.ToInt64(values[4]),
		NormalBetCount:       cast.ToInt64(values[5]),
		NormalBetAmount:      cast.ToInt64(values[6]),
		NormalPayoutAmount:   cast.ToInt64(values[7]),
		BuyBonusBetCount:     cast.ToInt64(values[8]),
		BuyBonusBetAmount:    cast.ToInt64(values[9]),
		BuyBonusPayoutAmount: cast.ToInt64(values[10]),
	}, nil
}

// Clear 删除当前策略会话状态。
func (m *Manager) Clear(ctx context.Context, uid string) error {
	return m.redis.Del(ctx, rds.GetPersonalRtpSessionKey(uid)).Err()
}

// IncrementEvent 递增指定策略在当前会话内的成功触发次数。
func (m *Manager) IncrementEvent(ctx context.Context, uid, strategy string) error {
	return m.redis.HIncrBy(ctx, rds.GetPersonalRtpSessionKey(uid), eventCountField(strategy), 1).Err()
}

// EventCount 返回指定策略在当前会话内的成功触发次数。
func (m *Manager) EventCount(ctx context.Context, uid, strategy string) (int64, error) {
	count, err := m.redis.HGet(ctx, rds.GetPersonalRtpSessionKey(uid), eventCountField(strategy)).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return count, err
}

// TryRecordTrigger 原子校验并记录策略触发，返回本次成功触发后的会话内次数。
// 返回 0 表示未通过次数或间隔校验。
func (m *Manager) TryRecordTrigger(ctx context.Context, uid, strategy string, currentBetCount, maxCount, minInterval int64) (int64, error) {
	if strategy == "" || currentBetCount < 0 || maxCount <= 0 || minInterval < 0 {
		return 0, fmt.Errorf("invalid trigger record arguments")
	}
	result, err := m.redis.Eval(ctx, `
local countField = ARGV[1]
local lastBetCountField = ARGV[2]
local currentBetCount = tonumber(ARGV[3])
local maxCount = tonumber(ARGV[4])
local minInterval = tonumber(ARGV[5])
if redis.call("EXISTS", KEYS[1]) == 0 then
	return 0
end
local count = tonumber(redis.call("HGET", KEYS[1], countField) or "0")
if count >= maxCount then
	return 0
end
local lastBetCount = redis.call("HGET", KEYS[1], lastBetCountField)
if lastBetCount and currentBetCount - tonumber(lastBetCount) <= minInterval then
	return 0
end
local newCount = redis.call("HINCRBY", KEYS[1], countField, 1)
redis.call("HSET", KEYS[1], lastBetCountField, currentBetCount)
return newCount
`, []string{rds.GetPersonalRtpSessionKey(uid)},
		eventCountField(strategy), lastTriggerBetCountField(strategy), currentBetCount, maxCount, minInterval,
	).Int64()
	if err != nil {
		return 0, err
	}
	return result, nil
}

// SetBalance 保存玩家最近一次结算后的余额，并设置其有效期。
func (m *Manager) SetBalance(ctx context.Context, uid string, balance int64) error {
	return m.redis.Set(ctx, rds.GetPersonalRtpBalanceKey(uid), balance, m.config.IdleTTL).Err()
}

// Balance 返回玩家最近一次结算后的余额。
func (m *Manager) Balance(ctx context.Context, uid string) (int64, error) {
	return m.redis.Get(ctx, rds.GetPersonalRtpBalanceKey(uid)).Int64()
}

func eventCountField(strategy string) string {
	if strategy == StrategyBankruptcy {
		return FieldEventCount
	}
	return FieldEventCount + ":" + strategy
}

func lastTriggerBetCountField(strategy string) string {
	return "lastTriggerBetCount:" + strategy
}
