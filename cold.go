package personalrtp

import (
	"context"
	"time"

	"github.com/SinonetCPGame/game-kit/log/logx"
	"github.com/SinonetCPGame/game-kit/rds"
	"github.com/SinonetCPGame/game-kit/userinfo"
)

// coldEvent 管理冷水事件。
type coldEvent struct {
	store *store
}

// newColdEvent 创建冷水事件处理器。
func newColdEvent(store *store) *coldEvent {
	return &coldEvent{store: store}
}

// GetMode 判断当前玩家是否处于或应进入冷水事件。
// 返回值依次为：是否冷水事件、事件 ID、冷水期 RTP mode、是否使用选中的 RTP mode、错误信息。
func (e *coldEvent) GetMode(ctx context.Context, user *userinfo.Info, roundNo string, switches StrategySwitches) (bool, string, int, bool, error) {
	event, handled, err := e.store.resolveModeEvent(ctx, user, EventTypeCold, roundNo, e.finish)
	if err != nil || (handled && event == nil) {
		return false, "", 0, false, err
	}
	if event != nil {
		recentRtp, strategyRtp, totalRtp := calcEventRtpDetail(event)
		logx.Infow(ctx,
			"msg", "personal rtp cold execute",
			"event_id", event.EventId,
			"event_version", event.Version,
			"strategy_bet_count", event.StrategyBetCount,
			"recent_rtp", recentRtp,
			"strategy_rtp", strategyRtp,
			"total_rtp", totalRtp,
			"cold_selected_rtp_mode", event.ColdSelectedRtpMode,
			"user_info", user,
			"round_no", roundNo,
		)
		return true, event.EventId, event.ColdSelectedRtpMode, true, nil
	}

	event, started, err := e.tryStart(ctx, user, roundNo, switches)
	if err != nil || !started {
		return false, "", 0, false, err
	}
	return true, event.EventId, event.ColdSelectedRtpMode, true, nil
}

// Advance 在冷水事件存在时推进策略进度，并在满足退出条件时结束事件。
// 仅应在普通投注结算后由业务侧调用。
func (e *coldEvent) Advance(ctx context.Context, user *userinfo.Info, betAmount, payoutAmount int64, roundNo string) error {
	event, err := e.store.resolveAdvanceEvent(ctx, user, EventTypeCold, roundNo, e.finish)
	if err != nil || event == nil {
		return err
	}

	advanceEventProgress(event, betAmount, payoutAmount)
	recentRtp, strategyRtp, totalRtp := calcEventRtpDetail(event)

	logx.Infow(ctx,
		"msg", "personal rtp cold progress",
		"event_id", event.EventId,
		"event_version", event.Version,
		"strategy_bet_count", event.StrategyBetCount,
		"strategy_bet_amount", event.StrategyBetAmount,
		"strategy_payout_amount", event.StrategyPayoutAmount,
		"recent_rtp", recentRtp,
		"strategy_rtp", strategyRtp,
		"total_rtp", totalRtp,
		"cold_selected_rtp_mode", event.ColdSelectedRtpMode,
		"user_info", user,
		"round_no", roundNo,
	)

	maxSpins := event.ColdMaxSpins
	if maxSpins <= 0 {
		// 兼容字段上线前已写入 Redis 的冷水事件：旧事件未固化档位最大局数。
		maxSpins = e.store.config.coldMaxSpins
	}
	if event.StrategyBetCount >= maxSpins {
		return e.finish(ctx, user, event, "spins_limit", roundNo)
	}
	if totalRtp <= e.store.config.coldExitRtp {
		return e.finish(ctx, user, event, "rtp_cooled", roundNo)
	}
	return e.store.updateActiveEvent(ctx, user.Uid, event)
}

// tryStart 在最近 RTP 高于当前档位阈值且不在冷却期时创建冷水事件。
func (e *coldEvent) tryStart(ctx context.Context, user *userinfo.Info, roundNo string, switches StrategySwitches) (*Event, bool, error) {
	uid := user.Uid
	cooling, err := e.store.redis.Exists(ctx, rds.GetPersonalRtpColdCooldownKey(uid)).Result()
	if err != nil {
		return nil, false, err
	}
	if cooling > 0 {
		return nil, false, nil
	}

	recentStats, exists, err := LoadRecentStats(ctx, e.store.redis, user.Uid, e.store.config.coldRecentSpins)
	if err != nil {
		return nil, false, err
	}
	if !exists || recentStats == nil || recentStats.BetCount < e.store.config.coldRecentSpins {
		return nil, false, nil
	}
	recentRtp := calcRtp(recentStats.BetAmount, recentStats.PayoutAmount)
	if recentRtp <= switches.ColdRecentRtpLowerLimit {
		return nil, false, nil
	}

	now := time.Now()
	event := &Event{
		EventId:             newEventID(EventTypeCold),
		Type:                EventTypeCold,
		Version:             EventVersionLatest,
		Uid:                 uid,
		RecentBetCount:      recentStats.BetCount,
		RecentBetAmount:     recentStats.BetAmount,
		RecentPayoutAmount:  recentStats.PayoutAmount,
		RecentBetAmounts:    append([]float64(nil), recentStats.BetAmounts...),
		ColdSelectedRtpMode: e.store.config.coldSelectedRtpMode,
		ColdMaxSpins:        switches.ColdMaxSpins,
		UseSelectedRtp:      true,
		StartedAt:           now,
		ExpiresAt:           now.Add(e.store.config.coldActiveTTL),
		LastUpdatedAt:       now,
	}

	event, started, err := e.store.startEvent(ctx, uid, event, e.store.config.coldActiveTTL)
	if err != nil || !started {
		return event, started, err
	}

	logx.Infow(ctx,
		"msg", "personal rtp cold start",
		"event_id", event.EventId,
		"event_version", event.Version,
		"recent_bet_count", event.RecentBetCount,
		"recent_bet_amount", event.RecentBetAmount,
		"recent_payout_amount", event.RecentPayoutAmount,
		"recent_rtp", recentRtp,
		"cold_selected_rtp_mode", event.ColdSelectedRtpMode,
		"expires_at", event.ExpiresAt.Format(time.DateTime),
		"user_info", user,
		"round_no", roundNo,
	)
	return event, true, nil
}

// finish 结束冷水事件；非过期退出时写入冷却。
func (e *coldEvent) finish(ctx context.Context, user *userinfo.Info, event *Event, reason, roundNo string) error {
	if event == nil {
		return nil
	}
	now := time.Now()
	var ttl time.Duration
	if reason != "expired" {
		ttl = e.store.config.coldCooldownTTL
	}
	if err := e.store.clearActiveEvent(ctx, user.Uid, rds.GetPersonalRtpColdCooldownKey(user.Uid), event.EventId, ttl); err != nil {
		return err
	}

	recentRtp, strategyRtp, totalRtp := calcEventRtpDetail(event)
	logx.Infow(ctx,
		"msg", "personal rtp cold finish",
		"event_id", event.EventId,
		"event_version", event.Version,
		"reason", reason,
		"strategy_bet_count", event.StrategyBetCount,
		"strategy_bet_amount", event.StrategyBetAmount,
		"strategy_payout_amount", event.StrategyPayoutAmount,
		"recent_rtp", recentRtp,
		"strategy_rtp", strategyRtp,
		"total_rtp", totalRtp,
		"cold_selected_rtp_mode", event.ColdSelectedRtpMode,
		"started_at", event.StartedAt.Format(time.DateTime),
		"ended_at", now.Format(time.DateTime),
		"user_info", user,
		"round_no", roundNo,
	)
	return nil
}
