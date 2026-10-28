# 雪地接缝遮挡桥面的视觉恢复

双人雪地真实画面中的静态接缝现可参与地形遮挡恢复。仅完整、精确匹配的 `Class=scenery, Sprite=196, Kind=0` 不透明像素进入独立 `terrainOcclusion`，在公开战斗阶段与已有精确玩家遮罩取并集。移动石头识别继续只接收 `playerOcclusion`，不会把接缝当作玩家。

生产改动限于 `bot_vision.go` 的接线与 `bot_vision_terrain.go` 的公共恢复函数提取。原剩余像素算法、阈值、电脑策略、导航和 Session 均未修改。

## 真实像素证据

输入来自 `.local/contra-bot/diagnostic-coop-snow-next/` 的两张原始公开 PNG。测试只读取最终 RGBA、公开战斗阶段、席位数和固定图形目录，不读取保存的 Session、OAM、逻辑实体或隐藏槽位。

| 图像 | 完整接缝匹配 | 目标接缝 |
| --- | --- | --- |
| `first-stall/frame-119968.png` | 2 个，包含左侧接缝 | 锚点 `(183,176)` |
| `terminal-window/frame-129000.png` | 1 个 | 锚点 `(183,176)` |

目标接缝模板矩形为 `(181,177,4,14)`，28 个不透明像素全部匹配。两图均完整识别出脚部附近的红色玩家。仅忽略接缝仍不足以恢复被玩家同时遮挡的格子；精确玩家与接缝的不透明像素并集足以让原算法唯一确定材质。

两图中的公开坐标 `(168,180)`、`(176,180)` 属于同一个地形格，原先均为 Unknown，修复后均为 Known Platform。远处 `(208,180)`、`(224,180)` 继续为 Known Platform。破坏目标接缝的一个必需像素后，完整接缝匹配消失，目标格继续为 Unknown。

可移植 PNG 保存在 `games/contra/engine/testdata/bot-snow-scenery-119968.png` 和 `bot-snow-scenery-129000.png`。前者 SHA-256 为 `e8612e26ede0a1d9d25e0cdceba989c2acd7a16af16f960a1d2115c528c32587`，后者为 `070f9d13df91f0ba59393505427fbb626267001c7d42cf734335e8659d3214c5`。[来源核验](evidence/bot-scenery-terrain-source.json) 保存与原始 PNG 的逐字节一致性，以及地形恢复函数体和移动石头 helper 未变的摘要核验。

## 保守边界

地形网格必须已从画面识别出来，只恢复仍未知的格子。遮罩只含精确匹配的不透明像素，透明孔洞仍参与验证；不使用矩形覆盖。单格遮挡超过 128 像素时拒绝恢复。其余像素必须至少含两种颜色、至少 8 个非主色像素，并在整个固定地形目录中具有一致的材质解释；没有匹配、颜色冲突或材质歧义时均保留 Unknown。

没有精确玩家时，完整接缝可以独立提供地形遮罩，但不能提供玩家身份。未知图案、其他 scenery 编号、敌人、玩家别名和不完整接缝均不能提供此遮罩；非战斗阶段不启用接缝恢复。

## 测试与复现

失败场景及测试先于实现。真实 PNG 的两个正例、合成无玩家正例及 Water 正例在实现前失败，负例通过，见 [先验失败记录](evidence/bot-scenery-terrain-red.jsonl)。Water 正例验证恢复使用目录中的实际材质，并非将 Unknown 一律改为 Platform。

聚焦检查通过 3 项顶层测试、23 条测试及子测试记录，见 [聚焦结果](evidence/bot-scenery-terrain-green.jsonl)。边界覆盖接缝损坏、透明孔洞、未遮挡像素冲突、材质歧义、遮挡过多、单色残片、错误类别／编号、玩家别名、非战斗阶段，以及 scenery 不得进入移动石头的玩家遮罩。

```sh
python3 -B scripts/contra/verify_scenery_terrain.py
```

此命令运行绑定源码的视觉 race 回归，覆盖本次测试、已有玩家遮挡地形、移动石头、通用视觉以及雪地／后关配方。本次通过 43 项顶层测试、101 条测试及子测试记录，零失败；运行期间及收尾复核时，绑定源码均未变化。测试日志、通过数量和源码摘要分别保存在 [race 日志](evidence/bot-scenery-terrain-race.jsonl) 与 [验证清单](evidence/bot-scenery-terrain-verification.json)。

本验收证明两张真实画面的地形识别和遮罩边界；闭环导航是否继续推进由对应策略接续验证。测试不修改生命、位置、胜负或游戏状态，也不增加标准命数通关要求。

清理范围为两份被完整红测替代的早期日志和本任务空临时目录 `.local/contra-scenery-terrain/`。原诊断目录、正式 PNG 夹具、完整红测、聚焦结果、绑定 race 记录及共享缓存均保留；删除前摘要见来源核验中的清理清单。
