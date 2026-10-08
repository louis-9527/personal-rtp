package bankruptcy

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/SinonetCPGame/game-kit/domain/entity"
	"github.com/SinonetCPGame/game-kit/domain/service/personal-rtp/session"
	"github.com/SinonetCPGame/game-kit/domain/service/predefined-result/v5"
	"github.com/SinonetCPGame/game-kit/log/logx"
	"github.com/SinonetCPGame/game-kit/pb/enum"
	"github.com/SinonetCPGame/game-kit/rds"
	"github.com/SinonetCPGame/game-kit/userinfo"
	"github.com/redis/go-redis/v9"
)

// New 创建破产策略管理器。
func New[R entity.IResult](redisClient redis.UniversalClient, sessionManager *session.Manager,
	predefinedLogic *predefined.PredefinedLogic[R], opts ...Option) (*Manager[R], error) {
	if sessionManager == nil {
		return nil, fmt.Errorf("bankruptcy session manager is nil")
	}
	if predefinedLogic == nil {
		return nil, fmt.Errorf("bankruptcy predefined logic is nil")
	}
	c := defaultConfig[R]()
	for _, opt := range opts {
		if opt != nil {
			opt(&c.config)
		}
	}
	if c.validator != nil {
		validator, ok := c.validator.(predefined.ResultValidator[R])
		if !ok {
			return nil, fmt.Errorf("bankruptcy validator result type does not match predefined logic")
		}
		c.Validator = validator
	}
	return &Manager[R]{
		redis:           redisClient,
		session:         sessionManager,
		config:          c,
		predefinedLogic: predefinedLogic,
	}, nil
}

// TryTrigger 应在普通投注出结果前调用，触发后返回符合最大派彩倍数的预制结果。
func (m *Manager[R]) TryTrigger(ctx context.Context, user *userinfo.Info, r *TriggerRequest) (TriggerResult[R], error) {
	if r == nil || !r.StrategySwitches.BankruptcyEnabled {
		return TriggerResult[R]{}, nil
	}
	if r.BetAmount <= 0 || r.MaxPayoutAmount <= 0 {
		return TriggerResult[R]{}, nil
	}
	sessionState, err := m.session.Get(ctx, user.Uid)
	if err != nil {
		return TriggerResult[R]{}, err
	}
	if sessionState.BetCount < m.config.MinSessionBets || sessionState.BetAmount <= 0 {
		return TriggerResult[R]{}, nil
	}
	daily, err := m.redis.Get(ctx, rds.GetPersonalRtpBankruptDailyKey(user.Uid)).Int64()
	if err != nil && err != redis.Nil {
		return TriggerResult[R]{}, err
	}
	count, abb, err := m.abb(ctx, user.Uid)
	if err != nil {
		return TriggerResult[R]{}, err
	}
	stage, ok := m.stageForTriggerCount(daily)
	if count < m.config.MinNormalSpins || abb <= 0 || !ok {
		return TriggerResult[R]{}, nil
	}
	balance, err := m.session.Balance(ctx, user.Uid)
	if err == redis.Nil {
		return TriggerResult[R]{}, nil
	}
	if err != nil {
		return TriggerResult[R]{}, err
	}
	rrl := float64(balance) / abb
	sessionRtp := float64(sessionState.PayoutAmount) / float64(sessionState.BetAmount)
	var principalLossRate float64
	if sessionState.Principal > 0 {
		principalLossRate = float64(max(0, sessionState.BetAmount-sessionState.PayoutAmount)) / float64(sessionState.Principal)
	}
	betAbbMultiplier := float64(r.BetAmount) / abb
	if rrl < stage.RrlLowerLimit || rrl > stage.RrlUpperLimit ||
		(sessionRtp >= stage.SessionRtpUpperLimit && principalLossRate < stage.PrincipalLossRateLowerLimit) ||
		betAbbMultiplier < m.config.BetAbbLowerMultiplier || betAbbMultiplier > m.config.BetAbbUpperMultiplier {
		return TriggerResult[R]{}, nil
	}
	loss := sessionState.BetAmount - sessionState.PayoutAmount
	if loss <= 0 {
		return TriggerResult[R]{}, nil
	}
	// rand.Float64 返回 [0, 1)，因此该写法满足返还系数的左开右闭区间。
	refundRate := stage.RefundRateUpperLimit - rand.Float64()*(stage.RefundRateUpperLimit-stage.RefundRateLowerLimit)
	refund := int64(math.Floor(float64(loss) * refundRate))
	maxPayout := min(refund, r.MaxPayoutAmount)
	if maxPayout <= 0 {
		return TriggerResult[R]{}, nil
	}
	randomValue := rand.Float64()
	logx.Infow(ctx,
		"msg", "bankruptcy try trigger probability",
		"random_value", randomValue,
		"trigger_probability", stage.TriggerProbability,
		"user_info", user,
		"balance", balance,
		"round_no", r.RoundNo,
	)
	if randomValue >= stage.TriggerProbability {
		return TriggerResult[R]{}, nil
	}
	logx.Infow(ctx,
		"msg", "bankruptcy try trigger",
		"session_bet_count", sessionState.BetCount,
		"session_bet_amount", sessionState.BetAmount,
		"session_payout_amount", sessionState.PayoutAmount,
		"session_rtp", sessionRtp,
		"abb", abb,
		"rrl", rrl,
		"bet_abb_multiplier", betAbbMultiplier,
		"session_loss_amount", loss,
		"refund_rate", refundRate,
		"refund_amount", refund,
		"max_payout_amount", maxPayout,
		"user_info", user,
		"balance", balance,
		"round_no", r.RoundNo,
	)
	maxPayoutMultiple := float64(maxPayout)/float64(r.BetAmount) + 1
	result, err := m.predefinedLogic.RandomResultByFloorOdds(ctx,
		m.config.ResultTypes,
		maxPayoutMultiple,
		m.config.Validator,
		&predefined.RecentResultParam{
			UserInfo: user,
			BetType:  enum.BetType_BT_NORMAL,
		})
	if err != nil {
		return TriggerResult[R]{}, err
	}
	if err = m.recordTrigger(ctx, user.Uid); err != nil {
		return TriggerResult[R]{}, err
	}
	eventID := stage.EventID + "-" + sessionState.ID
	logx.Infow(ctx,
		"msg", "bankruptcy triggered",
		"event_id", eventID,
		"refund_rate", refundRate,
		"refund_amount", refund,
		"max_payout_amount", maxPayout,
		"max_payout_multiple", maxPayoutMultiple,
		"result_id", result.Id,
		"result_odds", result.Odds,
		"result_source_id", result.SourceId,
		"user_info", user,
		"balance", balance,
		"round_no", r.RoundNo,
	)
	return TriggerResult[R]{
		Triggered: true,
		EventID:   eventID,
		Result:    result,
	}, nil
}

