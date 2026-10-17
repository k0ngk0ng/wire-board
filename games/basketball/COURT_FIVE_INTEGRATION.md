# Court 5 独立运动与无主道具发射

Court 5 已接入 Go 运行时，包括原场地运动、设备、背景列发射来源与动画。正常时钟的四种控制配置已完成 Native、race 与 Wasm 验证，准备阶段新增的 16 种配置也已完成 Native 验证。服务器和网页选项由后续平台验收控制，本轮不开放。

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

## 运行时接线

`WorldLoopState.LaunchColumn` 保存最后实际生成的背景列及阶段，初始化为 47／0；`Camera.TileColumn` 的 48 是初始化循环结束值。相机产生新列时，在准备和动作之前提交新来源，没有生成新列则保留旧值。跨节采用重新初始化的来源，JSON 恢复校验缺失、越界列与无效阶段；通过新指针提交避免污染输入快照。

Court 5 开场、运动、准备与动作门禁已接入。动画复用 Court 2 的权威状态槽，调用 Court 5 原算法，3,186 条源动画调用／探针均匹配。无主道具从保存的列和阶段获取原高度；没有背景来源仍拒绝发射，不以玩家高度或零值代替。

## 完整比赛暴露的附着动作缺口

暂停前保留的 `runtime-native.jsonl` 和本轮 `resume-native-before.log` 均在全电脑配置第 15,521 视频帧、第 5,158 逻辑更新失败。失败前人物为骑乘，输入阶段把动作改为普通跑步 `02`；原运动依然使用附着支撑，不能强行解除附着。修复后第 15,524 视频帧进一步暴露普通下落 `0F`。原 `$9749` 直接调用 `$EEFF`，因此下落仅在完整原运动绑定能提供支撑物时开放。

同轮恢复了已接受的骑乘跳跃：原 `$8938/$8B67` 在运动前清除附着；Kind 12 支撑物若自身未附着，地面时切动作 4 并继承人物速度／重力，空中时切动作 12。跳跃真正发起时保留原音效 23。骑乘位置使用实际支撑物的身体、姿态与种类，涵盖 Kind 14 道具，不再只允许人物支撑。

`verify_rider_jump.py` 和 `rider_jump_test.go` 先记录失败，再恢复生产分支。最终源夹具涵盖 288 条骑乘跳跃调用、72 条 Kind 10～15 支撑调用、48 条附着跑步／下落调用，以及 8 条一次初始化的跳跃落地连续链（136 次调用）。逐项比较完整世界状态及音效，连续链同时验证 JSON 恢复。源入口为固定 `$E165/$E18B → $E168`，只把 `$FBCC` 作为音效请求出口，不在 Go 运行时加入 CPU 模拟。

早版探针沿用了未初始化场景中 6 个篮筐的 owner 0，因而错误地同时构造了「抱住全部篮筐」的无关状态。原夹具另存 `rider-jump-uninitialized-ownership.json.gz`，失败日志保留；最终探针显式使用正常篮筐 owner 7～12。该修正仅改变人工配置的探针，不改变自然完整比赛的种子、初始状态或时钟。

## 四种正常时钟配置

全部采用种子 `0x12345678`，每种冷启动两次，保留原两节各三分钟及中场确认，恢复后比较下一视频帧、声音和显示。下表记录 Native 成功结果，完整轨迹哈希保存在日志中。

| 配置 | 视频帧 | 逻辑更新 | 每次恢复 | 比分 |
| --- | ---: | ---: | ---: | --- |
| 原默认单人 | 22,574 | 7,496 | 85 | 30∶57 |
| 全电脑 | 22,742 | 7,552 | 85 | 32∶36 |
| 双人合作 | 22,373 | 7,429 | 84 | 0∶85 |
| 双人对战 | 22,385 | 7,433 | 84 | 39∶6 |

复跑日志为 `.local/basketball/rea/court-five/resume-{native,race,wasm}-final.log`；仅日志终态 `PASS` 才表示对应运行完成。定向回归包含旧 1,998 次世界更新、26,093 次动作调用、原自由人物、骑乘、下落和完整运动绑定，不运行其他游戏。

独立观察程序 `observe-runtime.go` 从冷启动以同样配置运行完整比赛，`natural-runtime-events*.json` 记录自然 Go 运行中的设备音效 21、背景列更新与动画补丁。四配置均观察到设备和背景列变化，但没有观察到无主道具发射；无主发射目前由原 1,024 组函数链和世界来源顺序测试覆盖，不能据此宣称自然比赛或浏览器已覆盖。

准备阶段增量 16 配置仅选择 Court 5 的四种球员排序与四种控制配置，不重跑已验证的 80 配置。首轮 `preparation-incremental-native.log` 为 13 通过、3 失败，保留全部前态；修复后 `preparation-failed-three-after-launch.log` 覆盖原失败的 3 个全电脑配置，均冷跑两遍并通过。平台真实浏览器显示、准备阶段权限与选择入口另行验收；本记录不证明完整 32 角色、其他未开放场地、战役或任意真人技术已完成。


## 准备阶段增量配置恢复的双人配合

原失败的 roster 1／2／3 全电脑配置依次在视频帧 5,369／7,589／19,916 进入未恢复的动作 `07`。原 `$92F9` 让支撑人物停止水平运动、朝向当前球持有者，播放准备动画；游标到 1 时把已链接的队友改为动作 `3F`，并清除链接。没有队友时仍保留原写入：`$0512 + FF = $0611`，因此把人物 0 的 endurance 改为 63，而不是跳过这个原行为。附着中的支撑人物动画结束后回到骑乘，未附着者回到空闲。

恢复 `07` 后，原三个配置分别推进到视频帧 5,381／7,601／19,928，暴露后续动作 `3F`。它与已有 `3E` 共用 `$9E3D`，但使用当前球的整数坐标和高度作为目标、弧高 32，直接调用 `$FD91`；不执行 `3E` 的篮筐与装备目标覆盖。实现扩展 `LaunchedPlayerContext.BallPosition` 和原发射组件，共用已有运动算法，保留随机数、解绑、音效 6、球员 1 横坐标别名及落地请求。

`verify_rider_lift.py` 恢复 576 条原 `07` 调用和 16 条连续动画（176 次调用）；`verify_rider_launch.py` 恢复 288 条原 `3F` 调用和 4 条一次初始化的双角色「支撑 `07` → 骑乘 → `3F` 发射 → 落地」连续链（384 次调用）。测试先失败记录为 `rider-{lift,launch}-before.log`，源调用及旧 `3E`／发射上下文回归的 race、Wasm 结果为 `rider-complete-{race,wasm}.log`。

新增的三条全电脑完整比赛结果如下，均保留原种子、时钟和中场确认：

| 球员排序 | 视频帧 | 逻辑更新 | 恢复次数 | 比分 |
| --- | ---: | ---: | ---: | --- |
| roster 1 | 22,373 | 7,429 | 84 | 50∶30 |
| roster 2 | 22,403 | 7,439 | 84 | 49∶53 |
| roster 3 | 22,514 | 7,476 | 85 | 49∶45 |

三条新路径的完整赛 race／Wasm 单独记录在 `preparation-failed-three-{race,wasm}.log`，不扩大到先前通过的其余 13 配置或其他游戏。原 `07` 和 `3F` 首失败前态分别保存为 `failure-roster{1,2,3}-all-computer.json` 与 `failure-roster{1,2,3}-all-computer-after-lift.json`。
