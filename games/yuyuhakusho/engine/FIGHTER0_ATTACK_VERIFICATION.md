# 来源 0「幻海」攻击选择：实现前验证范围

本阶段只恢复 `15fdc/1611e/16228/16318` 四个选择器的真实完整返回，包括可达特殊构造、普通构造、地面／空中投技和冲刺 FX。名称绑定原版 HUD 的[菜单发现记录](../../../docs/research/md-yuyuhakusho/source0-next-selection.json)，不按名单顺序猜测来源 ID。后续 `0e/11/16/17/18/19` 持续动作、子对象更新、完整动作尾段、世界和对局均不在本阶段。

## 实现前列出的失败场景

来源编号、入口或模式错误；把空中投技内层 `16226 RTS` 当作整个选择器返回；忽略真实返回位置及 SP+4；请求 8 与请求 7／旧姿态 `0b` 的优先级错误；前后冲的正 `0x34000`、动作 `0e/11` 或方向不同于酎与来源 10 却被混用；FX 分配失败、池满、游标回绕或旧槽复用丢失副作用；方向表基准／nibble 分类错误；B／A 优先级错误；把 Pressed、Changed、Held、RawHeld 或扩展 byte 混用；地空投技坐标 word 回绕、宽高、源／目标姿态动作和偏移参数套用其他角色；空中投技提交后漏掉继续普通 B／A 选择；能量有符号高 word 门槛、成本、扣除及低 word 保留错误；请求 1／2 的中立方向要求遗漏；地面请求 3 与空中请求 3 的成本／负纵速／姿态混淆；请求 4／5 的主子槽占用门槛、C／B／A 优先级或保留请求错误；把 `Gravity` 最高 byte 蓄力模式另存一份或覆盖低 24 bits；速度高 word 的偏移／游标写入误清低 word；请求 6 的生命 111 边界或不应写的重力、纵速、Aux48 被改写；共享重置误清命令、原动画标志或未知字节；改写只读人物、三个子槽、能量小数或共享 RNG；非法模式、池游标、投技切片导致部分更新或调用者 slice 泄漏；逐调用回灌期望中间态伪装连续流；将私有 CPU 证据、菜单截图或自然无动作 selector 调用称作自然完整招式／完整对局。

## 顺序与数据边界

先保存本清单，按真实菜单输入从冷启动取得未观察／观察原内核的完整 RAM／视频基线；再取得完整自然、代表、边界和单初态流夹具；随后写测试并保留只缺新符号的真实编译失败，最后新增独立 Go 模块。既有 Go、语义包、报告、PROGRESS 和正在服务的页面资产保持不动。

快照比较人物 256 bytes、三个相邻子槽 768 bytes、四人物家族 4,096 bytes、FX 池 2,048 bytes、投技区 512 bytes、所有相关游标、能量低 word、Aux48 和共享 RNG。私有调用核对完整 RAM 的允许写入范围。接口复用 `ChuAttackState`／`ChuAttackMode`；`Body.Gravity` 和速度字段只建立原有存储的视图，不增加重复 canonical 字段。

## 已验证结果

| 范围 | 完整调用／步骤 | 实际覆盖 |
| --- | ---: | --- |
| 原菜单输入自然调用 | 1,644 | 地面普通 772、地面特殊 786、空中普通／特殊各 43；均为来源 0、对象 `8400` |
| 特殊输入自然交叉 | 1,700 | 地面普通 849、地面特殊 851；连射构造 `18a4c` 一次、蓄力构造 `189e8` 两次成功 |
| 明确私有代表调用 | 35 | 四个完整入口与全部特殊构造成功／能量不足、普通投技和冲刺 |
| 明确私有边界调用 | 13,276 | 四人物地址；12 个特殊构造根、9 个普通共用构造及两处投技提交 |
| 单初态流 | 16 条／320 步 | 四种模式各四条朝向流；只改变预定的基础 Held／Pressed／Changed |

原菜单输入为 [source0-next-inputs.json](../../../docs/research/md-yuyuhakusho/source0-next-inputs.json)，SHA-256 为 `ab1089aa8453df2997800c3e8477bf64161734d31521aec7c2f48827409a406a`。未观察原内核从冷启动生成[完整 3,600 帧参考](testdata/fighter0-attacks-reference-frames.json)；观察内核按同输入运行后，全部 RAM／视频摘要逐帧一致。自然实际命中普通构造 `18828` 两次、`18812` 一次。

