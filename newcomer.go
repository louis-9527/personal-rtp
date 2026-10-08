package personalrtp

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/SinonetCPGame/game-kit/log/logx"
	"github.com/SinonetCPGame/game-kit/rds"
	"github.com/SinonetCPGame/game-kit/userinfo"
)

// newcomerEvent 管理新手玩家事件。
type newcomerEvent struct {
	store *store
}

// newNewcomerEvent 创建新手玩家事件处理器。
func newNewcomerEvent(store *store) *newcomerEvent {
	return &newcomerEvent{store: store}
}

// GetMode 判断当前玩家是否处于或应进入新手玩家事件。
// 返回值依次为：是否新手事件、事件 ID、事件期间使用的 RTP mode、是否使用选中的 RTP mode、错误信息。
func (e *newcomerEvent) GetMode(ctx context.Context, user *userinfo.Info, roundNo string) (bool, string, int, bool, error) {
	event, handled, err := e.store.resolveModeEvent(ctx, user, EventTypeNewcomer, roundNo, e.finish)
	if err != nil || (handled && event == nil) {
		return false, "", 0, false, err
	}
	if event != nil {
		logx.Infow(ctx,
			"msg", "personal rtp newcomer execute",
			"event_id", event.EventId,
			"event_version", event.Version,
			"recent_bet_count", event.RecentBetCount,
			"target_bet_count", event.NewcomerTargetBetCount,
			"strategy_bet_count", event.StrategyBetCount,
			"newcomer_selected_rtp_mode", event.NewcomerSelectedRtpMode,
			"use_selected_rtp", event.UseSelectedRtp,
			"user_info", user,
			"round_no", roundNo,
		)
		return true, event.EventId, event.NewcomerSelectedRtpMode, event.UseSelectedRtp, nil
	}

	event, started, err := e.tryStart(ctx, user, roundNo)
	if err != nil || !started {
		return false, "", 0, false, err
	}
	return true, event.EventId, event.NewcomerSelectedRtpMode, event.UseSelectedRtp, nil
}

// Advance 在新手玩家事件存在时推进策略进度，并在普通投注超过 N 局时结束事件。
// 仅应在普通投注结算后由业务侧调用。
func (e *newcomerEvent) Advance(ctx context.Context, user *userinfo.Info, betAmount, payoutAmount int64, roundNo string) error {
	event, err := e.store.resolveAdvanceEvent(ctx, user, EventTypeNewcomer, roundNo, e.finish)
	if err != nil || event == nil {
		return err
	}

	advanceEventProgress(event, betAmount, payoutAmount)

	logx.Infow(ctx,
		"msg", "personal rtp newcomer progress",
		"event_id", event.EventId,
		"event_version", event.Version,
		"recent_bet_count", event.RecentBetCount,
		"target_bet_count", event.NewcomerTargetBetCount,
		"strategy_bet_count", event.StrategyBetCount,
		"newcomer_selected_rtp_mode", event.NewcomerSelectedRtpMode,
		"use_selected_rtp", event.UseSelectedRtp,
		"user_info", user,
		"round_no", roundNo,
	)

	if event.StrategyBetCount >= newcomerRemainSpins(event) {
		return e.finish(ctx, user, event, "spins_limit", roundNo)
	}
	return e.store.updateActiveEvent(ctx, user.Uid, event)
}