// stageForTriggerCount 返回当前 24 小时窗口内下一次可触发的阶段。
func (m *Manager[R]) stageForTriggerCount(triggerCount int64) (Stage, bool) {
	if triggerCount < 0 || triggerCount >= int64(len(m.config.Stages)) {
		return Stage{}, false
	}
	return m.config.Stages[triggerCount], true
}

// recordTrigger 递增从首次触发开始计算的 24 小时触发计数。
// 会话切换不会影响该计数；TTL 仅在首次触发时写入，后续阶段不续期。
func (m *Manager[R]) recordTrigger(ctx context.Context, uid string) error {
	key := rds.GetPersonalRtpBankruptDailyKey(uid)
	created, err := m.redis.SetNX(ctx, key, 1, 24*time.Hour).Result()
	if err != nil || created {
		return err
	}
	return m.redis.Incr(ctx, key).Err()
}

// abb 从个人 RTP 最近普通投注列表计算近 MinNormalSpins 局的 ABB 中位数。
func (m *Manager[R]) abb(ctx context.Context, uid string) (int64, float64, error) {
	items, err := m.redis.LRange(ctx, rds.GetPersonalRtpRecentListKey(uid), 0, m.config.MinNormalSpins-1).Result()
	if err != nil || len(items) < int(m.config.MinNormalSpins) {
		return int64(len(items)), 0, err
	}
	bets := make([]int64, 0, len(items))
	for _, item := range items {
		var r struct {
			BetAmount int64 `json:"betAmount"`
		}
		if err = json.Unmarshal([]byte(item), &r); err != nil {
			return 0, 0, err
		}
		if r.BetAmount <= 0 {
			return 0, 0, nil
		}
		bets = append(bets, r.BetAmount)
	}
	sort.Slice(bets, func(i, j int) bool { return bets[i] < bets[j] })
	n := len(bets)
	v := float64(bets[n/2])
	if n%2 == 0 {
		v = float64(bets[n/2-1]+bets[n/2]) / 2
	}
	return int64(n), v, nil
}
