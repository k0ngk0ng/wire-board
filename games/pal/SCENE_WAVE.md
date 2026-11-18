# 原版场景波动

原版 `PAL_MakeScene` 先画地图两层，再调用 `PAL_ApplyWave`，最后绘制人物、遮挡瓦片和界面文字。场景波动必须保留这个顺序；不能对包含人物和对白的最终画面统一变形。

## 实现与版本

- 场景绘制指令 `kind: 2` 记录本次实际应用的波动强度、相位和原版有符号增量。指令排在人物与遮挡瓦片前，画面备份和 RNG 起始画面也保留它。
- 波动只影响呈现。普通场景继续导出原版碰撞和队伍运动投影，支持本地预测；预测按原版每次绘图更新相位和强度，到达停止边界后清除波动。
- 预测最多接受 3 个待确认帧。旧预测晚到或超出边界时显示当前权威画面；帧号经过 `uint32` 回绕后仍正确计算相位。
- 战斗绘制图继续在原版指定的离屏表面应用波动，与普通场景共用逐行移位实现。未修改原始剧情脚本、地图、奖励或角色数值。

本次核心保存在独立候选目录，未覆盖此前已验收核心。已有存档按[版本保留说明](NATIVE_VERSIONS.md)选择真实旧核心，不改写存档构建标识。旧核心缺少场景波动指令，保留原有表现；新核心须从新游戏重跑到真实血池，不能用旧内存冒充新构建验收。

| 候选用途 | Wasm SHA-256 | 路径 |
| --- | --- | --- |
| 原版数值 | `0eb6fde6f2d5e738f36edec649f7d9bf0714f4fbe6caf83c011a1a0a73d6ca47` | `.local/pal/runtime-candidates/scene-wave/original/rules.wasm` |
| 仅开发加血 | `b1b85663da36e2a28a3fd0ee6ce33b8b8d08877a391ee05bc758f01fd5f116fb` | `.local/pal/runtime-candidates/scene-wave/hp-9999/rules.wasm` |
| 原版 C 像素参考 | `1517cedbb519e52e8a10e324163a3cf8892aedb5bb4158ce542df1de6e103841` | `.local/pal/runtime-candidates/scene-wave/reference/rules.wasm` |

## 验证范围

先记录失败，再修改实现：首次缺少波动时，原版参考与页面有 34,942 个像素不同；旧预测覆盖新权威画面时，有 38,915 个像素不同。

修复后，99 组原版 C 画面在 PC、320px 手机竖屏和手机横屏中逐像素一致，共 297 次比较。用例覆盖 32 相位循环、有符号增量、停止边界、人物与文字层级、带波动的不可变画面备份、移动预测、旧预测、预测上限和帧号回绕。Go 运行器另验证全新实例续档、波动参数校验和画面备份；战斗波动、普通队伍运动及保留旧核心的回归通过。

真实剧情回归使用 `PAL_CAMPAIGN_VARIANT=scene-wave`，已从新游戏通过第 8 段，到达原版第 60 场景、第 76 张地图。未使用场景或波动探针：原始脚本 12184 设置的二级波动、全部 32 相位、公开运动投影，以及离线 30 天后全新运行器续档均通过。原版三人队伍、现金与旧输入拒绝保持正确。

实际游戏页面另通过原生手机阅读 4 页、三人归队、键鼠／触屏移动、陌生账号拒绝、个人保存续玩和 320×200／1280×800 最大比例画布。实际 Canvas 与带波动的当前权威画面一致，并与同一地图／人物去掉波动后的画面不同。结果位于 `.local/pal/solo-blood-pool-wave-browser/`，阶段摘要为 `.local/pal/verification/scene-wave-stage.json`。这些功能结果不增加主线 `a/24`。

