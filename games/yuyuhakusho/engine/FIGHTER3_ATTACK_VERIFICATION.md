# 来源 3「幽助」攻击选择：实现前验证范围

本阶段只恢复 `16a40/16b82/16bf8/16cc2` 四个选择器的真实完整返回，包括七个特殊构造根 `18ea8/18ec0/18ef0/18f1e/18f82/18f9c/18fda`、地面投技后的 `18e8a`、普通共用构造和冲刺 FX。姓名沿用[原 HUD 字形绑定](../../../docs/research/md-yuyuhakusho/CHARACTER_IDS.md)，真实输入为 [source3-next-inputs.json](../../../docs/research/md-yuyuhakusho/source3-next-inputs.json)。持续槽、`1900c` 终结构造、子对象、完整动作尾段及世界／对局均由后续独立阶段处理。

## 实现前列出的失败场景

来源编号、入口或模式错误；未执行到真正 RTS 后的返回位置或 SP+4；套用幻海／兄的空中投技前置；地面投技记录 SourceAction 仍为 3、父体却进入 `13/83` 的差异丢失；目标姿态 `40`、Y−`20`、宽`18`／高`20`、OffsetX`18` 等记录参数或 word 回绕错误；前冲 `0f/0x40000`、后退 `11/0x34000` 与旧姿态／请求优先级错误；FX 申请、满池、旧槽残留或游标回绕错误；方向表、B／A 优先级、Pressed／Changed／Held／扩展 byte 混用；地面请求 1／2 中立方向与成本 12／16 错误；请求 3 的任一基础攻击 Changed、成本 32、横速最高 byte 清除错误；请求 4 主槽占用、C／B／A 优先级、成本 4、Gravity 最高 byte 或 32／−46 偏移高 word 写入错误；地面请求 5 被误实现；请求 6 的生命 111／成本 200 边界或未写字段被清除；空中特殊错误接受请求 1／2／3；请求 4 高度 `groundHigh−128` 的 signed word 回绕、比较方向、主槽占用或 B／A Changed 门槛错误；空中成本 24、完整 Aux48 弹速 `0x40000/0x60000`、28／−48 偏移高 word 错误；新增重复 Gravity／Aux48 存储；能量低 word、三个子槽、其他人物或 RNG 被改写；重置误清请求、命令、动画标志或未知字节；非法模式、FX 游标或投技存储导致部分输出／调用者 slice 泄漏；逐调用回灌期望中间态伪装连续流；把私有调用、自然无动作返回或目录存在当成自然完整招式／对局。

## 顺序与数据边界

先保存本清单，按真实未注入菜单输入取得未观察／观察原内核完整 RAM／视频基线；再采集完整自然、代表、边界和单初态流夹具；随后编写缺实现测试并保留真实红灯，最后新增独立 Go 模块。现有 Go、数据 bundle、报告、PROGRESS、页面资产与共享缓存不修改。

接口为 `Fighter3AttackState{ChuAttackState; AlternateVelocity int32}` 与 `SelectFighter3Attack(state, ChuAttackMode)`。AlternateVelocity 只是当前完整 Aux48 的视图，不增加 canonical 副本。比较人物 256 bytes、三个子槽 768 bytes、四人物家族 4,096 bytes、FX 2,048 bytes、投技 512 bytes、所有相关游标、能量小数、Aux48 和共享 RNG；私有调用核对完整 RAM 的允许写入范围。自然与私有上下文分开报告，未自然覆盖的构造诚实记零。

## 已验证结果

| 范围 | 完整调用／步骤 | 实际覆盖 |
| --- | ---: | --- |
| 普通菜单输入自然调用 | 1,742 | 地面普通 812、地面特殊 826、空中普通／特殊各 52 |
| 特殊输入自然调用 | 1,942 | 地面普通 922、地面特殊 925、空中普通 47、空中特殊 48 |
| 明确私有代表调用 | 29 | 四入口、七特殊构造成功／能量不足、投技与冲刺 |
| 明确私有边界调用 | 15,144 | 四人物地址；高度 word 回绕、投技坐标、成本、方向、占用与 FX 边界 |
| 单初态流 | 16 条／320 步 | 四模式各四条朝向流；只改变声明的基础 Held／Pressed／Changed |

