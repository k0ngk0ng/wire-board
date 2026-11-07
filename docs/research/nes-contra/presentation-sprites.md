# 完整场景精灵组装

本项将 26 个逻辑对象、原版生命 HUD 和场景粒子组合成下一次呈现提交的 256 字节 OAM。它不读取原版 after 状态来生成结果，也不把单个元精灵差分等同于整个场景组装。

## 实现前失败场景

- 逻辑对象排序、轮转起点或 `$C4` 步长错误，遮挡及闪烁顺序改变。
- 满 OAM 提前或延后退出，元精灵跨剩余容量时被人为截断。
- 隐藏槽只应修改 Y，却清空其他历史字节；跨帧状态因此改变。
- 线性模式误复用 HUD／雪粒路径，或沿用轮转模式的容量条件。
- 单人、双人、Demo、Game Over、零余命和四枚以上生命显示错误。
- 把背景绘制的街机分数虚构成 OAM 数字，或丢失真实逻辑对象中的文字／弹丸。
- 第 5 关雪粒的持久位置、方向、裁切与容量推进错误。
- 共享元精灵指针写入 `$06` 被忽略、错认为属性或无变化时强行清零，改变下一阶段敌人生成输入。
- 开局、暂停、模式切换、图形队列变化或恢复时跳过原版延后一帧提交。

先保存完整 `$919F` 调用的语义 before／after、OAM 和 `$06` 写入来源，再运行失败测试，之后编写独立 Go 组装函数。角色、敌人、弹丸的状态来自 World；得分、生命和规则推进仍由各自规则层负责。

## 已实现接口

[`presentation_sprites.go`](../../../games/contra/engine/presentation_sprites.go) 提供完整场景组装：

```go
state, err := NewSpriteCompositionState()
result, err := state.Compose(SpriteCompositionInput{
    Objects: objects, // 0..9：CombatFrame.Display；10..25：EnemySystem.Slots
    Flicker: true,
    HUD: true,
    TwoPlayers: true,
    Level: level, FrameScroll: frameScroll, SnowControl: snowControl,
    Lives: lives, GameOver: gameOver,
})
```

`SpriteCompositionState` 保存完整 `OAM [256]uint8`、轮转起点 `Rotation` 和 16 个 `SnowParticle{X,Y,Flags}`。`Compose` 在私有副本上完成整个阶段后提交，错误不产生部分修改。新状态隐藏所有槽位；切换关卡时，可按原版 `$A816` 调用 `ResetSnow()`，保留已有 OAM 和轮转起点。

对象直接使用 World 的逻辑精灵代码、位置和属性。不要再按敌人 Routine／HP 或玩家存活状态过滤；原版只检查代码是否为零，是否显示由对应规则层已经决定。

`SpriteCompositionResult` 包含 `GenerationBiasChanged` 与 `GenerationBiasValue`。共享部件跳转在 `$92D9` 写的是共享指针低字节，例如 `$960C` 对应 `$0C`，不是调色属性。即使最终值与进入值相同，也报告一次兼容写入事件。没有共享跳转时 `Changed=false`，World 保留前序阶段已有的值。

这个兼容事件应按当前 tick 的阶段顺序立即应用；新 OAM 则留到下一次呈现边界提交。组装函数没有提交画面、推进物理、修改生命／积分或清空图形队列。

## 原版顺序与边界

- 轮转模式读取 `$07EB & 1`，将 `$35` 增加 `$4C`，按 `$C4` 步长写 OAM。HUD 先写，逻辑对象按 25 到 0 的顺序处理，然后才考虑雪粒与未用槽位。
- 内层元精灵完成全部部件后，外层才检查剩余计数。最后一个大元精灵可以跨过剩余容量并绕回已使用槽位；硬件输出仍为 64 个四字节条目，不能人为提前截断内层循环。
- 隐藏未用槽位只把 Y 写成 `$F4`，其他三个字节保留。线性模式按 0 到 25 处理对象、步长 4，不绘制常规 HUD／雪粒，也不修改轮转起点；最后的隐藏循环即使游标为 0 也会执行一整圈。
- `$936E` 绘制生命勋章或 GAME OVER 字形。单人／双人、Demo、Game Over bit 0 和零／超上限余命均按原版处理。第二席 X 偏移是 `$B0`，因为 `ADC #$AF` 还使用了先前的 Carry。
- `$936E` 不读取 `ArcadeScores`。街机分数由背景图形路径绘制，不能在 OAM 中另造一套数字 HUD。

