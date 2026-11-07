# 基地 1 与基地 2 敌人规则

本轮恢复日版 FC 魂斗罗第 2、4 关的 17 类专属敌人，以及 Boss 房间共用的炮台和护板。实现使用独立的实体、计数器和绘图请求，不执行 ROM，也不保留 CPU 地址空间。原版录制用于开发对照；完整世界、联机、电脑玩家和浏览器验收仍由集成阶段完成。

原始 ROM 为 `Contra (J).nes`，来自用户目录中的 `03/魂斗罗日版(完全版).zip`，SHA-256 为 `6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe`。REA 使用私有 stdio 会话和 Ghidra 12.1.4。美版反汇编仅帮助定位；下表的地址、表项和运行结果均以该日版 ROM 为准。

| 类型 | 规则入口 | 恢复范围 |
| --- | --- | --- |
| `0x10`、`0x1B` | `StepBaseBoss` | Boss Eye 与追踪弹 |
| `0x11`、`0x12` | `StepBaseThrownWeapon` | 滚轮、手雷、弹道和爆炸 |
| `0x13`、`0x14` | `StepBaseWallTurret`、`StepBaseWallCore` | 墙炮、核心、破坏层与门洞 |
| `0x15`—`0x18` | `StepBaseIndoorSoldier` | 普通兵、跳跃兵、投雷兵和四人编组 |
| `0x19`、`0x1A`、`0x20` | `StepBaseGenerator` | 士兵、滚轮及红蓝兵生成程序 |
| `0x1C`、`0x1D` | `StepBaseBoss` | Gemini 双 Boss 与旋转泡泡 |
| `0x1E`、`0x1F` | `StepBaseRedBlueSoldier` | Boss 房间蓝兵与红兵 |
| 共用 `0x08`、`0x0A` | `StepBaseCommonWall` | Boss 房间炮台和护板 |
| 共用 `0x00` | `StepPickup` | 室内投掷、落地、透视推进移除 |

房间脚本、属性和状态表见 [enemies-base-inventory.json](evidence/enemies-base-inventory.json)。Boss 的受控与连续录制另见 [enemies-base-bosses.md](enemies-base-bosses.md)。

## 关键顺序

`EnemySystem.Base` 保存核心数、已破坏护板数、攻击轮次、红兵是否已生成及投雷兵状态。房间清除状态复用 `CombatFrame.ScreenCleared`，通过指针传入，避免出现第二份房间推进状态。室内房间加载器由公共敌人调度维护。

最后一个核心在状态 8 扣减核心数后，先隐藏自身并销毁符合原版条件的敌人。状态 9 分四次绘制门洞并生成爆炸；只有状态 10 的延迟结束，才把原版 `$6A` 设为 1。不能在核心 HP 归零或核心计数归零时立即放行下一房间。日版此字段为 `$6A`，不能照搬美版 RAM 名称。

墙炮和核心的 2 列图块动画经 `$EB70 → $F9F7 → $C593`，坐标各减 4；门洞、炮台和护板的超级图块经 `$EB1B`，坐标各减 12。两条路径都使用真正的 `GraphicsQueue`。拒绝绘制时保留原版重试计时和状态，不能用恒定成功的回调代替验收。

士兵生成程序按奇偶帧推进。满池时，已消费的生成程序和计数器仍按原版推进；四人编组按空槽降序分配，部分分配成功后不回滚。室内生成程序最多完成 7 轮，投雷兵存在期间会阻止相应士兵生成器继续倒数。

Boss Eye 需要 4 块护板，Gemini 需要 3 块护板。Gemini 会改写共享随机数，须使用同一个 `SharedRandom` 引用。最终 Boss 完成信号为 `BossBaseFinished`，关卡延迟为低字节 `0x60`、高字节 0；这不是平台胜利或积分结算信号。

室内武器拾取物使用独立的地面投影和抛物偏移。重力为每帧 `0x12`，落地后固定在 `Y=0xA4`，保留速度字段；透视推进 `$65` 非零时移除。`EnemyStepContext.ScreenScroll` 必须来自现有 `World.ScreenScroll`。该分支不能复用室外落地检测，也不能仅解除原有的室外限定检查。

## 对照与边界

