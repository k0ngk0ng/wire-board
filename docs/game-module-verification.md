# 游戏模块拆分验证记录

基于 `c969fde` 的工作区改动。代码按游戏拆包，并保留旧房间协议适配；新增游戏通过 `gameapi` 接入。开发步骤见 [游戏开发说明](game-development.md)。

## 已通过

| 验证 | 结果 | 本地产物 |
| --- | --- | --- |
| 前端生产构建 | TypeScript 和 Vite 通过 | `.local/module-refactor/browser/build-production.log` |
| 前端测试 | 265 项通过，0 失败 | `.local/module-refactor/frontend-tests.log` |
| 卡卡颂、DotA、铁路、三国杀、宝石及旧规则适配层的完整测试 | 265 个顶层用例通过，含子用例共 809 个通过事件 | `.local/module-refactor/game-regression.jsonl` |
| 卡坦针对性回归 | 18 个顶层用例通过，含子用例共 26 个通过事件 | `.local/module-refactor/catan-regression.jsonl` |
| 带 race 的服务端生命周期验收 | 15 个顶层用例通过，含子用例共 26 个通过事件 | `.local/module-refactor/lifecycle-race.jsonl` |
| 最终独立模块验收 | 注册、配置、隐私、非法动作、版本、幂等、重启、机器人、超时、结算、再来一局通过 | `.local/module-refactor/module-e2e.jsonl`、`module-lifecycle.json` |
| 真实浏览器 | 16 个检查点通过；含独立模块完整对局和六款旧游戏桌面／手机牌桌 | `.local/module-refactor/browser/result.json` 及同目录截图 |
| 公开素材 | DotA 的 58 个文件与构建产物逐字节一致 | 可对照 `games/dota/assets/public` 和 `web/dist` |
| 依赖边界 | 六款游戏无直接跨游戏实现依赖 | `python3 -B scripts/check_game_boundaries.py` |
| 测试迁移完整性 | 原有 1,721 个规则测试／示例／模糊测试入口全部保留 | 对照调研基线与当前游戏包 |
| 其他检查 | `go vet ./...`、Go 构建、主程序测试、分片脚本测试、格式与差异检查通过 | `.local/module-refactor/` 下对应日志 |

卡坦针对性回归覆盖基础开局、机器人完局与隐私、交易、发展卡、资源生产、胜负、双人规则、探索者存档与超时，以及前后端河流地图预览一致性。生命周期验收另外覆盖卡坦 HTTP 完局、多人弃牌／交易、重启及六款游戏完整托管对局。

浏览器验收使用真实生产构建和测试服务：从大厅进入临时注册的游戏，在新旧创建界面之间切换，编辑配置，添加电脑，准备／开局，提交独立动作，核对倒计时与空日志，完成对局并刷新核对结果。随后逐款检查原有游戏的 1,280 px 和 390 px 页面、资源加载、横向溢出和 JavaScript 错误。验收结束后移除临时游戏，并重新构建正常前端；生产包不包含验收模块。

## 范围与未解决事项

- 没有跑完完整卡坦扩展组合矩阵。探索性全量运行约 20 分钟后停止，部分日志保存在 `.local/module-refactor/go-tests.jsonl`，不能将这份部分运行记为全量通过。
- 探索性运行中，`TestCatanAttackTransportEndTimeoutRestart/false` 有一次自然对局在目标骑士移动阶段出现前结束。当前代码随后单独重复 3 次通过；原始 `c969fde` 基线重复 20 次也通过。该失败尚未稳定复现，原因未确定。相关输出分别位于 `attack-transport-recheck.jsonl` 和 `attack-transport-baseline.jsonl`，后续扩展矩阵验收需继续观察。
- 初次测试迁移发现一条宝石测试同时验证铁路隔离，已将该原有用例保留在旧 API 边界；最终完整回归通过。
- Dockerfile 已补齐前端构建阶段的 `games/` 输入，本次没有执行 Docker 镜像构建或部署。
- 六款旧游戏的复杂房间配置和计时／归档集成仍保留兼容逻辑。本次验证的是规则包隔离、前端归属和新游戏接入，不代表旧房间集成已全部迁移到 `gameapi`。

## 重复执行

```sh
. scripts/env.sh
python3 -B scripts/check_game_boundaries.py
npm --prefix web run build
npm --prefix web test
go vet ./...
go test -count=1 ./games/carcassonne/server ./games/dota/server \
  ./games/rail/server ./games/sanguosha/server ./games/splendor/server ./internal/game
go test -race -count=1 -timeout=2m \
  -run '^TestModule(RegistrationAndLifecycle|BotAndTimeout)E2E$' ./internal/server
python3 -B scripts/accept_game_modules.py
```

浏览器产物包含断言结果、页面快照、截图和日志；服务器接入验收包含操作轨迹。上述浏览器命令会临时构建验收模块，不应与其他前端构建同时执行。完整扩展矩阵仍按现有 CI 的 20 个规则分片运行。