候选第 9 段曾在赤鬼王战败，原失败输入与第 1107 步的公开选招、目标、体力轨迹保留于不可变 `scene-wave-stage/` 归档。现已修复全队合击占用其他施法者行动的问题，同一原始存档用 758 条普通托管指令获胜；灵儿选定进攻目标时保存、离线 7 天后全新运行器恢复，以及旧输入拒绝通过。完整第 9 段和土灵珠脱离已独立重跑，鬼阴山第 10 段、扬州第 11 段、蛤蟆山第 12 段、京城尚书府第 13 段、彩依与毒娘子第 14 段、蜀山与入塔第 15 段、锁妖塔前半第 16 段及实际 PC／手机页面也已通过。当前开发候选进度为 **16/24**，正式原版难度仍为 **4/24**；未增加体力以外的开发特权，未替换默认核心。

本轮摘要与归档分别为 `.local/pal/verification/red-king-bot-stage.json`、`red-king-bot-stage/` 和 `ghost-mountain-stage.json`、`development-health-stage-10/`。早期 `scene-wave-stage.json` 继续记录当时第 9 段失败的事实，不随当前结果改写。

后续扬州阶段见 `.local/pal/verification/yangzhou-stage.json` 与 `development-health-stage-11/`；该段不将可选赏金或隐藏物品候选计作实测完成。

蛤蟆山、五毒珠、原版苗人自动战斗及抵京见 `.local/pal/verification/toad-mountain-stage.json` 与 `development-health-stage-12/`。该段同样不替代正式原版难度和发布验收。

京城尚书府、林天南试剑、府内求助、原版法事及月如离开见 `.local/pal/verification/capital-manor-stage.json` 与 `development-health-stage-13/`。法事道具、可选 3000 文箱子和隐藏绣花鞋的实际领取／续档另有核对；十里香候选未计作实测完成。

彩依显形、原版两场剧情战斗、三段回忆、牺牲及抵达蜀山见 `.local/pal/verification/caiyi-forest-stage.json` 与 `development-health-stage-14/`。雷灵珠此时仅被酒剑仙发现，尚未赠予玩家，保留原版蜀山交付时机。

蜀山找回玉佛珠、酒剑仙实际授艺赠物、月如同行及入塔见 `.local/pal/verification/shu-tower-stage.json` 与 `development-health-stage-15/`。手机对白和移动使用直接画面触控，另有原版仙术菜单及奖励防重复核对。

锁妖塔前半的原版塔内人物、剧情战斗、解谜和三处机关／10 个桥段见 `.local/pal/verification/tower-upper-stage.json` 与 `development-health-stage-16/`。109 次阅读、61 场战斗、188 次全新运行器续档及同次书中仙 PC／手机页面通过；独立桥道层级触控和蜀山 7 处隐藏物品另有证据。核心摘要未变化，未因此通过原版难度或全程自主探索。

十年前余杭从第 20 段实际终点完整重跑通过：71 次阅读、8 场正常胜利、88 次全新运行器续档，包含手绢使用、木剑换水灵珠和现世阿奴交付蛋壳。同次灵月宫主与交换两组 PC／手机共 17 项检查通过，手机另在现世重逢对白中保存并恢复完整公开边界。摘要为 `.local/pal/verification/old-yuhang-stage.json`，复现见[十年前余杭与水灵珠验收](OLD_YUHANG_TESTING.md)。开发流程为 21/24，原版难度仍为 4/24；核心、原版资源与人物规则未因本段更改。

圣药、忆如出生与试炼窟已从第 21 段实际终点连续完成，前半与收集通过精确父档摘要衔接，均未使用调试检查点。83 次阅读、48 场正常胜利、1 场原版允许的盖罗娇战败续行及 232 次新运行器续档通过；实际收集 37 只傀儡虫，交付 36 后保留 1，灵儿／逍遥／阿奴按原版顺序归队。同次两组 PC／手机共 16 项检查、19 次手机阅读通过。摘要为 `.local/pal/verification/sacred-medicine-stage.json`，复现见[圣药、忆如出生与试炼窟验收](SACRED_MEDICINE_TESTING.md)。开发流程为 22/24，原版难度仍为 4/24，本段没有修改生产规则或策略。