普通输入 SHA-256 为 `3e787e19b0ddc2215823b85c69517ac10a6f91ca1864e6ba9dc964f2f97e1a84`，原内核从冷启动生成[3,600 帧参考](testdata/fighter3-attacks-reference-frames.json)后，观察内核全部 RAM／视频摘要逐帧一致。普通自然实际命中 `18828` 一次、`18812` 三次。

独立的 [fighter3-special-inputs.json](testdata/fighter3-special-inputs.json) SHA-256 为 `1d917b7eb7749b40f16fb3ff306bc67550957184c1acef4d3f7447b3ca719f30`。共享未观察原内核[3,602 帧参考](testdata/fighter3-attacks-special-reference-frames.json)供选择器、motion、子对象和组合阶段复用；本阶段观察内核全部帧一致，只纳入前 3,600 帧的新入口，末两帧用于等待完整返回。

特殊自然输入实际成功进入以下构造；没有将其他根的私有覆盖称作自然命中：

| 视频帧 | 特殊根 | 请求 | 能量变化 | 结果动作／姿态（hex） |
| --- | --- | ---: | --- | --- |
| 1,823 | `18ef0` | 3 | 223→191 | `21/a6` |
| 1,963 | `18f1e` | 4 | 191→187 | `17/a4` |
| 2,163 | `18ef0` | 3 | 140→108 | `21/a6` |
| 2,210 | `18f1e` | 4 | 114→110 | `17/a4` |
| 2,560 | `18f82` | 4 | 109→85 | `22/ac` |

全部采样执行到实际 RTS 之后，并核对返回 SP 为入口加 4。没有未完成调用、采样错误或私有范围外 RAM 改写。完整对象、FX、投技和相关计数无字节屏蔽；能量小数、三个子槽与共享 RNG 保持原值，实际 RNG 调用次数为零。Aux48 的完整输出逐次比较，包括成功空中构造写入 `0x40000/0x60000`。

原版夹具和测试先于实现，保留仅缺 `Fighter3AttackState`／`SelectFighter3Attack` 的[真实编译失败](testdata/fighter3-attacks-red.jsonl)。生产实现一次通过六项顶层测试，包括旧酎攻击三个回归：原生竞态 79.285 秒、Node/Wasm 18.755 秒。两份自然夹具在首次测试前就已纳入同一组断言，没有实现后追加测试代码。完整来源、夹具／源码摘要和日志见[验证报告](testdata/fighter3-attacks-validation.json)。

## 构造与存储归属

以下成本使用十进制，地址、动作和姿态使用十六进制。

| 请求／模式 | 特殊根 | 成本 | 原行为 |
| --- | --- | ---: | --- |
| 地面 1／2 | `18ea8/18ec0` | 12／16 | 中立方向、对应 B／A Changed；`15/a0` 或 `15/a1`，只清横速最高 byte |
| 地面 3 | `18ef0` | 32 | 任一基础攻击 Changed；`21/a6`，只清横速最高 byte |
| 地面 4 | `18f1e` | 4 | 主子槽空闲，C／B／A 优先级设置 Gravity 模式；高 word 偏移 32／−46；`17/a4` |
| 地面 6 | `18fda` | 200 | 生命不大于 111、A Changed；仅设横速 `0x40000`，`23/aa` |
| 空中 4 | `18f82/18f9c` | 24／24 | 主子槽空闲、高度门槛，B 优先 A；完整 Aux48 弹速 `0x40000/0x60000`，高 word 偏移 28／−48，`22/ac` |

地面请求 5 和空中的其他请求不执行构造。高度门槛为先按 word 回绕计算 `groundHigh−128`，再进行 signed word 比较；仅当结果不大于人物 Y 高 word 时允许，坐标小数不参与门槛。构造保留速度低 word、Gravity 低 24 bits及其他未写字段，特殊构造不重新转向。

