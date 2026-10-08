package recovery

import (
	"context"
	"fmt"
	"math"
	"math/rand"

	"github.com/SinonetCPGame/game-kit/domain/entity"
	personalrtp "github.com/SinonetCPGame/game-kit/domain/service/personal-rtp"
	"github.com/SinonetCPGame/game-kit/domain/service/personal-rtp/session"
	"github.com/SinonetCPGame/game-kit/domain/service/predefined-result/v5"
	"github.com/SinonetCPGame/game-kit/log/logx"
	"github.com/SinonetCPGame/game-kit/pb/enum"
	"github.com/SinonetCPGame/game-kit/userinfo"
	"github.com/redis/go-redis/v9"
)

// New 创建补偿策略管理器。
func New[R entity.IResult](redisClient redis.UniversalClient, sessionManager *session.Manager,
	predefinedLogic *predefined.PredefinedLogic[R], opts ...Option) (*Manager[R], error) {
	if redisClient == nil {
		return nil, fmt.Errorf("recovery redis client is nil")
	}
	if sessionManager == nil {
		return nil, fmt.Errorf("recovery session manager is nil")
	}
	if predefinedLogic == nil {
		return nil, fmt.Errorf("recovery predefined logic is nil")
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
			return nil, fmt.Errorf("recovery validator result type does not match predefined logic")
		}
		c.Validator = validator
	}
	return &Manager[R]{
		redis:           redisClient,
		session:         sessionManager,
		config:          c,
		random:          rand.Float64,
		predefinedLogic: predefinedLogic,
	}, nil
}

// TryTrigger 应在普通投注出结果前调用；命中后返回业务方可直接使用的预制结果。
func (m *Manager[R]) TryTrigger(ctx context.Context, user *userinfo.Info, r *TriggerRequest) (TriggerResult[R], error) {
	if r == nil || !r.StrategySwitches.RecoveryEnabled {
		return TriggerResult[R]{}, nil
	}
	if r.BetAmount <= 0 {
		return TriggerResult[R]{}, nil
	}

	sessionState, err := m.session.Get(ctx, user.Uid)
	if err != nil {
		return TriggerResult[R]{}, err
	}
	if sessionState.ID == "" || sessionState.NormalBetCount < m.config.MinSessionBets {
		return TriggerResult[R]{}, nil
	}
	if sessionState.NormalBetAmount <= 0 {
		return TriggerResult[R]{}, nil
	}
	sessionRtp := float64(sessionState.NormalPayoutAmount) / float64(sessionState.NormalBetAmount)
	if sessionRtp > m.config.SessionRtpLimit {
		return TriggerResult[R]{}, nil
	}
	recentStats, exists, err := personalrtp.LoadRecentStats(ctx, m.redis, user.Uid, int64(m.config.RecentBets))
	if err != nil {
		return TriggerResult[R]{}, err
	}
	if !exists || recentStats.BetCount < int64(m.config.RecentBets) {
		return TriggerResult[R]{}, nil
	}
	if recentStats.BetAmount <= 0 {
		return TriggerResult[R]{}, nil
	}
	recentRtp := float64(recentStats.PayoutAmount) / float64(recentStats.BetAmount)
	if recentRtp > m.config.RecentRtpLimit {
		return TriggerResult[R]{}, nil
	}
	recentLoss := recentStats.BetAmount - recentStats.PayoutAmount
	if recentLoss <= 0 {
		return TriggerResult[R]{}, nil
	}
	randomValue := m.random()
	logx.Infow(ctx,
		"msg", "recovery try trigger",
		"session_id", sessionState.ID,
		"session_rtp", sessionRtp,
		"recent_bet_count", recentStats.BetCount,
		"recent_rtp", recentRtp,
		"recent_rtp_limit", m.config.RecentRtpLimit,
		"recent_loss_amount", recentLoss,
		"random_value", randomValue,
		"trigger_probability", m.config.TriggerProbability,
		"user_info", user,
		"round_no", r.RoundNo,
	)
	if randomValue >= m.config.TriggerProbability {
		return TriggerResult[R]{}, nil
	}

	recoveryRate := m.config.RecoveryMinRate +
		m.random()*(m.config.RecoveryMaxRate-m.config.RecoveryMinRate)
	theoreticalRecovery := int64(math.Floor(float64(recentLoss) * recoveryRate))
	payoutMultiplier := float64(theoreticalRecovery)/float64(r.BetAmount) + 1
	if payoutMultiplier <= m.config.PayoutMultiplierMin || payoutMultiplier > m.config.PayoutMultiplierMax {
		return TriggerResult[R]{}, nil
	}

	result, err := m.predefinedLogic.RandomResultByFloorOdds(ctx,
		m.config.ResultTypes,
		payoutMultiplier,
		m.config.Validator,
		&predefined.RecentResultParam{
			UserInfo: user,
			BetType:  enum.BetType_BT_NORMAL,
		})
	if err != nil {
		return TriggerResult[R]{}, err
	}

	// TryTrigger 在当前普通投注结算前执行，因此记录当前局序号时需要加 1。
	triggerCount, err := m.session.TryRecordTrigger(ctx, user.Uid, session.StrategyRecovery, sessionState.NormalBetCount+1,
		m.config.MaxSessionTriggers, m.config.MinTriggerInterval)
	if err != nil {
		return TriggerResult[R]{}, err
	}
	if triggerCount == 0 {
		return TriggerResult[R]{}, nil
	}

	eventID := fmt.Sprintf("R%d-%s", triggerCount, sessionState.ID)
	logx.Infow(ctx,
		"msg", "recovery triggered",
		"event_id", eventID,
		"normal_bet_count", sessionState.NormalBetCount,
		"session_rtp", sessionRtp,
		"recovery_recent_bet_count", recentStats.BetCount,
		"recent_loss_amount", recentLoss,
		"recovery_rate", recoveryRate,
		"theoretical_recovery_amount", theoreticalRecovery,
		"payout_multiplier", payoutMultiplier,
		"result_id", result.Id,
		"result_odds", result.Odds,
		"result_source_id", result.SourceId,
		"user_info", user,
		"round_no", r.RoundNo,
	)
	return TriggerResult[R]{
		Triggered: true,
		EventID:   eventID,
		Result:    result,
	}, nil
}
