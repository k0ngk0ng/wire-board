# Base 滚轮跨阶段创建输入

第 26905 帧的合法 B 输入曾失败：slot 15 生成器没有产生碰撞框，slot 14 士兵成功分配滚轮时，World 没有提供继承属性。现已恢复该创建输入的完整传递，精确下一帧与从首帧开始的重放均通过。

## 实现前失败场景

失败场景在修复前记录，保留 red：遗漏玩家弹分派的真实生产者；把值固定为 0 或 0xFC；忽略弹种高位、槽位顺序、延迟／命中阶段和火焰独立写入；遗漏同一敌人阶段内的生成器、动画和射击覆盖；使用创建者自己稍后才计算的碰撞高度；无新生产者时丢失前值；失败提交半帧；修改旧原版期望或恢复归一化豁免。

## 实际来源与传递

`BaseThrowWork` 保存一个明确的创建输入及 `Known` 标记，跟随 World 存档。原版地址只用于定位来源，运行时通过已有规则操作更新这个命名值。

| 原版操作 | 保存的语义 |
| --- | --- |
| `$8043/$8053` 玩家开火准备 | 实际进入准备路径时的跳跃状态 |
| `$DA5E/$DA62` 玩家侧探测 | 本次探测的横坐标 |
| `$859B/$85AE` 玩家弹分派 | 实际弹种所选状态表的身份字节，含延迟与命中阶段 |
| `$886A/$88A7` 室内火焰 | 当前形态使用的纵向偏移表身份 |
| Base 生成器与投掷程序 | 实际读取的生成属性或车道值 |
| 手雷／爆炸呈现 | 本次真正使用的图形计数或图形表身份 |
| `$F225/$F29A` 敌方射击 | 申请时的弹类型，成功后为带符号 X 速度的小数字节 |
| `$E342` 碰撞框 | 按实体与接触顺序实际计算的最终高度 |
| `$A97B/$A991` 场景出生阶段 | 当前出生流身份，即使该室内房间已被消费也更新 |

玩家和玩家弹阶段将工作值传给敌人阶段；敌人按原版下降槽序继续覆盖；滚轮只在实际成功分配时复制当时的值。下一帧没有新生产者时保留前值。值为 0 与来源未知分别表示，未知仍报错。

失败帧有四颗室内普通玩家弹，原版分派留下 `0xFC`。这由完整 REA 代码和真实原版 `8584 → E54F → 972E` 运行共同确认。其他合法场景分别得到 `0x02`、`0x08`、`0x60`、`0x0E`、`0x14`、保留入值 `77`，或被后续碰撞改为 `20`。同槽阶段的生成器、爆炸、射击与门洞也产生不同的实际覆盖值，因此没有使用固定填充。

物理、敌人选择、HP、操作和胜负规则保持原实现。本次增加创建输入的来源传递，没有修改电脑策略或视觉分类器。

## 原版与重放验证

保留 27 个原版受控边界场景，覆盖玩家弹族、命中／延迟状态、火焰、无弹保留、碰撞覆盖，以及较高槽位的生成、爆炸、射击和门洞操作。原版从真实武器入口执行到完整敌人阶段返回；期望值在真实滚轮读取点记录。19 场景初组和 27 场景扩展组均独立重录，完整记录逐字节一致。它们是声明过输入的原版边界对照，不是自然通关。

Base 1、Base 2 各 800 个连续原版 World 调用，从首个输入值开始推进，中途只提供既有按键与声明的原版帧前随机输入；每帧的创建工作值和全部敌人输出一致。另回归八关各 800 个原版完整窗口、既有基地严格字段、玩家弹、CombatFrame、Session 与 Runtime 契约。

精确诊断 before 为第 26904 帧，SHA-256：`78c171aa5912b86e06e5776ff6e83520b88b602f3616af4bf92f6c579662434c`。直接载入后执行 `[B,0]` 即可推进，无需策略、补命或状态修改。

完整前缀 E2E 从标准 `NewSession(seed=42, players=1)` 开始，重放全部已录按键和 24 笔明确声明的备用生命补充：首帧前 `2 → 10`，随后 23 次 `9 → 10`。没有跳关、移动、改敌人或注入结果。第 26904 帧既有 combat、campaign、random、enemy、terrain、score、controls、presentation、audio 均与录制一致；新增工作值另由原版对照验证。第 26905 帧成功提交，真实滚轮保留 `0xFC`。

正常引擎／运行时包的限定 race 共 **95 项通过（35 个顶层、60 个子场景）**，本组源码与夹具在运行前后 SHA 不变。完整记录见 [验证结果](evidence/base-throw-work-verification.json)，来源表与 14 份完整 REA 记录见 [来源清单](evidence/base-throw-work-sources.json)。

## 复验

```sh
source scripts/env.sh
mkdir -p .local/contra-base-throw-work/tmp
export TMPDIR="$PWD/.local/contra-base-throw-work/tmp"
export GOTMPDIR="$TMPDIR"
go test -race ./games/contra/engine \
  -run '^(TestSessionBase2Throw.*|TestOriginalWeaponToBaseThrowWork|TestOriginalBaseThrowEarlierSlotProducers|TestOriginalContinuousBaseThrowWork|TestOriginalBaseRollerInheritsCollisionHeight)$' \
  -count=1 -json
python3 -B scripts/contra/base_throw_work_sources.py
.local/board-art-env/bin/python -B scripts/contra/base_throw_work_capture.py recheck
.local/board-art-env/bin/python -B scripts/contra/base_throw_work_capture.py recheck-producers producers
```

完整 race 选择器保存在验证 JSON 中。原版重录需要既有日版 ROM、FCEUX 和 `stage-4-v2` 原始入口；脚本拒绝覆盖同名运行目录，并核对输出与保留夹具一致。

首次受控采集曾把敌人定点速度的高低字节接反，导致投掷者离开投掷范围。该采集未成为 oracle；修正记录器后完成上述有效采集与独立重录。失效原因保留在报告，不用错误输出调整规则。

已清理本任务临时模拟器副本、缓存、重复重录输出及中间日志；保留不可变夹具、完整 REA、red／green、输入／补命和重录摘要。该修复不代表无补命八关通关或最终浏览器验收完成。
