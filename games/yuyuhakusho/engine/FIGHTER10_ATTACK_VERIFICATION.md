# 来源 10 攻击选择：实现前验证范围

本阶段只恢复来源 10 的地面普通 `18164`、空中普通 `182a6`、地面特殊 `183b0` 和空中特殊 `18420` 四个选择器的完整返回，包含七个特殊构造 `19ac0/19aea/19b14/19b46/19b78/19ba0/19bd8`、普通构造、地面／空中投技提交及实际特效分配。`11/2d/47/48/49` 的后续持续阶段、动作尾段、世界／会话和完整对局不在本阶段。现有酎代码、世界适配器和既有语义包不修改。

## 实现前列出的失败场景

来源 ID／入口或模式绑定错误；只执行选择分支而未等待真正返回；请求 8 与旧姿态 11／请求 7 的优先级错误；把前向 `0f` 的 `0x38000` 或后退 `11` 的正 `0x40000` 套成酎速度；只更新父人物而漏掉 FX／池游标；池满或复用旧槽时丢失残留；方向表相对偏移或 nibble 分类错误；B／A 同时按下的优先级错误；按下沿、变化、原始输入与扩展 byte 混用；忽略空中投技先提交再继续普通攻击的顺序；地面／空中投技参数、目标动作、朝向、坐标 word 回绕或旧记录字段保留错误；把投技提交等同于命中／伤害执行；能量有符号门槛、成本或高 word 扣除错误，误清低 word；请求 1 额外添加按键变化门槛；请求 3 错误转向或混用 B／A 姿态；请求 6 生命 111 边界错误；空中特殊误用请求 3 或酎的方向门槛；仅应清 `+48` byte 时清掉 canonical `Aux48` 的其余 24 bits；共享重置误清原动画标志、请求、速度、命令或未知字节；改写只读子槽／其他人物／共享 RNG；非法模式、池游标或投技存储导致部分输出／调用者 slice 泄漏；逐调用回灌期望中间态冒充连续流；把私有 CPU 调用、现有来源 9 自然基线或全角色动画／AI 验证称作来源 10 自然招式／完整对局。

## 验证顺序与数据边界

先保存本清单，采集固定原 CPU 的完整返回／边界／单初态流；再写测试并保留实际缺类型／缺函数失败；最后新增独立来源 10 模块。可直接调用已验证的重置、普通小构造、能量扣除和效果分配 helper，不复制动作尾段，不建立 RAM 指令机，也不修改既有共享实现。

原版快照比较人物完整 256 bytes、四人物及三个相邻子槽家族共 4,096 bytes、FX 池 2,048 bytes、投技区 512 bytes、投技与池游标、能量低 word 和共享 RNG。私有调用检查完整 RAM 的允许写入范围和真实返回栈；自然观察保持固定 3,600 帧完整 RAM／视频基线。现有基线是来源 9；来源 10 的自然计数单独报告，私有上下文不作自然证据。

新模块以独立 byte 字段表达人物 `+48`，对应 canonical `Aux48` 的最高 byte；调用者回写时保留其余 24 bits。本阶段显式保留能量小数与 RNG，不推进子对象或碰撞。

## 已验证结果

| 范围 | 完整调用／步骤 | 实际覆盖 |
| --- | ---: | --- |
| 真实菜单输入的自然调用 | 2,005 | 地面普通 891、地面特殊 906、空中普通 104、空中特殊 104，均为来源 10、对象 `8400` |
| 明确私有输入的代表调用 | 21 | 四个完整入口、七个特殊构造的成功／能量不足，以及普通投技和冲刺 |
| 明确私有输入的边界调用 | 11,428 | 四人物地址各 2,857 条；全部普通构造、两处投技提交和七个特殊构造入口 |
| 单初态流 | 16 条／320 步 | 每种模式四条朝向流；每步只改变预定的基础 Held／Pressed／Changed |

