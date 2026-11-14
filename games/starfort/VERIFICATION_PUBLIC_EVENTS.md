# 星垒公共事件与任务欠款验收

需求与实现前失败场景见 [本次修订](PUBLIC_EVENTS.md)。新局为存档版本 14；旧存档版本 1～13 保留原规则。

## 重放

所有缓存与产物留在当前仓库；先创建 `.local/tmp`、`.local/go-build` 与 `.local/go-mod`。执行：

```sh
export GOTOOLCHAIN=local
export GOCACHE="$PWD/.local/go-build"
export GOMODCACHE="$PWD/.local/go-mod"
export GOTMPDIR="$PWD/.local/tmp"
export TMPDIR="$PWD/.local/tmp"
go test -json -count=1 -timeout=180m ./games/starfort/server ./games/starfort/tests > .local/starfort-all.jsonl
go test -json -race -count=1 -timeout=15m -run '^TestStarfortPublic(Version|Shared|Settlement|Task|Boss|HTTP|Reward)' ./games/starfort/tests > .local/starfort-public-race.jsonl
node --test games/starfort/tests/*.test.mjs
npm --prefix web run build
STARFORT_FOLLOWUP_SOCKET_DIR="$PWD/.s" python3 -B games/starfort/tests/accept_public_events_ui.py --skip-build
```

浏览器验收保存 JSON、动作记录和桌面／320px／全屏截图，目录为 `.local/starfort-public/browser`。使用 Chromium 原生鼠标与触屏仿真，不替代物理手机或 Safari 验收。

## 验证范围

| 需求 | 证据入口 |
| --- | --- |
| 独立 30% 共用概率、刷新不重抽 | `TestStarfortPublicSharedProbabilityAndStableRestore`，固定种子 500 波 |
| 当波收入结算、欠款、任务奖励排除 | `TestStarfortPublicSettlementAndDebt`、`TestStarfortPublicTaskBorrowingAndEarlyCompletion` |
| 首领奖励和四塔成长修订 | `TestStarfortPublicBossBonusAndSkillGrowth` 与既有专精／天气／灾害验收 |
| 奖励选择不消耗整备时间、多人选择 | `TestStarfortPublicRewardChoiceClock` |
| 版本边界、负余额、事件与计时保存、托管接管 | `TestStarfortPublicVersionAndLegacy`、`TestStarfortPublicHTTP` |
| 1～4 人、20 波及简单／困难自然完整局 | `TestStarfortPublicNatural`，不注入金币或胜负 |
| 事件字号、实际可见性、十秒到期、道路分区 | `accept_public_events_ui.py` |
| 首领关闭／重开／刷新／全屏、欠款、光圈、三颗星、塔编号、动态停攻 | 同一浏览器脚本，另保留原有洪水紫圈及售塔确认验收 |
| 生命成长、逐路奖励、四类任务、商店及其他天气／灾害／技能 | 全部星垒规则与前端回归；不扩大到其他游戏 |

完整星垒套件包含各存档版本的自然对局矩阵。未分片 CI 的 Go 进程时限为 90 分钟，分片仍为 45 分钟；这只调整执行时限，测试集合、断言与游戏性能预算保持原要求。

本次发布的 main 构建、CDN 本地回读、版本镜像、生产冒烟与清理结果另保存在 `.local/starfort-delivery-nov14/release`。发布状态以实际 Release 和该目录最终收据为准。