先写失败场景和原版 oracle 测试，再实现规则。六项非 Boss 测试覆盖 1,850 次受控调用与 31,246 次连续原版敌人调用，比较整个敌人池、共享状态、音效请求、图形请求、队列字节、地图和碰撞窗口。受控调用覆盖所有状态及绘图拒绝、计时器为零、满池、生成开关、生成轮次等边界；其中 448 次额外遍历全部 13 个普通房间的士兵程序、两套滚轮程序和红蓝兵程序游标。

| 测试 | 调用数 |
| --- | ---: |
| 墙炮与核心 | 9,212 |
| 三类生成器 | 4,775 |
| 四类室内士兵 | 4,870 |
| 滚轮与手雷 | 2,169 |
| 红蓝士兵 | 1,794 |
| 共用炮台与护板 | 10,276 |
| 室内拾取物（追加） | 928 |

拾取物追加 576 次受控调用和 352 次连续原版调用，单独重新录制 `$65`，没有从帧末 RAM 猜测调用时的透视状态。公共拾取入口的修改另回归既有 121 次室外原版调用。

连续房间录制从声明过的原版关卡入口快照继续，仅输入按键。Boss 房间录制额外声明一次房间号、清除标志和屏障设置，再由原版过渡进入 Boss 房间。这些记录不能称为从第一关自然通关到基地 Boss。

受控调用切换敌人的场景类型，但不运行完整房间加载器。因此，共用墙体测试中，更新图块使用 Boss 表，背景属性页仍是入口房间的原始页。测试显式保留这个混合上下文；正常 Boss 上下文由连续录制另行验证。

普通室内兵经 `$92E4 → $971B → $971E` 生成滚轮时，会把此路径之前留下的元数据写入新滚轮的 `Attributes`。四条自然记录为：`id140/slot4=19`、`id8586/slot8=12`、`id8715/slot9=12`、`id9174/slot6=19`。当前实现已经恢复前一实体实际碰撞框高度的传递，复制真实入值，不再归一化为 0；详见 [碰撞高度恢复](enemies-base-throw-height.md)。此前 12,800 帧成对原版运行的无玩法影响证明仍保留为历史证据。

[旧兼容清单](evidence/enemies-base-compatibility.json) 绑定的 17 条豁免已从当前测试移除。新的只读输入侧录补齐历史场景的调用前数据，并保持原夹具及其期望值不变。旧完整成对字节证据 `enemies-base-roller-paired.json.gz` 只用于研究回溯。

## 复验

```sh
. scripts/env.sh
python3 -B scripts/contra/enemies_base_verify.py
```

该入口编译全部引擎生产文件，加载基地测试、室外拾取物回归及共享 oracle 解码辅助，运行 `-race` 并保存 JSONL 结果。这样可以在其他敌人组仍处于先写测试的阶段复验本组；它不替代父级集成验收。当前结果、命令、源码摘要和夹具摘要见 [enemies-base-verification.json](evidence/enemies-base-verification.json)。

原版受控录制可以重复执行：

```sh
.local/board-art-env/bin/python -B scripts/contra/enemies_base_cases_capture.py repeat-unique-label
.local/board-art-env/bin/python -B scripts/contra/enemies_base_cases_capture.py repeat-unique-label waves
.local/board-art-env/bin/python -B scripts/contra/enemies_base_capture.py --stage 2 --mode room --frames 1600 --label repeat-unique-label
.local/board-art-env/bin/python -B scripts/contra/enemies_base_capture.py --stage 4 --mode room --frames 1600 --label repeat-unique-label
.local/board-art-env/bin/python -B scripts/contra/enemies_base_pickup_capture.py repeat-unique-label controlled
.local/board-art-env/bin/python -B scripts/contra/enemies_base_pickup_capture.py repeat-unique-label natural
```

录制脚本拒绝覆盖原记录，并逐条比较已有夹具。`enemies_base_prepare.py` 生成源码绑定的私有 REA 窗口、种子和各分析请求；`enemies_base_tables_export.py` 从固定 ROM 重新导出室内生成表。完整 REA 记录和重复录制比较另存于证据目录。

本组清理了自己的失败探针、FCEUX 临时配置、REA 缓存和 PNG 对应的中间 GD 图片；完整原版调用改为无损 gzip 保存。保留原始 RAM、实际输入、PNG、源快照和验证结果，没有清理其他组目录。清单见 [enemies-base-cleanup.json](evidence/enemies-base-cleanup.json)。
