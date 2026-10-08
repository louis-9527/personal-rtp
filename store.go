package personalrtp

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/SinonetCPGame/game-kit/rds"
	"github.com/SinonetCPGame/game-kit/userinfo"
	amountutil "github.com/SinonetCPGame/game-kit/utils/amount-util"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

// RecentBetRecord 单局普通投注记录，用于 Redis 最近普通投注列表。
type RecentBetRecord struct {
	BetAmount    int64 `json:"betAmount" bson:"bet_amount"`       // 投注额（游戏侧放大后的整数金额）
	PayoutAmount int64 `json:"payoutAmount" bson:"payout_amount"` // 派彩额（游戏侧放大后的整数金额）
}

// RecentStats 玩家最近普通投注统计数据，由 Redis 列表动态汇总。
type RecentStats struct {
	BetCount     int64     `json:"betCount"`     // 局数
	BetAmount    int64     `json:"betAmount"`    // 投注总额（游戏侧放大后的整数金额）
	PayoutAmount int64     `json:"payoutAmount"` // 派彩总额（游戏侧放大后的整数金额）
	BetAmounts   []float64 `json:"betAmounts"`   // 各局下注额，按时间倒序（最新在前）
	Cv           float64   `json:"cv"`           // 变异系数：标准差 / 平均下注
	MedianBet    float64   `json:"medianBet"`    // 中位下注
	AvgBet       float64   `json:"avgBet"`       // 平均下注
	StdBet       float64   `json:"stdBet"`       // 标准差
	UpdatedAt    time.Time `json:"updatedAt"`    // 最近汇总时间
}

// store 维护三个个人 RTP 事件共用的 Redis 状态。
type store struct {
	redis  redis.UniversalClient
	config Config
}

// newStore 创建个人 RTP 事件共享 Redis 存储。
func newStore(redisCli redis.UniversalClient, opts ...Option) *store {
	config := defaultConfig()
	for _, opt := range opts {
		opt(&config)
	}
	return &store{redis: redisCli, config: config}
}

// recentListLen 获取玩家最近普通投注列表长度。
func (s *store) recentListLen(ctx context.Context, user *userinfo.Info) (int64, error) {
	count, err := s.redis.LLen(ctx, rds.GetPersonalRtpRecentListKey(user.Uid)).Result()
	if err != nil {
		return 0, err
	}
	return count, nil
}

// initRecentBets 用外部查询结果初始化最近普通投注列表。
// records 需按投注时间倒序排列（最新在前）。
func (s *store) initRecentBets(ctx context.Context, user *userinfo.Info, records []RecentBetRecord) error {
	key := rds.GetPersonalRtpRecentListKey(user.Uid)
	pipe := s.redis.Pipeline()
	pipe.Del(ctx, key)
	// 使用 RPush 从右侧插入，保持原有顺序
	for _, record := range records {
		payload, err := json.Marshal(record)
		if err != nil {
			return err
		}
		pipe.RPush(ctx, key, payload)
	}
	pipe.LTrim(ctx, key, 0, s.config.recentListMaxLen-1)
	pipe.Expire(ctx, key, s.config.recentStatsTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// appendRecentBet 将一局普通投注追加到最近普通投注列表头部。
func (s *store) appendRecentBet(ctx context.Context, user *userinfo.Info, record RecentBetRecord) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	key := rds.GetPersonalRtpRecentListKey(user.Uid)
	pipe := s.redis.Pipeline()
	pipe.LPush(ctx, key, payload)
	pipe.LTrim(ctx, key, 0, s.config.recentListMaxLen-1)
	pipe.Expire(ctx, key, s.config.recentStatsTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// enrichBetStats 根据前 spins 局下注额计算 CV、MedianBet 等统计并写回 RecentStats。
func (r *RecentStats) enrichBetStats(spins int64) error {
	if r == nil {
		return errors.New("recent stats is nil")
	}
	if spins <= 0 {
		return errors.New("spins must be greater than 0")
	}
	if int64(len(r.BetAmounts)) < spins {
		return errors.New("insufficient bet amounts")
	}

	bets := r.BetAmounts[:spins]
	for _, bet := range bets {
		if bet <= 0 {
			return errors.New("all bets must be greater than 0")
		}
	}

	// 计算平均下注
	var sum float64
	for _, bet := range bets {
		sum += bet
	}
	avg := sum / float64(len(bets))

	// 计算方差
	var varianceSum float64
	for _, bet := range bets {
		diff := bet - avg
		varianceSum += diff * diff
	}
	variance := varianceSum / float64(len(bets))

	// 计算标准差
	std := math.Sqrt(variance)

	// 计算变异系数
	cv := std / avg

	// 计算中位下注
	sortedBets := append([]float64(nil), bets...)
	sort.Float64s(sortedBets)

	n := len(sortedBets)
	var medianBet float64
	if n%2 == 1 {
		medianBet = sortedBets[n/2]
	} else {
		medianBet = (sortedBets[n/2-1] + sortedBets[n/2]) / 2
	}

	r.Cv = cv
	r.MedianBet = medianBet
	r.AvgBet = avg
	r.StdBet = std
	return nil
}

// LoadRecentStats 从 Redis 列表动态汇总玩家最近普通投注 RTP 数据。
// limit 大于 0 时只统计最近 limit 局；第二个返回值表示 Redis 中是否存在该列表。
func LoadRecentStats(ctx context.Context, redisCli redis.UniversalClient, uid string, limit int64) (*RecentStats, bool, error) {
	stop := int64(-1)
	if limit > 0 {
		stop = limit - 1
	}
	items, err := redisCli.LRange(ctx, rds.GetPersonalRtpRecentListKey(uid), 0, stop).Result()
	if err != nil {
		return nil, false, err
	}
	if len(items) == 0 {
		return nil, false, nil
	}

	stats := &RecentStats{UpdatedAt: time.Now()}
	for _, item := range items {
		record := RecentBetRecord{}
		if err = json.Unmarshal([]byte(item), &record); err != nil {
			return nil, false, err
		}
		stats.BetCount++
		stats.BetAmount += record.BetAmount
		stats.PayoutAmount += record.PayoutAmount
		stats.BetAmounts = append(stats.BetAmounts, amountutil.ShrinkToFloat(record.BetAmount))
	}
	return stats, true, nil
}

// calcEventRtpDetail 计算事件相关 RTP。
func calcEventRtpDetail(event *Event) (recentRtp, strategyRtp, totalRtp float64) {
	if event == nil {
		return 0, 0, 0
	}
	recentRtp = calcRtp(event.RecentBetAmount, event.RecentPayoutAmount)
	strategyRtp = calcRtp(event.StrategyBetAmount, event.StrategyPayoutAmount)
	totalRtp = calcRtp(
		event.RecentBetAmount+event.StrategyBetAmount,
		event.RecentPayoutAmount+event.StrategyPayoutAmount,
	)
	return recentRtp, strategyRtp, totalRtp
}

// calcRtp 根据投注额和派彩额计算 RTP。
func calcRtp(betAmount, payoutAmount int64) float64 {
	if betAmount <= 0 {
		return 0
	}
	return decimal.NewFromInt(payoutAmount).Div(decimal.NewFromInt(betAmount)).InexactFloat64()
}
