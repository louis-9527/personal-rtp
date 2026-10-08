package personalrtp

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SinonetCPGame/game-kit/rds"
	"github.com/SinonetCPGame/game-kit/userinfo"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	// EventVersionV1 未写入 version 的线上旧 recovery 事件。
	EventVersionV1 = ""
	// EventVersionV2 支持 CV 进入校验及 bet_spike 退出。
	EventVersionV2 = "v2"
	// EventVersionV2_1 支持 recovery、新手玩家、冷水三个互斥事件。
	EventVersionV2_1 = "v2.1"
	// EventVersionLatest 新创建事件使用的最新版本。
	EventVersionLatest = EventVersionV2_1
)

const (
	EventTypeRecovery = "recovery"
	EventTypeNewcomer = "newcomer"
	EventTypeCold     = "cold"
)

const (
	eventIDPrefixRecovery = "R-"
	eventIDPrefixNewcomer = "N-"
	eventIDPrefixCold     = "L-"
)

// Event 个人 RTP 事件状态，记录触发快照与策略执行进度。
type Event struct {
	EventId              string    `json:"eventId"`              // 事件 ID，按事件类型使用 R-/N-/L- 前缀。
	Type                 string    `json:"type"`                 // 事件类型；线上旧 recovery 事件为空。
	Version              string    `json:"version"`              // 事件版本，用于兼容线上旧事件；未写入时为空，视为 v1。
	Uid                  string    `json:"uid"`                  // 玩家 ID
	RecentBetCount       int64     `json:"recentBetCount"`       // 触发时最近局数普通投注局数快照
	RecentBetAmount      int64     `json:"recentBetAmount"`      // 触发时最近局数普通投注总额快照（游戏侧放大后的整数金额）
	RecentPayoutAmount   int64     `json:"recentPayoutAmount"`   // 触发时最近局数普通投注派彩总额快照（游戏侧放大后的整数金额）
	RecentBetAmounts     []float64 `json:"recentBetAmounts"`     // 触发时最近局数各局下注额快照（按时间倒序，最新在前）
	StrategyBetCount     int64     `json:"strategyBetCount"`     // 策略期间普通投注局数（用于达到最大策略局数退出及总RTP计算）
	StrategyBetAmount    int64     `json:"strategyBetAmount"`    // 策略期间普通投注总额（游戏侧放大后的整数金额）（用于总RTP计算）
	StrategyPayoutAmount int64     `json:"strategyPayoutAmount"` // 策略期间普通投注派彩总额（游戏侧放大后的整数金额）（用于总RTP计算）
	RecentCv             float64   `json:"recentCv"`             // 触发时最近局数下注变异系数
	MedianBet            float64   `json:"medianBet"`            // 触发时最近局数中位下注（用于下注激增退出判断）

	NewcomerTargetBetCount  int64 `json:"newcomerTargetBetCount,omitempty"`  // 新手事件随机判定局数 N
	NewcomerSelectedRtpMode int   `json:"newcomerSelectedRtpMode,omitempty"` // 新手事件选中的 RTP mode
	RecoverySelectedRtpMode int   `json:"recoverySelectedRtpMode,omitempty"` // recovery 事件选中的 RTP mode
	ColdSelectedRtpMode     int   `json:"coldSelectedRtpMode,omitempty"`     // 冷水事件选中的 RTP mode，是否真正使用由 event.UseSelectedRtp 判定
	ColdMaxSpins            int64 `json:"coldMaxSpins,omitempty"`            // 冷水事件开始时固化的最多执行局数
	UseSelectedRtp          bool  `json:"useSelectedRtp,omitempty"`          // 是否使用事件选中的 RTP mode

	StartedAt     time.Time `json:"startedAt"`     // 事件开始时间
	ExpiresAt     time.Time `json:"expiresAt"`     // 事件过期时间
	LastUpdatedAt time.Time `json:"lastUpdatedAt"` // 最近一次推进时间
}

func (e *Event) eventType() string {
	if e == nil {
		return ""
	}
	if e.Type != "" {
		return e.Type
	}
	return EventTypeRecovery
}

// newEventID 按事件类型生成带前缀的事件 ID。
func newEventID(eventType string) string {
	switch eventType {
	case EventTypeNewcomer:
		return eventIDPrefixNewcomer + uuid.NewString()
	case EventTypeCold:
		return eventIDPrefixCold + uuid.NewString()
	default:
		return eventIDPrefixRecovery + uuid.NewString()
	}
}