// tryStart 在满足新手玩家条件时创建新手事件。
func (e *newcomerEvent) tryStart(ctx context.Context, user *userinfo.Info, roundNo string) (*Event, bool, error) {
	uid := user.Uid
	cooling, err := e.store.redis.Exists(ctx, rds.GetPersonalRtpNewcomerCooldownKey(uid)).Result()
	if err != nil {
		return nil, false, err
	}
	if cooling > 0 {
		return nil, false, nil
	}

	stats, exists, err := LoadRecentStats(ctx, e.store.redis, user.Uid, e.store.config.newcomerMaxMonthlySpins)
	if err != nil {
		return nil, false, err
	}
	var recentBetCount int64
	if exists && stats != nil {
		recentBetCount = stats.BetCount
	}
	if recentBetCount > e.store.config.newcomerMaxMonthlySpins {
		return nil, false, nil
	}

	targetBetCount := e.randomTargetSpins()
	if recentBetCount >= targetBetCount {
		return nil, false, nil
	}

	useSelectedRtp := rand.Float64() < e.store.config.newcomerFixedRtpWeight
	now := time.Now()
	eventId := newEventID(EventTypeNewcomer)
	if useSelectedRtp {
		eventId = "T" + eventId
	} else {
		eventId = "F" + eventId
	}
	event := &Event{
		EventId:                 eventId,
		Type:                    EventTypeNewcomer,
		Version:                 EventVersionLatest,
		Uid:                     uid,
		RecentBetCount:          recentBetCount,
		NewcomerTargetBetCount:  targetBetCount,
		NewcomerSelectedRtpMode: e.store.config.newcomerSelectedRtpMode,
		UseSelectedRtp:          useSelectedRtp,
		StartedAt:               now,
		ExpiresAt:               now.Add(e.store.config.newcomerActiveTTL),
		LastUpdatedAt:           now,
	}

	event, started, err := e.store.startEvent(ctx, uid, event, e.store.config.newcomerActiveTTL)
	if err != nil || !started {
		return event, started, err
	}

	logx.Infow(ctx,
		"msg", "personal rtp newcomer start",
		"event_id", event.EventId,
		"event_version", event.Version,
		"recent_bet_count", event.RecentBetCount,
		"target_bet_count", event.NewcomerTargetBetCount,
		"newcomer_selected_rtp_mode", event.NewcomerSelectedRtpMode,
		"use_selected_rtp", event.UseSelectedRtp,
		"expires_at", event.ExpiresAt.Format(time.DateTime),
		"user_info", user,
		"round_no", roundNo,
	)
	return event, true, nil
}

// finish 结束新手玩家事件；非过期退出时写入冷却。
func (e *newcomerEvent) finish(ctx context.Context, user *userinfo.Info, event *Event, reason, roundNo string) error {
	if event == nil {
		return nil
	}
	now := time.Now()
	var ttl time.Duration
	if reason != "expired" {
		ttl = e.store.config.newcomerCooldownTTL
	}
	if err := e.store.clearActiveEvent(ctx, user.Uid, rds.GetPersonalRtpNewcomerCooldownKey(user.Uid), event.EventId, ttl); err != nil {
		return err
	}

	logx.Infow(ctx,
		"msg", "personal rtp newcomer finish",
		"event_id", event.EventId,
		"event_version", event.Version,
		"reason", reason,
		"recent_bet_count", event.RecentBetCount,
		"target_bet_count", event.NewcomerTargetBetCount,
		"strategy_bet_count", event.StrategyBetCount,
		"newcomer_selected_rtp_mode", event.NewcomerSelectedRtpMode,
		"use_selected_rtp", event.UseSelectedRtp,
		"started_at", event.StartedAt.Format(time.DateTime),
		"ended_at", now.Format(time.DateTime),
		"user_info", user,
		"round_no", roundNo,
	)
	return nil
}

// randomTargetSpins 随机生成新手事件判定局数 N。
func (e *newcomerEvent) randomTargetSpins() int64 {
	if e.store.config.newcomerTargetMaxSpins <= e.store.config.newcomerTargetMinSpins {
		return e.store.config.newcomerTargetMinSpins
	}
	return e.store.config.newcomerTargetMinSpins + rand.Int64N(e.store.config.newcomerTargetMaxSpins-e.store.config.newcomerTargetMinSpins+1)
}

// newcomerRemainSpins 计算新手事件剩余干预局数。
func newcomerRemainSpins(event *Event) int64 {
	if event == nil {
		return 0
	}
	remain := event.NewcomerTargetBetCount - event.RecentBetCount
	if remain < 0 {
		return 0
	}
	return remain
}
