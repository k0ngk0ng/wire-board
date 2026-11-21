# 随机选择、一对三与队伍显示验收

日期：2026-11-21。

## 交付行为

- 角色和场地菜单均提供「随机」，服务端开局时统一抽取。角色独立抽取，允许重复；加载确认、托管、重连和服务重启沿用已抽取结果。
- 四人房间新增「一对三」：首席为单人方，其余三席组成三人队。支持电脑、主动托管和手动接管。
- 准备界面将队友放入同一张分组卡片；对局玩家栏沿用相同队伍和标题。窄屏下场地、赛制上下排列，避免所选名称竖排。
- 平台胜者、名次、原版对战统计和结算台词按实际队伍计算；玩法速查与大厅描述同步更新。

## 验证范围

先记录[失败场景](../tests/match-options-failures.md)，再编写测试与实现。先行验证分别复现了未知赛制字段、随机值被拒绝、原有统计错误地沿用二对二分组，以及窄屏名称换行。

| 检查 | 结果 |
| --- | --- |
| 服务端与共享 tick 完整测试 | 通过，包含原有 32 种场地／人数／赛制组合的完整对局及新增一对三电脑整局 |
| 随机开局 | 256 次开局覆盖全部 11 个角色和八个场地，固定选择及恢复后的选择保持一致 |
| 引擎定向回归 | 10 项通过，覆盖已有开局、完整对局、事务边界与原版生命周期对照 |
| 真实 HTTP 与 SQLite | 四个独立身份一致收到抽取结果，托管切换和数据库重启保持队伍、选择与就绪状态 |
| 客户端规则与呈现回放 | 从真实主机的原样检查点恢复，五次托管切换后的规则与呈现摘要均一致 |
| 竞态检查 | 新增服务端、真实 HTTP 与共享 tick 共七项通过 |
| 前端与 lint | 15 项通过，包含 Go、TypeScript、TSX、CSS 和 MJS 的实际登记范围与正负例 |
| 浏览器 | 实际创建选项、准备组件和公共玩家栏通过；覆盖 1280×800、320×700、844×390，菜单与页面无横向溢出、选中名称不换行 |
| TypeScript | 通过 |
| 旧存档与检查点 | 两项通过，既有 schema 1 字节基线保持一致 |
| Go/Wasm | 当前源码绑定构建、真实启动、301 帧与检查点往返通过 |

本地结果位于 `.local/yuyuhakusho/match-options/`：`result.json` 汇总范围和产物摘要，`browser-result.json` 记录布局测量与截图；其余 JSONL 保留 Go 测试结果，日志保留前端与构建结果。

## 复跑

在仓库根目录执行：

```sh
mkdir -p .local/yuyuhakusho/match-options/tmp .local/yuyuhakusho/match-options/telemetry .local/go-build .local/go-mod
export GOCACHE="$PWD/.local/go-build" GOMODCACHE="$PWD/.local/go-mod"
export GOTMPDIR="$PWD/.local/yuyuhakusho/match-options/tmp" TMPDIR="$PWD/.local/yuyuhakusho/match-options/tmp"
export TEST_TELEMETRY_DIR="$PWD/.local/yuyuhakusho/match-options/telemetry"
go test ./games/yuyuhakusho/server ./games/yuyuhakusho/sessiontick -count=1
go test ./games/yuyuhakusho/engine -run 'TestOneVsThree|TestChuSession|TestMatchLifecycle' -count=1
go test ./games/yuyuhakusho/platformtest -run 'TestRandomOneVsThree|TestOneVsThreeReal' -count=1
go test -race ./games/yuyuhakusho/server ./games/yuyuhakusho/sessiontick ./games/yuyuhakusho/platformtest -run 'TestRandom|TestOneVsThree|TestSharedTick' -count=1
go test ./games/yuyuhakusho/runtimebridge -run 'TestSchema1SaveBytesFrozen|TestSchema2PreviewCheckpointAndOwnership' -count=1
node --test games/yuyuhakusho/tests/*.test.mjs games/yuyuhakusho/build/runtime-delivery.test.mjs
node games/yuyuhakusho/tests/match-options-browser.mjs
web/node_modules/.bin/tsc -p web/tsconfig.json --noEmit
```
