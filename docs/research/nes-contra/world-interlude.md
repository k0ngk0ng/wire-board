# 首关击杀至第二关开战的连续 World 验收

`world_interlude_test.go` 已通过 7,997 次连续原版调用对照。整个测试只从第一条 before 初始化一次；后续仅输入控制器采样和明确列为外部输入的原版空闲熵字节。规则、过场、地图、生命、分数和图形状态均由 World 连续推进，没有回填 after。

唯一 ROM 干预沿用既有 Boss 路线：第一次 `$C001` 将两个角色的 reserve lives 设为 30。后续击杀、过关、关间演出、关卡清理和第二关入口均由原版执行。它证明该路线的执行一致性，不是普通生命规则下的自然八关通关证明。

## 连续覆盖

| 关卡与阶段 | 调用数 |
| --- | ---: |
| 首关普通战斗 | 5,987 |
| 首关过关等待 | 162 |
| 首关出口 | 323 |
| 首关关间演出 | 701 |
| 跨关清理 | 1 |
| 第二关 Initialize | 1 |
| 第二关 Intro | 669 |
| 第二关 Prepare | 1 |
| 第二关 Entrance | 32 |
| 第二关战斗 | 120 |

前 6,472 条记录与既有真实 Boss 路线一致。两次独立录制的完整 10,000 仿真帧 RAM、被观测的 World RAM／PPU 和输入流均逐字节相同。新增观测没有改变原版执行。

录制以 `$C001` 为入口，以 `$F8DB` 为出口，并要求出口栈深度等于入口 `SP+2`。背景直接写入会跨过 NMI；不检查栈深度会把嵌套 `$F8DB` 误判成整帧完成。

## 状态投影

关间演出复用退休的关卡头部和弹丸计时地址。测试按实际语义归属比较，保留到该演出返回阶段 5 的当帧；下一帧阶段 5 执行清理后，才恢复普通玩法投影。

| 原地址 | 关间演出语义 |
| --- | --- |
| `$40` | 演出组；本路线为首关组 0 |
| `$41..$47` | `Scene.Interlude` 的渐隐、说话者、动画与文字计数 |
| `$48..$49` | `Scene.Interlude.TextOrigin` |
| `$4A..$4B` | `Scene.Interlude.TextAddress` |
| `$0418` | `Scene.Interlude.MouthIndex` |

这些字节不能同时与普通 `ScrollType`、地图切换位置、碰撞阈值或弹丸 Timer 比较。测试只从比较副本中排除退休字段，随后逐字节检查真正的演出 owner；运行中的 World 不被修改。

每帧同时验证玩家／弹丸、地图和碰撞、敌人及生成器、分数、动画、OAM、调色板、CHR、图形队列、实际 PPU、显示控制和声音顺序。每帧另从 JSON 恢复一份 World 并推进，要求结果一致；同 tick 重试必须完全不变。

## 发现并修复的接线差异

最初失败发生在 tick 6,472、原版 frame 7,872、Interlude 阶段 0：实现错误清除了 HUD，原版 `$37` 仍为 `$FF`。原版改变的是队列模式 `$23` 和图形模式 `$07EB`。HUD 必须保持到下一帧过关清理，不能随进入关间演出清零。

失败记录保存在 [HUD red](evidence/world-interlude-hud-red.jsonl)。生产代码由主任务修复；本项只增加录制、测试和投影，没有修改 Scene／World 生产代码。

## 证据与复查

- [原版完整记录](evidence/world-interlude-original.json.gz)：7,997 条 RAM／PPU／声音前后记录、场景、输入摘要和干预声明。
- [最终验证](evidence/world-interlude-verification.json)：`-race` 结果、源码与夹具 SHA-256、两次录制摘要；检查期间源码未变化。
- [测试输出](evidence/world-interlude-final-race.jsonl)：连续 World 以及同游戏场景回归。
- 原始录制、输入状态和关键截图保存在 `.local/contra-world-interlude/oracle/runs/world-interlude-v1` 与 `world-interlude-v2`。

复查命令：

```sh
.local/board-art-env/bin/python -B scripts/contra/world_interlude_capture.py --label review
python3 -B scripts/contra/world_interlude_verify.py
```

录制标签必须唯一。`--export` 可以重新导出已经完成的同名录制，但会校验场景内容和完成标记。后台只保留当前任务的原始验收资料；清理范围见 [清理记录](evidence/world-interlude-cleanup.json)。

本项没有替代普通生命通关、第三至第八关连续推进、浏览器显示、网络协作或积分验收。
