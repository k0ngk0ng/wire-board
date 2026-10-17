# 高台边缘静止球：原程序缺陷调查

状态：已验证原程序也能从平台的合法冷开场状态进入同一卡局；尚未修正规则，等待确认处理策略。球队 5 的原失败验收继续保留，不标记通过。

## 触发链

固定 Court1、球队 0／5、球员 0／1、四名原版电脑、seed `0x12345678`、正常两节时钟。

1. Go video 6297：球在 64 像素高台落稳，最后一次弹跳剩余计数为 1。
2. video 6300：原球动作 6 消耗最后一个弹跳次数，清空竖直速度与重力，转为动作 5；同一次横向移动从 X=392.28125 到 391.28125，离开高台进入地面区域。原地形判定设置 airborne，但未补回重力。
3. video 6303～6312：球以原摩擦逐渐停止，保持高度 64、重力 0。
4. video 6315：球转为动作 0，停在 X=388.78125、depth=72.1875。原动作 0 不补重力，球持续悬空。
5. 原 AI 的静止球入口明确要求球的整数高度为 0；原自动接球阶段也排除动作 0。原节末检查在球无主且 airborne 时等待，时钟为 0 后仍无法结束。

原球函数 `$B6CA`、`$B6A1`、`$B52E` 与地形函数 `$8249` 的这一组合缺少支撑面消失后的重力恢复。不能通过清除 airborne、直接判胜、缩短时钟或更换 seed 解决验收。

## 分层证据

### Go 失败与原球组件

原失败：`.local/basketball/team5-after-handler20/native/`。48,000 视频帧、160 次保存恢复仍未结束，源码摘要稳定。

`TestBallRestStallDiagnostic` 从相同冷配置重新开始，不改变任何游戏状态，保留第一段持续悬空状态及前 50 个逻辑更新。`probe_rest_stall.py` 将这 50 个真实状态逐个送入未修改的原 `$E149..E14C` 完整球分派，50／50 相同；只初始化第一次后连续执行原球分派，也 50／50 相同。这一层单独不能证明完整原程序也会卡住。

### 原菜单残留不同的对照

先运行仅设置初始规则选项的原程序，完整 15,000 视频帧内本次没有出现比赛中的悬空卡局，第一节结束。无观察的 reference 与只读观察版本的全帧 RAM 和 5 张截图一致。

进一步比较发现，其首次完整世界状态与平台冷开场有差异：原菜单残留人物 0 的深度小数 128、人物 1 的 X 小数 128；平台冷初始化为 0。原 RNG 与平台也相差一次演进，逻辑帧及 HUD 时钟来自不同的开场时序。没有为追求轨迹相等而修改平台冷初始化或 seed。

### 一次受控起点后的原整机

在原程序首次完整世界结束处 `$D355`，只进行一次明确记录的对齐：上述两项位置小数、RNG、逻辑帧及原时钟／HUD 缓存与平台首次 playing 世界一致。完整写入清单、写入前后 RAM 和发生帧保存在 `aligned-patch.json` 与 `alignment-*.ram`。

此后原 CPU、AI、输入、接触、球物理和节末检查正常执行，不再写入任何游戏状态，控制器保持空输入。该路径**不是原菜单自然选择覆盖**，也不是逐帧状态回放。

15,000 个视频帧中记录了 4,531 个完整世界更新：

- 第 2,067 个后续世界更新开始，原程序的球也停在 X=388.78125、depth=72.1875、height=64，动作 0、无主、airborne、零速度及零重力。
- 连续 2,464 个完整世界更新保持同一悬空状态。原时钟最终为 0、比分 3∶20，仍处于比赛阶段。
- 去掉持续观察代码、只保留一次起点对齐的 reference 运行，全帧 RAM 和 5 张截图逐字节相同。
- 独立推进 Go 的 4,531 个世界，球状态、TeamAI 状态及完整输入状态全部与原程序一致。
- 全语义比较不是全部字段相等：第 1,013 个世界起，人物 1 的 `SpecialShot` 在原程序为 0、Go 为 128，共 3,382 处差异，须另外恢复其清除来源。所有其他比较的规则字段一致。比较明确排除了动作阶段复用的零页 contact scratch 和渲染投影字段，不能把这些差异隐藏后宣称全 RAM 相等。