特殊交叉使用独立的 [fighter0-projectile-inputs.json](testdata/fighter0-projectile-inputs.json)，SHA-256 为 `b7976367c37584f8f154fed981be9529b0c5eb78cc521166a306b68dc77464b7`。复用子对象阶段提供的 3,602 帧双内核参考，精确解压为[本阶段参考](testdata/fighter0-attacks-special-reference-frames.json)，没有重复生成未观察基线；本阶段观察器的全部 RAM／视频帧同样一致。第 1,823 帧进入 `18a4c`，能量 223→207；第 1,963、2,413 帧进入 `189e8`，能量分别 207→203、163→159。两份自然记录保持独立，不将这些成功外推为全部特殊构造或完整招式的自然覆盖。

所有采样执行到真正 RTS 之后的返回位置，并检查 SP 为入口 SP 加 4；没有在空中投技内层 `16226 RTS` 提前结束。没有未完成调用、采样错误或私有允许范围外 RAM 改写。完整输出不屏蔽字节；子槽、能量小数、Aux48 和共享 RNG 均保持原值，RNG 实际调用次数为零。

测试和原版夹具先于生产模块，保留只缺 `SelectFighter0Attack` 的[实际编译失败](testdata/fighter0-attacks-red.jsonl)。实现一次通过主验证：六项顶层测试含旧酎攻击三项回归，原生竞态 66.261 秒、Node/Wasm 15.884 秒。追加特殊输入时，使用同一份未修改断言编译测试二进制，在独立临时工作目录加载该自然夹具的文件别名，分别通过原生竞态 7.271 秒和 Node/Wasm 1.431 秒；没有替换主测试目录的原菜单自然记录，没有在实现后新增测试断言。

[验证报告](testdata/fighter0-attacks-validation.json)绑定当前源码、五组夹具、输入、参考、静态资料及所有日志，明确记录特殊交叉的文件别名。既有 Go、语义包、报告和页面资产未修改；完整动作／子对象／世界组合由父阶段另行验收。

## 接口与构造边界

接口为 `SelectFighter0Attack(state ChuAttackState, mode ChuAttackMode) (ChuAttackState, error)`，不增加状态类型或 canonical 字段。能量小数、Aux48 和 RNG 属于调用者已有状态；选择器不拥有或修改它们。

以下成本使用十进制，其余动作／姿态／地址使用十六进制。12 个特殊根与普通共用构造、两投技提交及冲刺 FX 分开计数。

| 模式／请求 | 特殊根 | 成本 | 结果与门槛 |
| --- | --- | ---: | --- |
| 地面 1／2 | `18946/1895e` | 8／12 | 中立方向、对应 B／A Changed；`15/a0` 或 `15/a1`，只清横速最高 byte |
| 地面 3 | `1898e/189ae` | 12／16 | B 优先 A；`16/a2` 或 `16/a3`，横速 `0x40000`、纵速 `-0x40000/-0x60000` |
| 地面 4 | `189e8` | 4 | 主子槽空闲，C／B／A Changed 优先级决定模式；`17/a4` |
| 地面 5 | `18a4c/18a64` | 16／32 | 主子槽空闲，B 优先 A；`18/a6` 或 `18/a7`，只清横速高 word |
| 地面 6 | `18a92` | 200 | 生命不大于 111、A Changed；`19/aa`，仅把横速 long 设为 `0x40000` |
| 空中 1／2 | `18ac4/18adc` | 8／12 | 中立方向、对应 B／A Changed；`02/ac` 或 `02/ad` |
| 空中 3 | `18b06/18b28` | 16／16 | B 优先 A；`16/ae` 或 `16/af`，横速 `0x40000`、负纵速分别 `-0x40000/-0x60000` |

全部特殊构造都不执行普通方向转向。地面请求 4 只替换 `Body.Gravity` 最高 byte 为 0／1／2，保留低 24 bits；横／纵速度高 word 分别写 `0x1c/-0x28`，保留低 word。地面请求 5 保留横速低 word；地面请求 6 不清重力、纵速、Aux48 或其他未写字段。

