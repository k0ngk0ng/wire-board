# 服务端验收场景

以下场景在服务端实现之前登记。测试通过公开 Module 和真实 HTTP／WebSocket 房间执行，不注入位置、生命或得分。

- 创建：2、3、4 人可用；非法人数、地图、武器、角色、时长、重复字段、未知字段、空值和过长输入被拒绝。
- 准备：本人修改造型与武器；他人和旁观者不能代选；仅房主修改地图和时长；房主可以管理电脑席位；开局冻结配置。
- 对战：九张地图全部开放人数的电脑自然完整比赛；通过相同输入接口移动、开火、拾取、投掷；结算与实际得分一致，平分使用显式和局。
- 积分：真人开启托管后永久登记本局有电脑参与，取消托管或重启不能恢复竞技计分资格。
- 输入：过时代次、复活前生命、未来或跳跃序号、越界轴与角度、客户端结果注入被拒绝；重复输入不重复攻击；不同真人的旧房间版本输入可并发提交。
- 确认：收到与已应用序号区分；短按边沿保留；事件游标不重播；常规帧不含地图、素材、输入队列和无限历史。
- 时钟：固定 60 Hz 规则推进；同一时刻重复定时不重复规则；输入释放与取消托管不改变身份、得分、赛时或其他席位输入。
- 恢复：真实 SQLite 重启保持同局；释放瞬时操作；不重置分数或复活；重连不自动托管。
- 隔离：慢 WebSocket 不阻塞快端与房间时钟；同服务多个满员房间并行，记录推进与投影耗时和协议字节。

同服务负载验收固定为 8 个四人房间，全部席位经过普通平台托管输入推进，另保留一个不读取的 WebSocket。逐房间读取实际权威时钟，P95 落后不得超过 67 ms；这与模块回调 P95 不超过 8 ms 的预算分别检查。

本地输出统一为 `.local/hotlinemiami/verification/`；首次执行的编译失败作为红色测试记录保留，之后保留最终测试 JSON 和自然比赛结果。

本次大图复核先登记失败场景：自然比赛矩阵仍只跑旧六图；大图入口存在但真实 Module 不可完成 2／3／4 人比赛；八房压力只用旧小图而遗漏 Exposed；慢读 WebSocket 没有挂在大图满员房；只报告汇总百分位而丢失原始样本；快速 View 掩盖慢 BotAction 的回调尾延迟；测试运行期间放宽 8 ms 回调或 67 ms 规则时钟预算。

自然比赛明确覆盖 9 图 × 2／3／4 人，共 27 场真实生产 Module 比赛，按 60 Hz 时钟走满默认 3 分钟。八房配置固定含 3 个 Exposed、Highball、Hot&Heavy 和 3 个原有场景，均为四席；第一个 Exposed 房间附加不读取且请求 1,024 字节接收缓冲的 WebSocket。每种回调分别记录纳秒原始样本、P95 与最大值，各自 P95 不超过 8 ms；权威时钟保留逐房间毫秒原始落后样本，P95 不超过 67 ms。

复跑生产模块矩阵：`go test ./games/hotlinemiami/tests -run '^TestNaturalCompleteMatches$' -count=1 -timeout=30m -json`。复跑真实八房负载：`go test ./games/hotlinemiami/tests -run '^TestEightLiveRoomsAndSlowReceiver$' -count=1 -timeout=2m -json`。执行前显式创建 `.local/hotlinemiami/cache/go`、`.local/hotlinemiami/tmp` 与验收输出目录，并将 `GOCACHE`、`GOTMPDIR`、`TMPDIR` 指向对应工作树路径，设置 `GOTELEMETRY=off`。

逐场生产 Module 初始权威状态、随机种子、最终投影、输入数量、峰值消息字节和结算保存在 `.local/hotlinemiami/verification/module/TestNaturalCompleteMatches-<map>-<players>.json`，原始测试事件使用 `-json` 另存。八房真实 HTTP／SQLite 结果保存在 `TestEightLiveRoomsAndSlowReceiver.json`，包含 320 个逐房时钟样本和逐类原始回调纳秒样本，失败时也先写结果与违反项；不能只保留通过标记或经过排序的汇总值。
