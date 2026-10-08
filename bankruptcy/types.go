package bankruptcy

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
	MaxPayoutAmount  int64                        // 当前局允许的最大派彩金额，使用游戏侧放大后的整数。
	RoundNo          string                       // 游戏局号。
	StrategySwitches personalrtp.StrategySwitches // 业务方基于资金池缓存计算的单局策略开关。
}

// TriggerResult 是破产策略触发后返回的预制游戏结果及其策略上下文。
type TriggerResult[R entity.IResult] struct {
	Triggered bool                        // 是否成功触发本局破产策略。
	EventID   string                      // 本次策略生成的破产事件 ID。
	Result    *entity.PredefinedResult[R] // 本次策略触发的游戏结果。
}

// Manager 维护破产策略会话和事件。
type Manager[R entity.IResult] struct {
	redis           redis.UniversalClient          // Redis 客户端。
	session         *session.Manager               // 玩家策略会话管理器。
	config          Config[R]                      // 破产策略配置。
	predefinedLogic *predefined.PredefinedLogic[R] // 预制结果随机逻辑。
}