自然输入为 [fighter10-inputs.json](testdata/fighter10-inputs.json)，SHA-256 为 `d7859f794df2a2a9cf356eae01d239bf658632f58e69cee0cc5abb979ad3776e`。未挂钩原内核从冷启动生成 [3,600 帧参考](testdata/fighter10-attacks-natural-reference-frames.json)，观察内核按相同输入运行后，全部 RAM／视频摘要逐帧一致。参考摘要与原内核身份见 [reference manifest](testdata/fighter10-attacks-natural-reference-manifest.json)。最初的来源 9 输入也保持原基线一致，没有来源 10 自然调用；该零计数证据单独保留为 `fighter10-attacks-source9-baseline`，不混入上述 2,005 条。

自然片段实际命中普通构造 `18828/18812` 各一次。其余普通构造、两投技提交和七个特殊构造由明确私有输入覆盖，不能称作全部特殊招式的自然证据。本片段是来源 10 真人对来源 9 电脑，没有来源 10 自然 AI 验证。

全部调用均已执行到原 RTS 后的返回位置，并检查 SP 为入口 SP 加 4；没有未完成调用、采样错误或私有范围外 RAM 改写。每次比较完整人物、三个子槽、四人物家族、FX、投技、计数及游标，所有字节均未屏蔽。能量小数与共享 RNG 全部保持原值，实际 RNG 调用数为零。压缩文件和解压后的完整 JSON 均按 SHA-256 绑定。

测试先于生产模块，保留了只缺 `Fighter10AttackState`／`SelectFighter10Attack` 的[实际编译失败](testdata/fighter10-attacks-red.jsonl)。实现一次通过代表／边界／连续流；追加真实自然记录后，原生竞态与 Node/Wasm 均重新运行六项顶层测试，包括原有三个酎攻击回归。最终分别用时 60.724 秒和 14.640 秒。[验证报告](testdata/fighter10-attacks-validation.json)绑定当前源码、夹具、日志、输入及静态证据；现有酎源码与既有报告未修改。

## 接口与原版细节

接口为 `SelectFighter10Attack(state Fighter10AttackState, mode ChuAttackMode)`，继续使用既有四个模式常量。状态嵌入 `ChuAttackState`，增加 `Aux48High uint8`、`EnergyFraction uint16` 和 `RandomSeed uint32`。后两者只穿透保留；没有新增 canonical 存储副本。调用者已有完整 `+48` long 时，以最高 byte 建立视图，回写采用 `(old & 0x00ffffff) | uint32(next.Aux48High)<<24`。

- 地面请求 8 优先于旧姿态判断，进入 `11`、姿态 `0c`、正横速 `0x40000`；请求 7 仅在旧姿态不是 `0b` 时进入 `0f`，横速为 `0x38000`。FX 的方向判断使用朝向 byte 是否非零；没有把后退持续阶段提前执行。
- 地面前向 A 的记录使用 X 偏置 ±8、Y 偏置 `-0x28`、宽 `0x18`／高 `0x28`、`SourcePose=0xb8`／`SourceAction=3`、`TargetPose=0x41`／`TargetAction=9`、`TargetFacing=1`、`OffsetX=0x20`／`OffsetY=-0x30`。记录先使用旧朝向计算，再进入普通朝向构造。
- 空中普通选择先在方向非 0／1 且 A pressed 时提交记录，再继续 B／A 普通选择；两键同时 pressed 时，记录仍提交，后续由 B 优先。空中记录为 X 偏置 ±`0x18`、Y 偏置 `-0x30`、宽 `0x18`／高 8、`SourcePose=0xb9`／`SourceAction=9`、`TargetPose=0x41`／`TargetAction=9`、`TargetFacing=1`、`OffsetX=0x18`／`OffsetY=0`。
- 地面请求 1 不检查方向或 Changed，直接尝试成本 24；请求 2 的任一基础攻击 Changed 均可触发成本 16；请求 3 的 B 优先于 A，两者成本均为 16，分别选姿态 `a2/a3`，不执行转向；请求 6 检查生命不大于 111、A Changed 和成本 200，只清 `+48` 的一个 byte。
- 空中特殊只接受请求 2，B 优先于 A，成本 16／20；方向不设额外门槛，成功后才按保留方向转向。所有重置与能量扣除继续调用已有小 helper。

