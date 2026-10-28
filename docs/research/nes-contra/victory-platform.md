# 辅助通关终局的平台验收

本入口验证已记录的真实胜利终局如何进入平台加载、正式规则计时器、统一结算浮层和积分账本。来源为 `.local/contra-bot/diagnostic-alien-terminal`：单人、177,154 帧、`CampaignCleared` 且 `Finalized`，包含已声明的备用生命补充。因此它不是标准命数、新局或完整电脑策略验收，也不是双人通关证据。

准备工具从标准 `NewSession` 开始重放完整按键流，按记录逐项应用补命，并核对原记录已有的全部 Session 字段一致。它只在验证成功后导出第 177,094 帧；最后 60 帧的记录输入全部为零，最后一次补命发生在第 176,174 帧。导出边界没有改写胜负、生命、位置或物理状态。

旧诊断早于持久战场回看，因此其新增 `review` 字段由同次重放派生，明确标记 `replayDerivedReviewAdded: true`。原字段的 `recordedRulesAndEffectsMatch` 为真，整份存储对象的 `recordedStateMatches` 为假；不能把新增呈现信息说成原存档本来就有。

测试宿主只在 `CONTRA_VICTORY_BOUNDARY` 显式启用时，把该边界作为房间的测试初态。它保留真实模块、关卡资源、Wasm、加载屏障及 `OnTimer`；规则时钟由私有原子门暂缓，释放后正式规则自然完成 60 帧并提交终局。没有客户端写检查点或直接获胜的操作，相关适配仅在 `_test.go` 中编译。

积分测试明确使用私有测试用户和测试数据库，将真人计分资格投影到这个边界，以验证通关的 `+20`、胜场、历史记录和幂等结算。它不表示补命诊断本身应获得正式竞技积分。真实双人共同加分仍由已有平台合作积分测试覆盖，待有真实双人终局后再沿用此入口。

## 终局回看

实际胜利终局的原版调色板已淡出为全黑。此前关闭平台结果后只剩黑色 Canvas，浏览器红测实测非黑像素为零。现在 `Session.Review` 持久保存实际提交的战场及同帧备用生命；记录覆盖战斗、死亡延时和退出序列的非淡出阶段，不记录标题／续关菜单或渐黑退出帧。正常规则、输入、RNG、World、原版调色板和 `FlushTerminal` 都保持原语义。

只有终局 `runtime.View` 选择这份保存的画面与配套 HUD。动作不会重新执行，音频仍读取真正终局时钟，刷新从检查点恢复同一画面。没有 `review` 的旧记录保持旧事实，不凭空生成回看画面。

本次真实通关仍在第 177,154 帧完成，终局提交 revision 为 177,155；保留的最后战场是第 177,061 帧／revision 177,061。PC 与 320px 结果浮层均显示「全队通关」和 `+20`，测试用户积分从 1000 到 1020，胜场和对局数各增加一次，刷新不重复记账。关闭后战场有 13 种颜色、19,256 个非黑像素，刷新前后 RGBA SHA-256 完全一致。另以标准命数自然失败回归 `−10` 及死亡延时战场保存，不把辅助胜利当作标准失败验收的替代。

验收还修复了公共房间摘要的真实遗漏：独立模块提前返回时缺少 `ratingPolicy`、`ratingStarted` 和 `ratingEligible`。HTTP 红测覆盖后，两个摘要分支现在都返回同一份冻结计分元数据；数据库公式没有改变。

完整结果、截图、来源与红绿记录见 `evidence/victory-platform-verification.json`。本轮没有宣称还原原版尾声动画；最终最新构建、完整新局和双人 AI 验收仍需后续统一冻结后执行。

## 复验

从工作区根目录执行；各输出目录必须尚不存在。

```sh
source scripts/env.sh
go run ./scripts/contra/replay_bot \
  --run .local/contra-bot/diagnostic-alien-terminal \
  --before-terminal 60 \
  --boundary-out .local/contra-victory/boundary-recheck

CONTRA_VICTORY_BOUNDARY="$PWD/.local/contra-victory/boundary-recheck" \
go test -race ./games/contra/server \
  -run '^TestAssistedVictoryFixtureUsesRealTerminalPath$' -count=1

python3 -B scripts/contra/browser_build.py --out .local/contra-victory/build
npm --prefix web run build -- --outDir "$PWD/.local/contra-victory/web-dist"

CONTRA_PLATFORM_BROWSER=1 \
CONTRA_PLATFORM_BROWSER_PUBLIC="$PWD/.local/contra-victory/build/public" \
CONTRA_PLATFORM_BROWSER_WEB="$PWD/.local/contra-victory/web-dist" \
CONTRA_PLATFORM_BROWSER_OUT="$PWD/.local/contra-victory/fixture-recheck" \
CONTRA_VICTORY_BOUNDARY="$PWD/.local/contra-victory/boundary-recheck" \
go test ./games/contra/server -run '^TestContraPlatformBrowserFixture$' \
  -count=1 -timeout 30m -v
```

在另一终端运行浏览器验收：

```sh
source scripts/env.sh
python3 -B scripts/contra/victory_browser_accept.py \
  --fixture .local/contra-victory/fixture-recheck/fixture.json --label recheck
```

脚本先验证等待房间没有全屏入口，加载完成后终局边界保持静止，再进入全屏并释放测试时钟。它检查结果浮层不早于已显示的终局帧、`全队通关` 与 `+20`、关闭后棋盘位置稳定、结果可重新查看、320px 的可见性／命中、退出后状态恢复，以及刷新不重复增加积分和历史。

每个 fixture 的计时门只释放一次，重新运行应启动新的 fixture。结束后创建其 `fixture.json` 中声明的 `stop` 文件关闭服务。浏览器脚本自动清理自身 profile、缓存和 socket；原始按键流、补命记录和检查点不应删除。规则源码仍在变化时，构建身份不一致必须重建，不能跳过校验；最终游戏验收应在统一源码冻结后另行完成。
