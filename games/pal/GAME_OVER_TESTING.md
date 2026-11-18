# 原版战败、失败退出与重新开始验收

## 实现前失败场景

1. 将盖罗娇等原版允许战败后继续的剧情改成普通死亡，或修改人物／敌方体力、资源、随机状态或战斗结果制造失败。
2. 省略原版红色渐变、文字颜色保留及「胜败乃兵家常事也／大侠请重新来过吧」对白，或者无法在画面点击继续原版消息。
3. 原版 `PAL_ReloadInNextTick` 在没有原生文件槽时静默重置剧情；将重开当成读取上次平台存档，或生成平台胜利结算。
4. 保存退出失败画面，覆盖此前健康的手动基准；继续后只能反复读到失败。当前失败快照和可回退基准须独立保留。
5. 没有此前手动存档时伪造存档；重新开始仍沿用旧 `MatchID`、旧操作边界、旧资源或旧存档入口；重试再次初始化。
6. 回退／重新开始允许陌生人、观众、电脑或旧页面操作；数据库或模块初始化失败时提前丢弃当前状态和基准。
7. 红色相位、索引 0x4F 到 0x4E 的原版画面变换、原生等待及新实例保存恢复不一致。前端只变调色板而漏掉原版索引替换。
8. 只用独立失败脚本夹具就宣称真实战败完成；旧核心内存被改写身份移到新核心。
9. PC／手机不能读档或重新开始，失败期间计时空转；按钮、失败面板或加载层缩小游戏区域。

固定资源中失败脚本从 41075 开始：开始对白、`0x004F` 红色渐变、居中对白、原消息 13135～13138，最后 `0x004E` 淡出并请求原生文件槽重载。红色演出和消息保持原始实现，重载边界交给平台本人读档／重新开始操作，不能伪造原生载入。

先验证公共平台的失败保存不覆盖手动基准，以及重新开始的事务／身份／幂等；再用原版脚本夹具核对呈现和等待，最后从新核心真实新游戏普通操作到达战斗，以原版防御指令自然战败。夹具、实际流程及浏览器分别留证。

## 当前实现

`PAL_FadeToRed` 的原版循环、原文、索引替换和计时保持不变，仅增加失败观察和绘制图中的索引替换节点。`PAL_ReloadInNextTick` 在普通初始化时仍执行原版函数；原版失败演出已经开始后，文件槽重载转为可持久化的失败等待事件。此处不重新初始化人物，不改变战斗结果，也不调用平台胜利结算。

原生失败状态区分演出中和已请求重载。演出中继续保留原版对白和按画面继续；重载请求到达后，当前实例停止计时，等待本人读取平台手动存档或重新开始。旧核心没有该观察接口时保持旧核心绑定，不改写其内存身份。

平台 `Match.PreserveSavedCheckpoint` 用于明确保留此前手动基准，`Match.Restartable` 仅在失败等待时开放本人重新开始。保存退出仍保留当时失败演出的完整当前快照，但不会用它替换可回退基准。读取恢复同一 `MatchID`；重新开始通过新原版游戏初始化建立新 `MatchID`，清除旧基准和旧操作边界。初始化／持久化失败保留原旅程；重试不会再次初始化。

重新开始能力随普通游戏快照和即时流同步。失败保存保护与重新开始状态变化均属于必须持久化后才能发布的关键边界；不能只在低频房间摘要中更新权限，导致画面已经失败而按钮没有出现。暂停呈现回调沿用冻结完整控制信封的约束，不能改变上述能力。

## 独立原版脚本与像素结果

| 用途 | Wasm SHA-256 | 目录 |
| --- | --- | --- |
| 原版剧情 | `aa3645c50f8fd8fde37b065a8b2df6e7e8b20354587dd8b0e6335890ab09af78` | `.local/pal/runtime-candidates/game-over/original/` |
| 仅开发加血剧情 | `291f7877225ce34358635f901b5c94ed1b98a749063b97555a0df503fda5e687` | `.local/pal/runtime-candidates/game-over/hp-9999/` |
| 独立失败脚本 | `2bddc28e45df2f433397f70d5e3cbe8c77c1c6ddfd795c8b16cb4512e8bd66ef` | `.local/pal/runtime-candidates/game-over/fixture/` |
| 原版 C 字体像素参考 | `7481ab1ba6a075c086e550430e9c92bfc17e9c88c2f280b07ed183b0346dc1f1` | `.local/pal/runtime-candidates/game-over/reference/` |

上述候选没有覆盖默认核心。原版源文件、104 个资源文件和 14 个附加媒体的摘要回读保持一致，记录为 `.local/pal/verification/game-over-identities.json`。已有场景波动核心的存档继续绑定其真实版本。