## 雪粒状态

第 5 关的 `$91F8` 从雪粒 15 向 0 处理，只在逻辑对象后还有槽位时执行。每个雪粒仍消耗一个 OAM 槽，即使其可见位为零；容量耗尽后，其余雪粒保持原位置。

标志 bit 1 选择 `(Y 增量 3／4，X 减量 4／5)`。X 减量还要加当前滚动量并按字节回绕。X 下溢会进入 `$926A`，使用 `SnowControl & 1` 更新可见位，同时将本次 Y 增量也改为 0／1。这是原版局部变量复用产生的行为，不能简化成恒速粒子。Y 更新后的字节值达到 `$EE` 时归零并重新取可见位。

`SnowControl` 来自 World 的 `$CE59` 程序：根据 `$CE87` 的屏幕位置序列更新 `$5A`，并设置影响 CHR 动画的 `$5D`。组装只消费该结果，不擅自推进天气程序。原版 `$A816` 对三个 16 字节表逆序复制的初始位置与标志已保留。

## 原版差分结果

| 范围 | 结果 |
| --- | --- |
| 自然完整组装 | 10 组、4,868 次；含单／双人开机、阶段及 HUD 切换、八关各 256 帧 |
| 受控呈现边界 | 20 组、39 次；含 26 对象、满 OAM、线性模式、翻转裁切、HUD、雪粒和共享指针 |
| 总原版对照 | 4,907 次完整 OAM、轮转和雪粒状态匹配 |
| 连续状态推进 | 2,087 次只从首个 before 初始化，后续使用 Go 上一帧结果，全部匹配 |
| 共享指针兼容写入 | 原版共 1,314 次实际写入；793 次组装调用产生该事件，其中 615 次最终值与进入值相同 |
| 重复原版录制 | 十组自然输入的完整 RAM 与输出记录逐字节一致 |
| Go 检查 | 原版差分、恢复／初始化、无效输入原子拒绝及定向 race 检查通过 |

自然样本的所有组装都在同一视频帧内完成。人为构造的线性超量场景跨越视频帧；记录器仅推进到已进入的调用结束，并记录额外排空帧，不再采样新调用。这只验证完整函数的输出，不声称该压力输入是自然玩法或用它替代 World 的视频时序。

原版样本中的图形队列长度覆盖 0、12、13、24、33、36、48、49、57、61、86。组装没有读取或清空队列，因此队列提交由 World 的相应阶段负责。当前没有接入 Canvas、浏览器 UI 或音频。

## 来源与复查

- 原版完整夹具：[`original-presentation-sprites.json.gz`](../../../games/contra/engine/testdata/original-presentation-sprites.json.gz)，旁边的 source JSON 绑定 ROM、夹具和运行时数据 SHA。
- 独立运行数据：[`presentation_sprites_data.json`](../../../games/contra/engine/presentation_sprites_data.json)，包含 255 个命名元精灵、共享指针事件及 HUD／雪粒表，不包含可执行 ROM。
- REA 完整记录：[presentation-sprites-rea.json.gz](evidence/presentation-sprites-rea.json.gz)。验证及 red／green／race 结果见 [交付记录](evidence/presentation-sprites-verification.json)。

```sh
. scripts/env.sh
go test -race ./games/contra/engine -run 'Test(OriginalFullSpriteComposition|SpriteComposition)' -count=1
```

重新生成原版证据使用 `presentation_sprites_prepare.py`、私有 `presentation_sprites_rea.mjs` 和 `presentation_sprites_capture.py --mode natural|controlled --label <新名称>`。导出脚本通过 `--first-label`、`--natural-label`、`--controlled-label` 选择重复与主录制；`--runtime-data` 单独生成运行时描述。录制器拒绝覆盖同名运行，用户 ROM 与其他任务产物始终只读。