祭母、圣物与换装、五灵珠祭坛、祈雨及地魔兽已从第 22 段实际终点完整重跑通过：28 次阅读、10 场正常胜利、101 次全新运行器续档。同次三组 PC／手机共 20 项检查通过，两端均观察到六处祈雨／灾情／庆典场景，手机在演出中保存恢复完整公开边界。摘要为 `.local/pal/verification/rain-prayer-stage.json`，复现见[灵儿归来与祭天祈雨验收](PRAYER_TESTING.md)。开发流程为 23/24，原版难度仍为 4/24。物理震屏尚未还原，演出页面可见不代表该项已通过；本段没有修改生产规则或策略。

持续波动或菜单高亮须比较真实冻结时与恢复后尚未推进时的画面；加载就绪后的在线动画另验。候选页面夹具只读记录这两个公开边界，逐项比较场景、队伍、物品、菜单及完整绘制指令，不抹去波动或高亮相位。平台规则见[存档与续玩规范](../../rules/saved-games.md)。

## 复现

仓库根目录执行，先加载工作区缓存配置：

```sh
. scripts/env.sh
python3 -B games/pal/build/native.py --out .local/pal/runtime-candidates/scene-wave/original
python3 -B games/pal/build/native.py --reference-font --out .local/pal/runtime-candidates/scene-wave/reference
python3 -B games/pal/build/native.py --test-battle-hp 9999 --out .local/pal/runtime-candidates/scene-wave/hp-9999
PAL_NATIVE_WASM=.local/pal/runtime-candidates/scene-wave/reference/rules.wasm PAL_ORIGINAL_PIXELS=1 node games/pal/tests/scene-wave-native.mjs
PAL_NATIVE_WASM=.local/pal/runtime-candidates/scene-wave/original/rules.wasm node games/pal/tests/scene-wave-native.mjs
PAL_WAVE_CORE=.local/pal/runtime-candidates/scene-wave/original/rules.wasm go test -race -v ./games/pal/engine -run '^TestOriginalSceneWaveProjectionAndSavedContinuation$' -count=1 -timeout=4m
python3 -B games/pal/tests/visual-browser.py --scene-wave
```

剧情回归沿用[开发测试说明](DEVELOPMENT_TESTING.md)的完整顺序，额外设置 `PAL_CAMPAIGN_VARIANT=scene-wave`。从新游戏通过将军墓后执行真实血池检查：

```sh
PAL_CAMPAIGN_VARIANT=scene-wave PAL_DEVELOPMENT_HEALTH=9999 go test -v ./games/pal/tests -run '^TestCandidateBloodPoolActualMapWaveAndSavedField$' -count=1 -timeout=4m
# 从不可变归档中的原失败输入回归，不使用后续重跑覆盖的检查点
PAL_CAMPAIGN_VARIANT=scene-wave PAL_DEVELOPMENT_HEALTH=9999 PAL_RED_KING_BOT=1 PAL_RED_KING_SOURCE=archived go test -v ./games/pal/tests -run '^TestOriginalRedGhostKingBotFromSavedBattle$' -count=1 -timeout=8m
```

真实页面先构建 `web`，在一个终端启动 `PAL_BOARD_BROWSER=1 PAL_BOARD_ENTRY=solo-blood-pool-wave go test -v ./internal/server -run '^TestPALBoardBrowserFixture$' -count=1 -timeout=12m`；另一个已加载环境的终端执行 `python3 -B games/pal/tests/solo-board-browser.py --blood-pool-wave`。该入口读取候选核心实际剧情生成的存档，验证单人阅读、队伍归队、波动画面、原生键鼠／触屏、个人保存及最大比例画布。
