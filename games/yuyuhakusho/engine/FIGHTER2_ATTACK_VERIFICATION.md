# 来源 2「桑原」攻击选择：实现前验证范围

本阶段恢复 `16714/16856/168cc/169b0` 四个选择器的真实 RTS 后 SP+4 返回，包括 12 个特殊构造根、地面投技与冲刺 FX。名称绑定[原 HUD 与菜单发现](../../../docs/research/md-yuyuhakusho/source2-next-selection.json)，真实未注入输入为 [source2-next-inputs.json](../../../docs/research/md-yuyuhakusho/source2-next-inputs.json)。持续动作、子对象和完整世界由后续独立阶段处理，不修改当前幽助检查点。

## 实现前列出的失败场景

来源 ID、入口或模式错误；只观察构造而未等真正返回；SP／返回位置错误；冲刺请求 8 优先级、请求 7 与旧姿态 `0b` 判断错误；前 `0f/0x40000`、后 `11/0x38000` 或 FX 方向／池游标／满池／旧槽残留错误；地面投技 Y−`24`、宽`1a`／高`24`、目标`40`、源／目标动作3和OffsetX`18`的 word 回绕错误；套入其他来源空中投技；Held／Pressed／Changed／扩展 byte 混用；地面请求1擅加 Changed／按键门槛，三方向组／成本12或普通动作姿态错误；仅11项的 `1690c..16921` 被补成16项；索引11..15的相邻指令字误当正规表项；未运行诊断就默认动作或称原版无动作；诊断使用不同寄存器／RAM上下文却隐藏差别；执行预算结束、异常、未返回和正常返回混为一谈；只保存已知字段而漏掉非人物写入；把正常模式16项方向也一并拒绝；地面请求2误执行；请求3／4主槽占用、B／A或C／B／A优先级错误；完整横速写与Gravity高word清理、横速高word偏移写入混淆；未拥有低word／Aux48／能量小数丢失；请求5成本16及两姿态错误；请求6生命111／成本200／普通动作3被套成专属持续；空中请求1擅加方向／Changed／主槽门槛；重置误清请求／命令／标志；共享RNG或子槽被改写；非法模式、实际使用的FX游标／投技存储造成部分更新或调用者slice泄漏；连续流回灌期望中间态；将私有或诊断上下文称作自然招式／完整对局。

## 取证顺序与隔离边界

先保存本清单，按真实菜单输入取得原未观察／观察内核的完整 RAM／视频基线；正常请求1只在0..10表结构范围内采集自然／私有／连续返回。索引11..15另做有界原CPU诊断，保存完整RAM及寄存器、声明上下文、实际返回／异常／预算结束，不混入正常调用统计。根据实际证据确定接口支持边界；Go的有界“不支持”错误不能描述为原版 no-op。

全部原版夹具和实际缺符号测试红灯先于生产代码。接口复用 `SelectFighter2Attack(ChuAttackState, ChuAttackMode)`，不新增canonical存储。正常快照比较父体256 bytes、三子槽768 bytes、四人物家族4096 bytes、FX2048 bytes、投技512 bytes、游标、能量小数、Aux48和RNG；私有调用核对完整RAM的允许写入范围。诊断保存这些语义范围之外的完整64KiB RAM，不用RAM指令机补未知行为。

## 已验证结果

| 范围 | 完整调用／步骤 | 实际覆盖 |
| --- | ---: | --- |
| 普通自然输入 | 1,580 | 地面普通728／特殊746，空中普通／特殊各53 |
| 特殊自然输入 | 2,245 | 地面普通1,121／特殊1,124；`18de0/18d56/18d9e/18d3e`各一次自然构造入口 |
| 明确私有代表 | 31 | 四入口、12特殊构造、普通投技和冲刺 |
| 正常有界私有矩阵 | 20,476 | 请求1只用结构索引0..10；其他模式全部16方向及字节、成本、占用、游标边界 |
| 单初态流 | 16条／320步 | 只改变声明的基础Held／Pressed／Changed |
| 独立结构外诊断 | 15 | 三种声明寄存器上下文×索引11..15；不计入正常完整调用 |

