package recovery

import (
	"github.com/SinonetCPGame/game-kit/domain/entity"
	personalrtp "github.com/SinonetCPGame/game-kit/domain/service/personal-rtp"
	"github.com/SinonetCPGame/game-kit/domain/service/personal-rtp/session"
	"github.com/SinonetCPGame/game-kit/domain/service/predefined-result/v5"
	"github.com/redis/go-redis/v9"
)

// TriggerRequest 是普通投注出结果前的补偿策略判定入参。
type TriggerRequest struct {
	BetAmount        int64                        // 当前普通投注金额，使用游戏侧放大后的整数。
	RoundNo          string                       // 当前游戏局号，用于日志追踪。
	StrategySwitches personalrtp.StrategySwitches // 业务方基于资金池档位计算的策略开关。
}

// TriggerResult 是业务方消费的补偿策略触发结果。
type TriggerResult[R entity.IResult] struct {
	Triggered bool                        // 是否成功触发本局补偿策略。
	EventID   string                      // 本会话内本次补偿事件 ID。
	Result    *entity.PredefinedResult[R] // 本次策略触发的游戏结果。
}

// Manager 维护会话补偿策略的触发判定。
type Manager[R entity.IResult] struct {
	redis           redis.UniversalClient
	session         *session.Manager
	config          Config[R]
	random          func() float64
	predefinedLogic *predefined.PredefinedLogic[R]
}