这足以证明卡局由完整原程序在同一受控语义起点上复现，不是仅由 Go 的球尾函数、AI 或节末实现造成。

## 可复核产物

目录 `.local/basketball/rea/ball-rest-stall/`：

- `first-entry/`、`original-approach.json`：最早触发状态、完整球源调用及连续链。
- `original-configured-result.json`：只配置初始选项的参考运行；其统计中的 91 帧静止悬空球属于开场阶段，不是比赛卡局。
- `aligned-patch.json`、`aligned-spec.json`：受控起点字段和完整控制器时间线。
- `aligned-result.json`：原整机复现及 reference RAM／截图一致结果。
- `aligned-world-comparison.json`：独立 Go 与原完整世界的逐字段比较，保留 SpecialShot 差异。
- 原始整机 RAM、每个完整世界 RAM、截图在 `.local/nes-basket/runs/ball-rest-team5-aligned/`，reference 在同名 `-reference/`。

原完整世界轨迹 SHA-256：`93fd388aab383aa7a82c7bff46981266027a080cc26b8cf2146297765d9dc8a5`。整机全视频 RAM SHA-256：`6d074a5040abe19f5d83c252d2c933875f6d2475ccea67e329af55bb01cfc4a2`。ROM SHA-256：`0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`。

```sh
source scripts/env.sh
BASKETBALL_REST_STALL_OUTPUT="$PWD/.local/basketball/rea/ball-rest-stall/first-entry-repeat" go test ./games/basketball/engine -run '^TestBallRestStallDiagnostic$' -count=1 -v
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/probe_rest_stall.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_rest_stall_aligned.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/compare_rest_stall_worlds.py
```

## 最小修复建议，尚未实施

若选择修正原版缺陷，应在游戏规则层恢复「无支撑的松散球正常下落」，保留原 AI 策略：

- 限定无主球、动作 0／5、球高于当前位置实际支撑面、竖直速度和重力同时为 0；在原动作 6 转 5 的同次物理后也检查该条件。
- 使用游戏已有普通自由落体重力 `Pixel`，保留当前位置、小数、水平运动、球权和 airborne；随后由正常物理落地、接触和节末检查完成，不直接修改得分或比赛状态。
- 不能作用于持球、挂筐、特殊球暂停动画或仍有高台／设备支撑的球。
- 明确把这项作为「修复原版缺陷」的规则行为，与原指令严格对照模式区分。源函数 oracle 仍应保留原结果，平台运行启用修正的选择必须保存到对局配置，恢复存档不能切换策略。不通过悄悄改变 source oracle 期望值伪装成原行为。

实施前回归计划：先保留原缺陷复现，再写未通过的修正模式验收，覆盖台面边缘的最后一次弹跳、动作 5 滚停、动作 0 悬空存档恢复、真正受支撑的静止球、持球／挂筐／特殊球排除；随后同 seed 正常时钟的球队 5 两次完整比赛及每帧／周期恢复、Native／race／Wasm 一致性、浏览器可见下落。原 48,000 帧失败证据保留，不延长上限替代修复。

## 后续：落地特殊投篮标记已恢复

之后通过只读 `$0554..$0557` 写入追踪，找到了上述 `SpecialShot` 差异：原共享落地函数 `$8BBD` 在首入时通过 `$8BD7` 清零当前人物的标记。现已按原时序恢复，详见 `LANDING_SPECIAL_CLEAR_VERIFICATION.md`。

修复后再次从相同冷配置独立推进 Go，4,531 个完整世界的比较规则字段全部与受控起点原整机一致；仍明确排除已被后续阶段复用的零页 contact scratch 和渲染投影，未宣称全 RAM 一致。新结果保存在 `aligned-world-comparison-after-clear.json`，旧差异结果和旧 Go 轨迹全部保留。

悬空球的位置、时机、持续状态和原 AI 行为完全保留；这次只恢复原有清零，没有修正卡局缺陷，也没有让球队 5 验收转绿。
