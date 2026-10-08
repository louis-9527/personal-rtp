package personalrtp

import (
	"context"
	"time"

	"github.com/SinonetCPGame/game-kit/userinfo"
	"github.com/redis/go-redis/v9"
)

// GetModeRequest 是业务侧统一查询个人 RTP 事件模式的入参。
type GetModeRequest struct {
	BetAmount        int64            `json:"betAmount"`        // 当前普通投注金额，recover 下注激增退出判断会使用。
	RoundNo          string           `json:"roundNo"`          // 当前局号，仅用于日志。
	StrategySwitches StrategySwitches `json:"strategySwitches"` // 业务方基于资金池缓存计算的单局策略开关。
}

// ModeResult 是业务侧统一消费的事件模式结果。
type ModeResult struct {
	InEvent        bool   `json:"inEvent"`        // 当前是否命中个人 RTP 事件。
	EventId        string `json:"eventId"`        // 当前事件 ID。
	EventType      string `json:"eventType"`      // 当前事件类型，主要用于日志和排查。
	RtpMode        int    `json:"rtpMode"`        // 选中的 RTP mode。
	UseSelectedRtp bool   `json:"useSelectedRtp"` // 是否使用选中的 RTP mode。
}

// EventManager 对外提供统一的个人 RTP 事件入口。
type EventManager struct {
	store    *store
	newcomer *newcomerEvent
	cold     *coldEvent
}

// modeGetter 封装一种事件的 GetMode 调用，便于按优先级尝试启动事件。
type modeGetter func(context.Context, *userinfo.Info, GetModeRequest) (ModeResult, bool, error)

// NewEventManager 创建统一事件管理器。
func NewEventManager(redisCli redis.UniversalClient, opts ...Option) *EventManager {
	store := newStore(redisCli, opts...)
	return &EventManager{
		store:    store,
		newcomer: newNewcomerEvent(store),
		cold:     newColdEvent(store),
	}
}

// RecentListLen 获取玩家最近普通投注列表长度。
func (m *EventManager) RecentListLen(ctx context.Context, user *userinfo.Info) (int64, error) {
	return m.store.recentListLen(ctx, user)
}

// InitRecentBets 用 Mongo 查询结果初始化最近普通投注列表。
// records 需按投注时间倒序排列（最新在前）。
func (m *EventManager) InitRecentBets(ctx context.Context, user *userinfo.Info, records []RecentBetRecord) error {
	return m.store.initRecentBets(ctx, user, records)
}

// AppendRecentBet 将一局普通投注追加到最近普通投注列表头部，并动态维护列表长度。
func (m *EventManager) AppendRecentBet(ctx context.Context, user *userinfo.Info, record RecentBetRecord) error {
	return m.store.appendRecentBet(ctx, user, record)
}

// GetMode 统一判断当前局是否应进入任一个人 RTP 事件。
func (m *EventManager) GetMode(ctx context.Context, user *userinfo.Info, req GetModeRequest) (ModeResult, error) {
	event, exists, err := m.store.getEvent(ctx, user.Uid)
	if err != nil {
		return ModeResult{}, err
	}
	if exists {
		// recovery 已改为独立会话补偿策略，遗留的旧 recovery 事件不再继续执行，也不应阻塞新手或冷水事件。
		if event.eventType() == EventTypeRecovery {
			if err = m.store.clearActiveEvent(ctx, user.Uid, "", "", 0); err != nil {
				return ModeResult{}, err
			}
			exists = false
		}
	}
	if exists {
		result, _, err := m.getModeByType(ctx, user, event.eventType(), req)
		return result, err
	}

	var getters []modeGetter
	if req.StrategySwitches.NewcomerEnabled {
		getters = append(getters, m.getNewcomerMode)
	}
	if req.StrategySwitches.ColdEnabled {
		getters = append(getters, m.getColdMode)
	}
	for _, getMode := range getters {
		if result, ok, err := getMode(ctx, user, req); err != nil || ok {
			return result, err
		}
	}
	return ModeResult{}, nil
}

// Advance 统一推进当前 active 的个人 RTP 事件。
func (m *EventManager) Advance(ctx context.Context, user *userinfo.Info, betAmount, payoutAmount int64, roundNo string) error {
	event, exists, err := m.store.getEvent(ctx, user.Uid)
	if err != nil || !exists {
		return err
	}

	switch event.eventType() {
	case EventTypeNewcomer:
		return m.newcomer.Advance(ctx, user, betAmount, payoutAmount, roundNo)
	case EventTypeCold:
		return m.cold.Advance(ctx, user, betAmount, payoutAmount, roundNo)
	default:
		return nil
	}
}

// GetActiveEventID 获取当前未过期的任一个人 RTP 事件 ID。
func (m *EventManager) GetActiveEventID(ctx context.Context, user *userinfo.Info) (string, error) {
	event, exists, err := m.store.getEvent(ctx, user.Uid)
	if err != nil || !exists {
		return "", err
	}
	if !time.Now().Before(event.ExpiresAt) {
		return "", nil
	}
	return event.EventId, nil
}

// getModeByType 根据 active 事件类型分发到对应事件处理器。
func (m *EventManager) getModeByType(ctx context.Context, user *userinfo.Info, eventType string, req GetModeRequest) (ModeResult, bool, error) {
	switch eventType {
	case EventTypeNewcomer:
		return m.getNewcomerMode(ctx, user, req)
	case EventTypeCold:
		return m.getColdMode(ctx, user, req)
	default:
		return ModeResult{}, false, nil
	}
}

// getNewcomerMode 将新手玩家事件结果转换为统一 ModeResult。
func (m *EventManager) getNewcomerMode(ctx context.Context, user *userinfo.Info, req GetModeRequest) (ModeResult, bool, error) {
	ok, eventID, rtpMode, useSelectedRtp, err := m.newcomer.GetMode(ctx, user, req.RoundNo)
	if err != nil || !ok {
		return ModeResult{}, false, err
	}
	return newModeResult(eventID, EventTypeNewcomer, rtpMode, useSelectedRtp), true, nil
}

// getColdMode 将冷水事件结果转换为统一 ModeResult。
func (m *EventManager) getColdMode(ctx context.Context, user *userinfo.Info, req GetModeRequest) (ModeResult, bool, error) {
	ok, eventID, rtpMode, useSelectedRtp, err := m.cold.GetMode(ctx, user, req.RoundNo, req.StrategySwitches)
	if err != nil || !ok {
		return ModeResult{}, false, err
	}
	return newModeResult(eventID, EventTypeCold, rtpMode, useSelectedRtp), true, nil
}

// newModeResult 生成业务侧统一消费的事件模式结果。
func newModeResult(eventID, eventType string, rtpMode int, useSelectedRtp bool) ModeResult {
	return ModeResult{
		InEvent:        true,
		EventId:        eventID,
		EventType:      eventType,
		RtpMode:        rtpMode,
		UseSelectedRtp: useSelectedRtp,
	}
}