投技存储按调用者提供的语义切片验证，不能访问的记录原子失败；原程序没有固定容量夹取。本阶段的错误验证覆盖未知模式、FX 游标失配，以及地面／空中投技地址未对齐或超出所提供记录。失败不会改写调用者 slice、人物或池。原程序的任意越界存储地址别名不在该语义接口范围。

静态来源复用已保存的[选择器清单](../../../docs/research/md-yuyuhakusho/static/fighter-action-selectors-focused.txt)与[构造清单](../../../docs/research/md-yuyuhakusho/static/fighter-action-constructors-focused.txt)，没有重新扩大 ROM 范围。`11/2d/47/48/49` 的持续阶段由独立模块处理；本函数没有动作尾段、指令 VM 或 ROM 运行时依赖。

## 复跑

从当前 worktree 根目录运行，固定原版工具按[研究工具说明](../../../scripts/yuyuhakusho/README.md)准备。私有矩阵和真实自然采集使用独立输出目录：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter10_attacks.py \
  --out .local/md-yuyuhakusho/fighter10-attacks-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter10_attacks.py \
  --out .local/md-yuyuhakusho/fighter10-attacks-natural-replay \
  --inputs games/yuyuhakusho/engine/testdata/fighter10-inputs.json \
  --baseline games/yuyuhakusho/engine/testdata/fighter10-attacks-natural-reference-frames.json \
  --natural-only
```

未观察基线可直接按同输入复核，不附加 CPU observer：

```sh
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH="$PWD/scripts/yuyuhakusho" python3 - <<'PY'
import json
from reference import ROOT, ReferenceSession, buttons_at
events = json.loads((ROOT/'games/yuyuhakusho/engine/testdata/fighter10-inputs.json').read_text())
expected = json.loads((ROOT/'games/yuyuhakusho/engine/testdata/fighter10-attacks-natural-reference-frames.json').read_text())
with ReferenceSession(output=ROOT/'.local/md-yuyuhakusho/fighter10-baseline-replay') as ref:
    actual = [ref.step(frame, buttons_at(events, frame))['reference'] for frame in range(1, len(expected)+1)]
assert actual == expected
print('unobserved original baseline matched', len(actual))
PY
```

本阶段聚焦测试显式使用 19 个现有／新增源码及测试文件，复用原有字节适配器，并重跑酎攻击三个测试。父阶段仍需运行完整包、持续动作和世界集成门禁。

```sh
task_root="$PWD"
mkdir -p "$task_root/.local/md-yuyuhakusho/tmp"
(
  cd games/yuyuhakusho/engine
  GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
  GOTMPDIR="$task_root/.local/md-yuyuhakusho/tmp" \
  go test -race -count=1 -run '^Test(Fighter10Attacks|ChuAttacks)' -json \
    input.go fighter_input.go animation_clock.go animation_records.go ground_actions.go \
    motion.go jump.go effects.go throw_contacts.go throw_records.go chu_attacks.go \
    fighter10_attacks.go attack_contacts.go effects_test.go jump_test.go \
    throw_contacts_test.go attack_contacts_test.go chu_attacks_test.go fighter10_attacks_test.go \
    > "$task_root/.local/md-yuyuhakusho/fighter10-attacks-native-replay.jsonl"
  GOOS=js GOARCH=wasm GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
  GOTMPDIR="$task_root/.local/md-yuyuhakusho/tmp" \
  go test -count=1 -run '^Test(Fighter10Attacks|ChuAttacks)' -json \
    -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
    input.go fighter_input.go animation_clock.go animation_records.go ground_actions.go \
    motion.go jump.go effects.go throw_contacts.go throw_records.go chu_attacks.go \
    fighter10_attacks.go attack_contacts.go effects_test.go jump_test.go \
    throw_contacts_test.go attack_contacts_test.go chu_attacks_test.go fighter10_attacks_test.go \
    > "$task_root/.local/md-yuyuhakusho/fighter10-attacks-wasm-replay.jsonl"
)
```

上述连续流仅证明选择器与共享输入状态的连续组合，没有推进动作时钟、人物运动、投技命中或世界调度，部分特殊流在首次成功消耗请求后保持无动作。本阶段不作为来源 10 完整招式或完整对局验收。
