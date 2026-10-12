# 游戏模块

每款游戏独立维护规则、动作、视图、前端和测试。新游戏实现 `server.Module()`，在本目录 `registry.go` 注册后端，并默认导出 `web/module.tsx` 供前端自动发现。

接入契约、目录约定和可重复验收命令见 [游戏开发说明](../docs/game-development.md)。游戏之间禁止直接导入实现，可运行 `python3 -B scripts/check_game_boundaries.py` 检查。

现有六款游戏保留旧房间协议适配；新游戏直接使用 `internal/gameapi`，不扩展旧 `internal/game` 或 `web/src/legacy`。
