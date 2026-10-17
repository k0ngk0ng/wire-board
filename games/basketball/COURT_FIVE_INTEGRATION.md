# Court 5 独立运动与无主道具发射

本次完成独立场地运动绑定和无主道具发射所需的背景列别名组件，尚未开放 Court 5 的完整运行入口、服务器或网页选项。完整比赛不能由下述原函数验证替代。

## 运动、碰撞和设备

现有原地形表为 Court 5 提供排他纵深上限 81，以及后排高台的边缘修正。设备原入口为 bank 4 `$863C`：纵深至少 72，横坐标位于 `[120, 184)` 或 `[272, 336)` 时触发对应桥面机关。持有中的球和动作 5 的携带篮筐不触发；其他对象按原对象表切换动作，人物清除跳跃阶段，音效 21 保留原顺序。已有源证据见 `COURT_TERRAIN_VERIFICATION.md` 中设备批次 `ev_4f740cca747dd4aad8894e0f9f027a601b02c4f9dfd1fa5454b37b39f44e693a`。

先列出纽约回退、边界错误、重复触发被抑制、持有对象误触发、其他对象写入丢失、共享旗标重复保存和辅助对象错误拒绝等失败场景，再增加 `court_five_binding_test.go`。`binding-before.log` 记录原绑定拒绝 Court 5；随后只扩展运动绑定及要求原运动的辅助对象分支。

`verify_court_five_motion.py` 执行 1,664 次原 `$EEFF → $8249/$863C`，每个对象 128 次，覆盖全部 13 个对象、区域边界、持有状态、已处于设备动作及下落，共产生 540 次原设备音效请求。完整运动状态、绑定提交、跳跃阶段写入、音效和地形事件均匹配。该矩阵是配置输入的原函数执行，不是 1,664 场自然比赛。

Court 5 动画原入口 `$9FEF` 已有独立源恢复。它与 Court 2 使用相同 `$06D7..$06DB` 状态布局，但图块、左右裁切和三行地址不同；不能只复制 Court 2 的图块。

## 无主道具的真实发射高度来源

桥面可把松散道具切换到动作 2。原 `$B086` 只有 owner 小于 4 时才复制人物朝向；其他 owner 保留道具自身朝向。随后 `$DFF8` 使用 `owner & 0x7f`，因此 owner `0xff` 会读取 `$0135/$0142` 作为高度字，而不是某个人物的高度。

这两个字节位于 CPU 栈页，但实际是背景列缓冲：`$0135` 是下层图块列的第 19 行，`$0142` 是上层属性列的第 2 行。原 bank 1 `$BCBF/$BDE1/$BE18` 会随背景列生成更新它们，不能设为常量 0，也不能从当前玩家或显示图片猜测。

`capture_court_five_lob_alias.py` 对既有 1,750 帧 Court 5 原参考增加只读钩子，记录 200 次写入。71 次完整 `$BC74` 返回后的字节与恢复算法全部相符，完整 RAM 与无钩子参考保持同一 SHA-256。按视频帧分组的初版验证会抓到跨帧执行中的临时属性值；`alias-capture-v2.log` 保留该失败，最终在 `$BCFE` 返回边界核验，记录为 `alias-capture-v3.log`。这段参考没有进入 `$B07E`，不宣称已观察自然无主发射。

`OriginalBackgroundLaunchHeight` 从原静态图块／属性表和最后实际生成的列、阶段恢复两个字节。`background_launch_tables.go` 是源数据提取，不在运行时读取 ROM。独立原 `$BC74` 的 1,024 组输出覆盖 8 个场地、64 列和初始化／游戏阶段，逐项匹配此组件。

`AuxiliaryLaunchContext.OwnerHeight` 为非玩家 owner 提供明确的原高度来源；缺失时仍返回错误。`probe_auxiliary_unowned_lob.py` 在原机器码中先执行 `$BC74`，再执行 owner `0xff` 的 `$B07E`，共 1,024 组。道具完整状态、保留自身朝向、目标及弧度均与 Go 匹配。这是隔离原函数链，不能称为自然比赛。

## 验证与剩余接线

```sh
source scripts/env.sh
python3 -B scripts/basketball/verify_court_five_motion.py
python3 -B scripts/basketball/extract_background_launch_alias.py
python3 -B scripts/basketball/probe_auxiliary_unowned_lob.py
.local/board-art-env/bin/python -B scripts/basketball/capture_court_five_lob_alias.py
go test -race ./games/basketball/engine -run '^(TestCourtFive.*|TestOriginalBackgroundColumnLaunchAlias|TestOriginalUnownedAuxiliaryLobFromBackgroundColumn|TestUnownedAuxiliaryLobRequiresExplicitHeight|TestOriginalAuxiliaryLob.*|TestOriginalAuxiliaryCarryHiddenAndThrowInstructions)$' -count=1 -v
```

日志位于 `.local/basketball/rea/court-five/`：`binding-{before,after,race,wasm}.log`、`alias-{before,after,race,wasm}.log`。源夹具及摘要保存在 `engine/testdata/court-five-motion*`、`background-launch-alias*`、`auxiliary-unowned-lob*`。既有完整原地形、运动、动画、人物弹射及道具调用也纳入定向回归。

完整比赛仍需以下接线及验收：

- 保存最后实际生成的背景列与阶段，或保存其确定生成的发射别名；初始化最后生成列为 47、阶段为 0。`Camera.TileColumn` 初始化值 48 是原生成循环结束值，不能直接当作最后列。
- 相机要求生成新列时，在本轮动作执行前更新该来源；不生成新列时保留它，JSON 恢复和跨节处理遵守原缓冲更新顺序。给无主道具提供该明确来源；其他非玩家 owner 仍须按原掩码读取对应来源，不能统一借用无主别名。
- 将 Court 5 开场、运行、准备及世界动作门禁接入完整原运动；动画适配读取／写回与 Court 2 相同的权威槽，但调用 Court 5 的原动画。
- 保留原种子和时钟，验证自然设备触发、无主道具发射、四种控制配置的完整两节、恢复和显示。未完成这些步骤前不开放平台选项。
