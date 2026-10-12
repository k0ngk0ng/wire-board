# 璀璨宝石

`server/` 是本游戏独立的 Go 包，包含规则、动作、机器人、嵌入数据和规则测试。`web/` 包含棋盘、专属类型和界面组件；`web/module.tsx` 为前端注册入口。已有前端测试位于 `tests/`（如有）。

从仓库根目录执行 `. scripts/env.sh` 后，可用 `go test -count=1 ./games/splendor/server` 验证规则。房间协议的集成测试目前保留在 `internal/server`；六款旧游戏通过兼容层接入，暂不改变存档格式。

不导入其他游戏的实现，不向公共层新增游戏规则。新游戏应使用 [独立模块接入契约](../../docs/game-development.md)，而非复制旧房间适配代码。
