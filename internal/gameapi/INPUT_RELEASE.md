# 托管席位的输入释放

实时模块可显式声明 `Module.IsInputRelease(Context, Match, json.RawMessage) bool`。它仅允许已登录真人在自己的托管席位清除尚未处理的物理输入，不取消托管，不授权按下、恢复或其他游戏决策。非实时模块不能注册此能力；未声明的模块维持原拦截行为。

模块须检查动作结构、席位、对局 epoch 和序号，只在确认是合法释放时返回 true。宿主仍执行完整 `Apply`、版本／nonce、加载屏障和持久化检查，不能把此 predicate 当作直接执行。旧房间版本还必须满足该模块已有的 `AcceptStale`。

HTTP 普通动作、HTTP 旧版本入口与 WebSocket 实时输入共享同一个能力检查。回调得到独立的 Match、Context.Automated 和 RawAction；修改这些值不会影响请求日志或房间。回调 panic 按拒绝处理。

魂斗罗的真实 HTTP／WS／SQLite 验收先复现旧 400 拒绝，再覆盖合法释放、nonce 重试、旧版本、未启用能力、predicate panic／修改参数、其他操作及观战者拒绝、重启和取消托管后的序号保持。记录位于 `docs/research/nes-contra/evidence/server-host-release-verification.json`。前端也须提供相应的显式释放例外，不能把普通操作整体放行。