前／后冲分别进入 `0f/11`，正横速为 `0x40000/0x34000`。地面前向 A 先提交 Y−`0x20`、宽`0x18`／高`0x20`、SourcePose`b8`／SourceAction3、TargetPose`40`／TargetAction3、TargetFacing1、OffsetX`0x18`／OffsetY0 的记录，再通过 `18e8a` 进入父动作 `13/83`、横速 `0x40000` 并按保留方向转向。记录中的动作 3 不能误改成父体选择后的 `13`。

空中普通 selector 没有投技前置，不访问投技存储。测试另验证其在不可访问投技切片／无关 FX 游标下仍完成普通攻击，避免引入来源 0／10 的错误前置。未知模式、实际会用到的 FX 游标或投技存储错误均整体返回初态，调用者 slice 不变；任意原版无效地址别名不属于本语义接口。

状态只增加现有完整 Aux48 的视图 `AlternateVelocity`，并返回给调用者合并，不能另存一个弹速 long；能量小数和 RNG 继续由调用方持有。普通构造、重置、扣能量和分配继续复用已验证的小 helper，没有复制动作尾段、ROM 读取或指令 VM。

## 复跑

从 worktree 根目录运行，固定研究工具按[说明](../../../scripts/yuyuhakusho/README.md)准备。两个输入保持独立：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter3_attacks.py \
  --out .local/md-yuyuhakusho/fighter3-attacks-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter3_attacks.py \
  --out .local/md-yuyuhakusho/fighter3-attacks-special-replay \
  --inputs games/yuyuhakusho/engine/testdata/fighter3-special-inputs.json \
  --baseline games/yuyuhakusho/engine/testdata/fighter3-attacks-special-reference-frames.json \
  --natural-entry-limit 3600 --natural-only
```

特殊输出目录中的 `fighter3-attacks-natural` 对应已保存的 `fighter3-attacks-special-natural` 前缀；不要覆盖普通自然夹具。两次原内核参考的身份、输入及全部帧摘要分别保存在对应 reference-manifest 中。

主聚焦验证显式使用 19 个源码／测试文件，复用已有字节适配器；它不替代完整包或后续世界组合门禁。

```sh
task_root="$PWD"
mkdir -p "$task_root/.local/md-yuyuhakusho/tmp"
(
  cd games/yuyuhakusho/engine
  GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
  GOTMPDIR="$task_root/.local/md-yuyuhakusho/tmp" \
  go test -race -count=1 -run '^Test(Fighter3Attacks|ChuAttacks)' -json \
    input.go fighter_input.go animation_clock.go animation_records.go ground_actions.go \
    motion.go jump.go effects.go throw_contacts.go throw_records.go chu_attacks.go \
    fighter3_attacks.go attack_contacts.go effects_test.go jump_test.go \
    throw_contacts_test.go attack_contacts_test.go chu_attacks_test.go fighter3_attacks_test.go \
    > "$task_root/.local/md-yuyuhakusho/fighter3-attacks-native-replay.jsonl"
  GOOS=js GOARCH=wasm GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
  GOTMPDIR="$task_root/.local/md-yuyuhakusho/tmp" \
  go test -count=1 -run '^Test(Fighter3Attacks|ChuAttacks)' -json \
    -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
    input.go fighter_input.go animation_clock.go animation_records.go ground_actions.go \
    motion.go jump.go effects.go throw_contacts.go throw_records.go chu_attacks.go \
    fighter3_attacks.go attack_contacts.go effects_test.go jump_test.go \
    throw_contacts_test.go attack_contacts_test.go chu_attacks_test.go fighter3_attacks_test.go \
    > "$task_root/.local/md-yuyuhakusho/fighter3-attacks-wasm-replay.jsonl"
)
```

单初态流只推进选择器与其初始化状态，没有动画时钟、持续动作、子对象更新或碰撞。特殊流消费请求后可保持无动作，不能用来声称完整招式。自然输入没有来源 3 AI 覆盖；完整动作／输入／人物／世界及可玩验收仍由组合阶段完成。本阶段没有修改既有 Go、bundle、PROGRESS、其他报告或正在服务的页面资产。
