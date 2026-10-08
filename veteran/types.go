package veteran

import (
	"github.com/SinonetCPGame/game-kit/domain/entity"
	personalrtp "github.com/SinonetCPGame/game-kit/domain/service/personal-rtp"
	"github.com/SinonetCPGame/game-kit/domain/service/personal-rtp/session"
	"github.com/SinonetCPGame/game-kit/domain/service/predefined-result/v5"
	"github.com/redis/go-redis/v9"
)

// TriggerRequest 是本局普通投注出结果前的判定入参。
type TriggerRequest struct {
	BetAmount        int64                        // 当前普通投注金额，使用游戏侧放大后的整数。
	RoundNo          string                       // 游戏局号。
	StrategySwitches personalrtp.StrategySwitches // 业务方基于资金池缓存计算的单局策略开关。
}

// TriggerResult 是老玩家策略的触发结果。
type TriggerResult[R entity.IResult] struct {
	Triggered bool                        // 是否成功触发本局老玩家策略。
	EventID   string                      // 本次策略事件 ID。
	Result    *entity.PredefinedResult[R] // 本次策略触发的游戏结果。
}

// Manager 维护老玩家策略会话及触发判定。
type Manager[R entity.IResult] struct {
	redis           redis.UniversalClient
	session         *session.Manager
	config          Config[R]
	predefinedLogic *predefined.PredefinedLogic[R]
}