原菜单输入SHA-256为 `43a1cab4f2147616db4798cd5ab88d6c208f2311d5b99468816f3265d9fee596`，从冷启动取得[3,600帧未观察参考](testdata/fighter2-attacks-reference-frames.json)后，观察内核全部RAM／视频帧一致。自然普通构造实际为 `18828`两次、`18812`一次。

唯一采用的特殊输入为 [fighter2-projectile-inputs.json](testdata/fighter2-projectile-inputs.json)，SHA-256 `90d7d03e229a1cdd4eac98273f867f3c8118a34cfcdbe275d1932e630c529af8`；原内核共享[3,602帧参考](testdata/fighter2-attacks-special-reference-frames.json)供并行阶段复用。本观察器全帧一致，纳入前3,600帧入口，末两帧等待完整返回。不采用其他发现输入或把不同输入的基线混合。

所有正常完整调用均实际通过RTS到达返回位置，并核对SP+4；没有未完成、采样错误、私有允许范围外RAM改写或字节屏蔽。能量低word、Aux48、子槽和共享RNG保持原值，正常RNG调用次数为零。私有矩阵命中12特殊根、9普通共用构造及地面投技提交；自然只有上述有限构造，未自然覆盖的根不作外推。

原版五组夹具和独立诊断、四项新测试先于实现，保留只缺 `SelectFighter2Attack`的[实际编译失败](testdata/fighter2-attacks-red.jsonl)。生产首次实现通过七项顶层测试：旧酎攻击三项及桑原四项（含诊断）。原生竞态103.570秒、Node/Wasm23.678秒；没有实现后修改测试或生产修正。[验证报告](testdata/fighter2-attacks-validation.json)绑定全部输入、参考、源码、夹具、红绿日志与支持边界。

## 方向表结构、真实诊断与API边界

`1690c..16921`只有11个结构项：0／1／3／7→`18cc0`、4／5／8／9→`18cea`、2／6／10→`18d14`。这三组均不要求Changed，成本12，进入 `03/a0..a2`。表后相邻指令字计算出的目标，不自动成为正常表项。

诊断恢复原frame2400核心检查点，以真实来源2自然入口的完整RAM为基础，分别使用清零寄存器、自然寄存器移植、非零D4/A4三种声明上下文；统一私有SP／返回、关闭中断，并把请求设1、方向设11..15、能量高word设100。每次预算100,000 cycles，保存完整RAM／寄存器、改写范围、观察事件、原核心状态及执行结束状态。检测到返回／已登记异常向量目标即停，否则记录预算结束。

| 索引 | 静态计算目标 | 三上下文实测 | API策略 |
| --- | --- | --- | --- |
| 12 | `18cc0` | 均SP+4返回，成本12、`03/a0`、RNG不变，输出由现有语义状态闭合 | 支持独立验证的结构外别名 |
| 14 | `18cea` | 均SP+4返回，成本12、`03/a1`、RNG不变，输出闭合 | 支持独立验证的结构外别名 |
| 11／13 | `1c922/1c926` | 均能返回，但进入其他动作尾段，修改图形／`14/ae`／RNG；非零A4还改变人物header | 原子unsupported，不模拟寄存器或任意RAM写入 |
| 15 | `1c92a` | 三次均在预算内反复重入selector并增长栈，未返回；没有观察到已登记异常向量目标 | 原子unsupported，不称原版无动作 |

返回快照在sentinel指令执行前保存；核心可能再执行该一条指令才响应停止预算，因此执行结束PC／核心状态另外保存，不能与SP+4边界快照混用。诊断中的“未观察到异常目标”不等于证明所有异常状态均已排除；“预算内未返回”也不等于证明无限循环。

最终有界API只在 **地面特殊请求1** 拒绝11／13／15；支持0..10及实测12／14别名。不删除结构内的矛盾方向3／7，也不限制其他模式的16项方向表。独立[诊断夹具](testdata/fighter2-attacks-direction-diagnostics-manifest.json)保留原执行结果；Go返回错误是接口支持政策，绝不当作原版no-op。正常矩阵、诊断及其Go策略断言分开统计。

## 构造与字段保留

特殊构造根为 `18cc0/18cea/18d14/18de0/18e00/18d3e/18d56/18d6e/18d9e/18db6/18e60/18e36`。下表成本为十进制，动作／姿态及参数为十六进制。