普通前／后冲分别进入 `0e/11`，两者使用正横速 `0x34000`；FX 继续使用已恢复的分配规则，保留池满、游标推进与旧槽残留。地面投技使用 X 不偏移、Y−`0x1c`、宽`0x14`／高`0x1c`、源／目标动作 3、目标姿态 `40`。空中投技先使用旧朝向提交 X±`0x10`、Y−`0x30`、宽高`0x10`、源／目标动作 9、目标姿态 `41`，再继续普通 B／A 选择；两键同时按下时仍先提交记录，随后 B 优先。两种投技均为 `OffsetX=0x18/OffsetY=0/TargetFacing=1`。

普通构造共用 `enterChuAttack`、`ResetActionState`，成本共用 `spendChuEnergy`，FX 分配共用 `AllocateEffect`。没有复制动作前置或尾段，没有 ROM／模拟器运行时依赖或指令 VM。非法模式、未对齐 FX 游标、地空投技未对齐／超出调用者记录切片时整体返回初态；调用者 slice 不变。原程序任意越界存储的总线别名不属于本语义接口范围。

## 复跑

从 worktree 根目录运行，原版工具按[研究工具说明](../../../scripts/yuyuhakusho/README.md)准备。主采集的默认参数即为幻海原菜单输入与 3,600 帧参考；特殊交叉另用独立输出目录：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter0_attacks.py \
  --out .local/md-yuyuhakusho/fighter0-attacks-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter0_attacks.py \
  --out .local/md-yuyuhakusho/fighter0-attacks-special-replay \
  --inputs games/yuyuhakusho/engine/testdata/fighter0-projectile-inputs.json \
  --baseline games/yuyuhakusho/engine/testdata/fighter0-attacks-special-reference-frames.json \
  --natural-only
```

主聚焦测试显式使用下面 19 个源码／测试文件。缓存、临时文件和日志均留在 worktree：

```sh
task_root="$PWD"
mkdir -p "$task_root/.local/md-yuyuhakusho/tmp"
(
  cd games/yuyuhakusho/engine
  GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
  GOTMPDIR="$task_root/.local/md-yuyuhakusho/tmp" \
  go test -race -count=1 -run '^Test(Fighter0Attacks|ChuAttacks)' -json \
    input.go fighter_input.go animation_clock.go animation_records.go ground_actions.go \
    motion.go jump.go effects.go throw_contacts.go throw_records.go chu_attacks.go \
    fighter0_attacks.go attack_contacts.go effects_test.go jump_test.go \
    throw_contacts_test.go attack_contacts_test.go chu_attacks_test.go fighter0_attacks_test.go \
    > "$task_root/.local/md-yuyuhakusho/fighter0-attacks-native-replay.jsonl"
  GOOS=js GOARCH=wasm GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
  GOTMPDIR="$task_root/.local/md-yuyuhakusho/tmp" \
  go test -count=1 -run '^Test(Fighter0Attacks|ChuAttacks)' -json \
    -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
    input.go fighter_input.go animation_clock.go animation_records.go ground_actions.go \
    motion.go jump.go effects.go throw_contacts.go throw_records.go chu_attacks.go \
    fighter0_attacks.go attack_contacts.go effects_test.go jump_test.go \
    throw_contacts_test.go attack_contacts_test.go chu_attacks_test.go fighter0_attacks_test.go \
    > "$task_root/.local/md-yuyuhakusho/fighter0-attacks-wasm-replay.jsonl"
)
```

特殊交叉复用上述文件清单，以 `go test -race -c -o <独立目录>/native.test` 和 `GOOS=js GOARCH=wasm go test -c -o <独立目录>/cross.test.wasm` 编译同一测试。将 `fighter0-attacks-special-natural.json.gz` 及其 manifest 复制到该独立目录的 `testdata/`，分别命名为 `fighter0-attacks-natural.json.gz`／`fighter0-attacks-natural-manifest.json`，然后在该目录运行测试二进制的 `-test.run '^TestFighter0AttacksOriginalFullCalls/fighter0-attacks-natural$'`。Wasm 通过现有 `go_js_wasm_exec` 执行；用 `go tool test2json -t` 保留结构化日志。这里的别名只存在于独立工作目录，不能覆盖主夹具。

连续流仅推进选择器和其初始化状态，不推进动画时钟、人物运动、子对象、碰撞或世界。部分特殊流在首次成功消费请求后保持无动作，不能作为完整招式／对局验收。当前没有来源 0 自然 AI 覆盖；两份自然输入都是来源 0 真人的观察。