// getEvent 读取玩家当前 active 的个人 RTP 事件。
func (s *store) getEvent(ctx context.Context, uid string) (*Event, bool, error) {
	payload, err := s.redis.Get(ctx, rds.GetPersonalRtpEventActiveKey(uid)).Bytes()
	if err != nil {
		if err != redis.Nil {
			return nil, false, err
		}

		// 兼容 v2.1 上线前写入的 recovery active key。
		payload, err = s.redis.Get(ctx, rds.GetPersonalRtpRecoveryActiveKey(uid)).Bytes()
		if err != nil {
			if err == redis.Nil {
				return nil, false, nil
			}
			return nil, false, err
		}
	}

	event := &Event{}
	if err = json.Unmarshal(payload, event); err != nil {
		return nil, false, err
	}
	return event, true, nil
}

// setActiveEvent 以 SetNX 创建 active 事件，保证三类事件互斥。
func (s *store) setActiveEvent(ctx context.Context, uid string, event *Event, ttl time.Duration) (bool, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return false, err
	}
	return s.redis.SetNX(ctx, rds.GetPersonalRtpEventActiveKey(uid), payload, ttl).Result()
}

// updateActiveEvent 使用事件剩余时间刷新 active 事件状态。
func (s *store) updateActiveEvent(ctx context.Context, uid string, event *Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	remaining := time.Until(event.ExpiresAt)
	if remaining <= 0 {
		return nil
	}
	return s.redis.Set(ctx, rds.GetPersonalRtpEventActiveKey(uid), payload, remaining).Err()
}

// clearActiveEvent 清理 active 事件，并按需要写入对应冷却 key。
func (s *store) clearActiveEvent(ctx context.Context, uid, cooldownKey, cooldownValue string, cooldownTTL time.Duration) error {
	pipe := s.redis.Pipeline()
	pipe.Del(ctx, rds.GetPersonalRtpEventActiveKey(uid))
	pipe.Del(ctx, rds.GetPersonalRtpRecoveryActiveKey(uid))
	if cooldownTTL > 0 && cooldownKey != "" {
		pipe.Set(ctx, cooldownKey, cooldownValue, cooldownTTL)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// eventFinishFunc 是各事件 finish 方法的通用签名。
type eventFinishFunc func(context.Context, *userinfo.Info, *Event, string, string) error

// resolveModeEvent 处理 GetMode 共有的 active 事件读取、互斥和过期逻辑。
// 返回值 handled 表示已经存在 active 事件或刚处理过过期事件，不应继续尝试创建新事件。
func (s *store) resolveModeEvent(ctx context.Context, user *userinfo.Info, eventType, roundNo string, finish eventFinishFunc) (*Event, bool, error) {
	event, exists, err := s.getEvent(ctx, user.Uid)
	if err != nil || !exists {
		return nil, false, err
	}
	if event.eventType() != eventType {
		return nil, true, nil
	}
	if !time.Now().Before(event.ExpiresAt) {
		if err = finish(ctx, user, event, "expired", roundNo); err != nil {
			return nil, true, err
		}
		return nil, true, nil
	}
	return event, true, nil
}

// resolveAdvanceEvent 处理 Advance 共有的 active 事件读取、类型过滤和过期逻辑。
func (s *store) resolveAdvanceEvent(ctx context.Context, user *userinfo.Info, eventType, roundNo string, finish eventFinishFunc) (*Event, error) {
	event, exists, err := s.getEvent(ctx, user.Uid)
	if err != nil || !exists {
		return nil, err
	}
	if event.eventType() != eventType {
		return nil, nil
	}
	if !time.Now().Before(event.ExpiresAt) {
		return nil, finish(ctx, user, event, "expired", roundNo)
	}
	return event, nil
}

// startEvent 创建 active 事件；如果并发下已有同类型事件，则返回已有事件。
func (s *store) startEvent(ctx context.Context, uid string, event *Event, ttl time.Duration) (*Event, bool, error) {
	ok, err := s.setActiveEvent(ctx, uid, event, ttl)
	if err != nil {
		return nil, false, err
	}
	if ok {
		return event, true, nil
	}

	active, exists, err := s.getEvent(ctx, uid)
	if err != nil || !exists || active.eventType() != event.eventType() {
		return active, false, err
	}
	return active, true, nil
}

// advanceEventProgress 推进事件期间普通投注统计。
func advanceEventProgress(event *Event, betAmount, payoutAmount int64) {
	event.StrategyBetCount++
	event.StrategyBetAmount += betAmount
	event.StrategyPayoutAmount += payoutAmount
	event.LastUpdatedAt = time.Now()
}
