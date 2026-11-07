# 基地、基地 Boss 与瀑布玩家动作

`Player.Step` 已支持日版的室外横向、瀑布纵向、基地和基地 Boss 场景，不再将这些合法场景拒绝或代入横版行为。新增字段为 `Player.IndoorAnchorX/Y` 与只读输入 `PlayerStepContext.ScreenCleared`；`OutdoorWorld.IndoorScroll` 继续保存唯一的推进状态。

实现前先建立模式差分测试，保存缺少字段／场景未恢复的 red 结果，再编写规则。验证接入真实 `WeaponSystem.Fire`，原版 after 状态只用于比较。

| 验证 | 结果 |
| --- | --- |
| 原版玩家场景调用 | 9,776 次，46 个玩家字段、同伴、世界、完整 16 槽弹丸池和音效序列全部匹配 |
| 场景分布 | 瀑布 2,278 次；基地 3,642 次；基地 Boss 3,856 次 |
| 电击／推进／强制跳跃 | 主场景记录分别覆盖 390／536／4 次进入状态 |
| 竖向镜头 | 198 次滚屏标志；32 个调用中原版在跳跃与 X 更新阶段分别调用镜头逻辑 |
| 合作推进拒绝 | 受控原版调用覆盖同伴入场、死亡、等待、跳跃四个拒绝条件；另覆盖同伴已出局及已推进 |
| Boss 生命周期 | 临时研究屏障释放后，由原版敌人造成 606 次死亡推进及 4 次重生，未设置玩家死亡或复活结果 |
| 整帧接入 | 世界模块新增 3,719 个 `$D296` 场景阶段通过，覆盖基地 1,815、瀑布 1,139、Boss 765；没有扩大为完整关卡通过 |
| 重复采样 | 场景、边界、Boss 生命周期分别独立录制两次，调用状态与碰撞窗口一致 |

测试夹具：[original-player-modes.json.gz](../../../games/contra/engine/testdata/original-player-modes.json.gz)，SHA-256 `13844ca60439831b7a5934520e5b4d9a3d73b15ab6d3285a5a995264b3504c4e`。[来源清单](../../../games/contra/engine/testdata/original-player-modes-source.json) 分别标明三组原版证据及控制设置。

## 原版初始化与控制设置

基地／瀑布初态使用世界任务的 `stage-{2,3,4}-v2.fcs`。这些初态是在原版关卡初始化 `$CC5F` 处单次设置关号后，由原程序完整初始化生成；它们不是自然完成前序关卡的存档。普通模式采样在该初态后只提交控制器输入，实际击毁基地目标、向前推进、电击、跳跃和死亡均由原版程序决定。

Boss 入口另明确设置最后一个基地房间号、原版实际使用的清屏值 `$80`，并临时启用屏障，让原版完成四段房间前进及 Boss 场景加载；没有直接改写 `SceneType`。第二组生命周期采样复现这段初始化的完整 RAM 前缀，再一次性解除屏障。之后不修改 HP、生命、玩家状态、敌人状态或结果，由原版敌人触发死亡和重生。

12 个合作／转场边界案例只在玩家函数入口设置声明的输入字段，记录实际函数返回后恢复整份 RAM。它们单独标记为受控调用，不冒充自然通关或联机验收。

## 场景差异

| 项目 | 室外／瀑布 | 基地 | 基地 Boss |
| --- | --- | --- | --- |
| `SceneType` | 0 | 1 | `$80` |
| 脚底判定 | 地形窗口，锚点向下 16 像素 | Y > 160 为地面 | Y > 200 为地面 |
| 落地锚点 | 按滚动及 16 像素格吸附 | Y = 168 | Y = 201 |
| 起跳速度 | `$FBF0` | `$FC90` | `$FBF0` |
| 左／右限制表值 | 26／230 | 48／208 | 32／224 |
| 死亡水平速度 | 按朝向向后 ±1 | 0 | 0 |
| 死亡动画序列 | 4 | 6 | 4，但使用 Boss 专用选择表 |

边界比较保持原版条件，例如 X 等于左表值仍可向左一步。出生位置按关号与场景选择；瀑布初始 Y=80，基地／Boss Y=96，其他横向室外 Y=32。角色进入与死亡规则没有借用横版硬编码位置。