| 模式／请求 | 门槛与结果 |
| --- | --- |
| 地面1 | 已支持方向组，成本12；无Changed或子槽门槛，`03/a0..a2` |
| 地面3 | 主槽空闲，B优先A Changed，成本18；`20/a8`，横速完整写40000／80000，仅清Gravity高word |
| 地面4 | 主槽空闲，C／B／A Changed优先，成本20；`1e/a6`，仅横速高word偏移30／70／b0 |
| 地面5 | B优先A Changed，成本16；`1f/a4`或`1f/a5`，不改速度 |
| 地面6 | 生命≤111、A Changed、成本200；普通 `03/aa`，不额外清速度／重力 |
| 空中1 | 直接成本12进入 `02/a3`；无方向、Changed或主槽门槛 |

地面2及其他空中请求保持。字段别名均使用已有Body存储，低word、Aux48和能量小数保留。地面前向A先提交SourceAction3／TargetAction3／TargetPose40、Y−24、宽1a／高24、OffsetX18／Y0／Facing1的记录，再进入父 `03/83`；空中普通没有投技前置。普通构造、重置、扣能量和分配继续复用已验证小helper，没有完整动作尾段副本或ROM／指令VM依赖。

## 复跑

从worktree根目录运行，固定工具按[研究说明](../../../scripts/yuyuhakusho/README.md)准备。正常与诊断独立输出：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter2_attacks.py \
  --out .local/md-yuyuhakusho/fighter2-attacks-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter2_attacks.py \
  --out .local/md-yuyuhakusho/fighter2-attacks-special-replay \
  --inputs games/yuyuhakusho/engine/testdata/fighter2-projectile-inputs.json \
  --baseline games/yuyuhakusho/engine/testdata/fighter2-attacks-special-reference-frames.json \
  --natural-entry-limit 3600 --natural-only
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/diagnose_fighter2_directions.py \
  --context games/yuyuhakusho/engine/testdata/fighter2-attacks-diagnostic-context.json.gz \
  --out .local/md-yuyuhakusho/fighter2-attacks-direction-replay
```

特殊输出的 `fighter2-attacks-natural` 对应已保存的独立 `fighter2-attacks-special-natural`，不要覆盖主自然。诊断context保留真实入口寄存器／RAM与原检查点，身份及捕获来源分别绑定；原程序的私有输入替换完整声明。

聚焦验证使用下列19个源码／测试文件；缓存、临时文件和日志保持worktree内：

```sh
task_root="$PWD"
mkdir -p "$task_root/.local/md-yuyuhakusho/tmp"
(
  cd games/yuyuhakusho/engine
  GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
  GOTMPDIR="$task_root/.local/md-yuyuhakusho/tmp" \
  go test -race -count=1 -run '^Test(Fighter2Attacks|ChuAttacks)' -json \
    input.go fighter_input.go animation_clock.go animation_records.go ground_actions.go \
    motion.go jump.go effects.go throw_contacts.go throw_records.go chu_attacks.go \
    fighter2_attacks.go attack_contacts.go effects_test.go jump_test.go \
    throw_contacts_test.go attack_contacts_test.go chu_attacks_test.go fighter2_attacks_test.go \
    > "$task_root/.local/md-yuyuhakusho/fighter2-attacks-native-replay.jsonl"
  GOOS=js GOARCH=wasm GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
  GOTMPDIR="$task_root/.local/md-yuyuhakusho/tmp" \
  go test -count=1 -run '^Test(Fighter2Attacks|ChuAttacks)' -json \
    -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
    input.go fighter_input.go animation_clock.go animation_records.go ground_actions.go \
    motion.go jump.go effects.go throw_contacts.go throw_records.go chu_attacks.go \
    fighter2_attacks.go attack_contacts.go effects_test.go jump_test.go \
    throw_contacts_test.go attack_contacts_test.go chu_attacks_test.go fighter2_attacks_test.go \
    > "$task_root/.local/md-yuyuhakusho/fighter2-attacks-wasm-replay.jsonl"
)
```

本阶段没有持续时钟／运动、子对象生命周期、碰撞、完整action／actor／world或对局验收；连续流只改变声明输入，消耗请求后可保持无动作。自然输入没有来源2 AI覆盖。既有Go、bundle、PROGRESS、其他报告及页面资产未由本阶段修改。
