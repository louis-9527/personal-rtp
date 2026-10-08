package veteran

import (
	"context"
	"fmt"
	"math"
	"math/rand"

	"github.com/SinonetCPGame/game-kit/domain/entity"
	betstats "github.com/SinonetCPGame/game-kit/domain/service/bet-stats"
	"github.com/SinonetCPGame/game-kit/domain/service/personal-rtp/session"
	"github.com/SinonetCPGame/game-kit/domain/service/predefined-result/v5"
	"github.com/SinonetCPGame/game-kit/log/logx"
	"github.com/SinonetCPGame/game-kit/pb/enum"
	"github.com/SinonetCPGame/game-kit/userinfo"
	"github.com/redis/go-redis/v9"
)

// New 创建老玩家策略管理器。
func New[R entity.IResult](redisClient redis.UniversalClient, sessionManager *session.Manager, predefinedLogic *predefined.PredefinedLogic[R], opts ...Option) (*Manager[R], error) {
	if sessionManager == nil {
		return nil, fmt.Errorf("veteran session manager is nil")
	}
	if predefinedLogic == nil {
		return nil, fmt.Errorf("veteran predefined logic is nil")
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
			return nil, fmt.Errorf("veteran validator result type does not match predefined logic")
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

// TryTrigger 应在普通投注出结果前调用；命中后返回业务方可直接使用的预制结果。
func (m *Manager[R]) TryTrigger(ctx context.Context, user *userinfo.Info, r *TriggerRequest) (TriggerResult[R], error) {
	if !r.StrategySwitches.VeteranEnabled {
		return TriggerResult[R]{}, nil
	}
	if r.BetAmount <= 0 {
		return TriggerResult[R]{}, nil
	}
	sessionState, err := m.session.Get(ctx, user.Uid)
	if err != nil {
		return TriggerResult[R]{}, err
	}
	triggerCount, err := m.session.EventCount(ctx, user.Uid, session.StrategyVeteran)
	if err != nil {
		return TriggerResult[R]{}, err
	}
	// 仅允许成功触发前的次数小于上限，确保本次成功后总次数仍不超过上限。
	if sessionState.ID == "" || triggerCount >= m.config.MaxSessionTriggers {
		return TriggerResult[R]{}, nil
	}

	stats, err := betstats.New(m.redis, user.Uid).GetAll(ctx)
	if err != nil {
		return TriggerResult[R]{}, err
	}
	if stats.BetCount < m.config.MinHistoricalBets || stats.BetAmount <= 0 {
		return TriggerResult[R]{}, nil
	}
	historicalRtp := float64(stats.PayoutAmount) / float64(stats.BetAmount)
	if historicalRtp > m.config.HistoricalRtpLimit {
		return TriggerResult[R]{}, nil
	}
	historicalLoss := stats.BetAmount - stats.PayoutAmount
	if historicalLoss <= 0 {
		return TriggerResult[R]{}, nil
	}
	if float64(r.BetAmount)*m.config.BetPayoutMultiplier >= float64(historicalLoss)*m.config.PayoutLossRate {
		return TriggerResult[R]{}, nil
	}

	triggerProbability := m.probabilityForHistoricalRtp(historicalRtp)
	eventId := fmt.Sprintf("V%d-%s", triggerCount+1, sessionState.ID)
	randomValue := rand.Float64()
	logx.Infow(ctx,
		"msg", "veteran try trigger",
		"random_value", randomValue,
		"trigger_probability", triggerProbability,
		"event_id", eventId,
		"historical_rtp", historicalRtp,
		"historical_bet_count", stats.BetCount,
		"user_info", user,
		"round_no", r.RoundNo,
	)
	if randomValue >= triggerProbability {
		return TriggerResult[R]{}, nil
	}

	result, err := m.predefinedLogic.RandomResultByOddsRange(ctx,
		m.config.ResultTypes,
		m.config.OddsRange,
		m.config.Validator,
		&predefined.RecentResultParam{
			UserInfo: user,
			BetType:  enum.BetType_BT_NORMAL,
		})
	if err != nil {
		return TriggerResult[R]{}, err
	}

	if err = m.session.IncrementEvent(ctx, user.Uid, session.StrategyVeteran); err != nil {
		return TriggerResult[R]{}, err
	}
	logx.Infow(ctx,
		"msg", "veteran triggered",
		"event_id", eventId,
		"result_id", result.Id,
		"result_odds", result.Odds,
		"result_source_id", result.SourceId,
		"user_info", user,
		"round_no", r.RoundNo,
	)
	return TriggerResult[R]{Triggered: true, EventID: eventId, Result: result}, nil
}

// probabilityForHistoricalRtp 计算触发概率（历史rtp每降低整整 1% 增加 1%概率）。
func (m *Manager[R]) probabilityForHistoricalRtp(rtp float64) float64 {
	// + 1e-9 是为规避浮点精度：0.85 - 0.84 在二进制浮点中可能得到 0.009999999...，直接 Floor 会误判成 0
	decrements := math.Floor((m.config.HistoricalRtpLimit-rtp)*100 + 1e-9)
	probability := m.config.MinimumProbability + decrements*0.01
	return min(m.config.MaximumProbability, max(m.config.MinimumProbability, probability))
}