## 基地前进与电击

没有清屏时按上将电击计时设为 48，当前行动停顿并产生原版 `$34` 音效。正在电击时先减计时；如果另一名玩家已经清屏，立即解除电击，但不会在同一分支又开始前进。

清屏后按上，只有同伴已经出局，或同伴正常且未跳跃，才能启动前进。启动按原版顺序处理第二名角色、第一名角色；即使某角色已出局，也保留原版对内部角色槽的字段更新。已经在前进的角色不会再次重设锚点或速度。

前进速度为 Y `−176/256`，X `2 × (128−X)/256`，分别使用独立的室内 Y 速度和 X 定点字段；进入时保留既有小数累计。启动者本帧初始化前进，但不立即执行室内 Y 位移；后处理的同伴可在同帧执行位移，因此不能人为对齐两人的阶段。

`IndoorScroll>=2` 时，仅在角色未跳跃时恢复记录的锚点；恢复只改像素位置，保留小数。强制跳跃先清强制标志及前进标志，再通过同样的跳跃 guard；已经在空中时不重置 Y 速度，也不错误恢复锚点。受控调用专门覆盖这两个日版条件。

基地向前行走的动画使用递减计时和每次 11 tick 的步伐周期，并产生 `$0B` 音效；它与横向跑动的 8 tick 动画周期不同。死亡姿态则以特殊计时是否达到 27 为界选择精灵。

## 瀑布镜头与原版关联

上升到屏幕 Y<80 时，原版可以把 Y 位移转为镜头位移，只推进小数累计而保持玩家像素位置。跳跃阶段会查询镜头，随后向右的 X 更新也可能再次查询；本轮实际观察到 32 个双调用案例。实现保留两次调用，未按“每帧一次”去重。

竖向自动滚屏使角色 Y 加一；同伴随镜头移动的修正仍由世界帧模块负责。竖屏 Down+A 在正常滚动时可直接穿下；进入 Boss 自动滚屏标记后，重新采用下方支撑检查。

Boss 的瞄准表选择读取第一名角色的跳跃标志，即使当前处理的是第二名角色。本轮有 61 次第二名角色未跳、第一名正在跳的记录，已保留这种原版跨角色关联，没有按直觉改成读取自己。

## 代码与证据

新增规则位于 [player_indoor.go](../../../games/contra/engine/player_indoor.go)，共同动作、生命周期和动画分别继续在 `player.go`、`player_lifecycle.go`、`player_animation.go` 维护。只有已恢复的场景值 0、1、`$80` 及合法横／竖组合通过校验；未知组合仍返回错误。

原版证据分别保留：

- [场景采样](evidence/static-player-modes.json.gz)
- [合作与转场边界](evidence/static-player-mode-boundaries.json.gz)
- [Boss 死亡与重生](evidence/static-player-boss-lifecycle.json.gz)
- [固定区 REA](evidence/static-modes-rea.json.gz)
- [基地动画 REA](evidence/static-mode-sprites-rea.json.gz)

从保留证据重新生成并验证 Go 夹具：

```sh
python3 -B scripts/contra/static_export_player_modes.py
source scripts/env.sh
go test ./games/contra/engine -run '^TestOriginalPlayerBaseBossAndWaterfallModes$' -count=1 -v
```

重新分析和采样时使用新的运行标签，避免覆盖已验证记录：

```sh
python3 -B scripts/contra/static_prepare_player_modes.py
node scripts/contra/static_rea.mjs .local/contra-static/modes-spec.json
node scripts/contra/static_rea.mjs .local/contra-static/mode-sprites-spec.json
.local/board-art-env/bin/python -B scripts/contra/static_capture_player_modes.py --label <新的标签>
.local/board-art-env/bin/python -B scripts/contra/static_capture_mode_boundaries.py --label <新的标签>
.local/board-art-env/bin/python -B scripts/contra/static_capture_boss_lifecycle.py --label <新的标签>
```

上述采样依赖世界任务已核验的关卡初态；Boss 生命周期脚本还核对构造过程与保留的 Boss 入口 RAM 前缀。通过范围是玩家和已接入的战斗阶段；完整关卡推进、Boss／敌人、动态地形、结算、服务器恢复、网络及浏览器操作仍按各自证据验收。