- 原版失败脚本的 32 个红色相位、原始消息 13135～13138、淡出及原生重载请求通过。112 个呈现位置，6 个全内存新实例续行位置通过；现金、随机状态不变。Go 引擎另验证相同链路和非法失败元数据拒绝。
- 原版 C 像素参考与实际 `SceneRenderer`／`PalStage` 在 PC、手机竖屏和横屏通过 336 次逐像素比较，包含画面索引 0x4F 到 0x4E 的替换及后续新增文字不被再次替换。
- 专用失败夹具用途为 `game-over-fixture`，不是剧情核心，已验证不能进入剧情运行器目录。原版字体参考和普通规则夹具的元数据分别绑定实际程序摘要。

结果为 `.local/pal/verification/game-over-reference.json`、`game-over-authority.json`、`game-over-engine.json` 与 `.local/pal/game-over-browser/result.json`。这些结果单独不能证明自然战败或实际剧情完成。

## 真实原版战斗与平台结果

真实来源从原版数值的新游戏开始，正常完成求药、返回、救治、夜间授艺及客栈救援战入口。原版此时已经是逍遥／灵儿两人，并非单独逍遥；首次测试的队伍断言错误已修正，未修改人物、队伍或资源。入口及原路证明保存在新核心自身的验收目录。

从同一真实入口只提交原版防御指令，累计 242 次选招后自然全队倒地。原版失败对白、红色演出和重载等待均实际到达，覆盖全队倒地、红色演出中、失败阅读和等待恢复中的保存／新实例恢复。等待状态在离线计时调用后保持不变，读取原手动快照后回到相同的原版战斗、体力、真气和选招位置，新操作代次高于失败实例。结果为同候选目录内的 `game-over-actual-1.json`。

浏览器从该原版战斗通过普通防御得到的低体力操作边界继续，没有设置体力、随机状态或敌方结果。PC 原生键盘防御和手机原版菜单触摸防御均自然到达失败。PC 验证无手动存档、取消重开、确认后新对局及原版新游戏，随后可以继续点击新剧情页面；手机在红色渐变中保存退出，继续失败演出后实际读取此前仍可战斗的手动基准，完整公开边界一致，并可继续触摸选招。两端全屏保留，画布为 1280×800／320×200，截图已复核。实际按键／菜单提交次数记录在 `.local/pal/game-over-board-browser/result.json`。

公共 HTTP／SQLite 另通过失败退出保护、本人权限、初始化前后的身份、数据库失败和重复重开请求；原有手动读档、个人保存、加载、即时流和保留旧核心回归通过。这里的重新开始是未结算失败旅程的新对局，不撤销已经结算的结果。

未声称新核心已重跑全部剧情，盖罗娇等允许战败继续的后段仍须随真实新游戏链路回归。失败标记来自原版 `PAL_FadeToRed`，没有把所有战斗失败改成死亡；仅全队当前体力为零时保留既有手动基准，当前脚本仍按原版续行。开发剧情仍为 23/24，原版难度验收仍为 4/24。

## 复现

```sh
. scripts/env.sh
python3 -B games/pal/build/native.py --game-over-fixture --reference-font --out .local/pal/runtime-candidates/game-over/reference
python3 -B games/pal/build/native.py --game-over-fixture --out .local/pal/runtime-candidates/game-over/fixture
PAL_NATIVE_WASM=.local/pal/runtime-candidates/game-over/reference/rules.wasm PAL_ORIGINAL_PIXELS=1 node games/pal/tests/game-over-native.mjs
PAL_NATIVE_WASM=.local/pal/runtime-candidates/game-over/fixture/rules.wasm node games/pal/tests/game-over-native.mjs
PAL_GAME_OVER_E2E=1 go test -v ./games/pal/engine -run '^TestOriginalGameOverScriptPaletteDialogueAndSavedReload$' -count=1 -timeout=8m
python3 -B games/pal/tests/screen-shake-browser.py --game-over
```

真实原版流程使用独立剧情构建：

```sh
. scripts/env.sh
python3 -B games/pal/build/native.py --out .local/pal/runtime-candidates/game-over/original
PAL_CAMPAIGN_VARIANT=game-over PAL_GAME_OVER_CAMPAIGN=1 go test -v ./games/pal/tests -run '^TestOriginalCampaignNaturalDefeatAndSavedRecovery$' -count=1 -timeout=35m
```

调试只允许使用同一核心已经正常取得并校验摘要的入口，增加 `PAL_GAME_OVER_CHECKPOINT=earned`；报告明确记录此续跑，不作为完整剧情段验收。重新编译后仍须校验实际核心身份，不能改写旧入口身份。

浏览器先运行 `PAL_GAME_OVER_BROWSER=1 go test -v ./internal/server -run '^TestPALOriginalGameOverBrowserFixture$' -count=1 -timeout=22m`，另一个终端执行 `python3 -B games/pal/tests/game-over-board-browser.py`。构建／浏览器命令前仍执行 `. scripts/env.sh`，实际页面使用本轮前端构建。
