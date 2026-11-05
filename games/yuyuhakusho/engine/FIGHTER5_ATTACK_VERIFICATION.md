# 来源 5「仙水」攻击选择与后续构造

四个选择器 `1705c/1719e/17214/172c4`、十个特殊根和五个无成本后续构造已实现。普通及特殊自然、私有边界、单初态流与原 CPU 完整返回一致；native race 和 Wasm 均通过。十个特殊根均有合法固定物理输入下的自然成功证据。五个后续构造的自然范围单独列明。

## 接口与字段所有权

[fighter5_attacks.go](fighter5_attacks.go) 提供三个接口：

- `SelectFighter5Attack(Fighter5AttackState, ChuAttackMode)`：四个选择器及完整构造返回。
- `FinishFighter5Combination(Fighter5AttackState, alternate bool)`：`false` 对应 `1926a`，进入 `03/a5`；`true` 对应 `192ae`，进入 `03/a7`。原 motion 以 `Pose!=a4` 选择后者。
- `BeginFighter5FinalProjectile(Fighter5AttackState, variant uint8)`：0/1/2 对应 `19408/19426/19444`；其他值原子返回错误。原 motion 自行判断 variant，不将无效值调用此接口。

`Fighter5AttackState` 嵌入既有 `ChuAttackState`，仅增加 `AlternateVelocity int32` 作为 `+48` full long 的唯一存储。`+4c` 继续使用 `Body.Gravity`。最高 byte 与高 word 均为临时视图，未增加模式或重复计数器。

## 分支合同

地面快移 Request 8 优先，进入 `11/0c`、VX=`38000`；Request 7 且 Pose≠11 才进入 `0f/0b`、VX=`40000`。普通攻击以 B 优先。横向 A 投技使用 Y 高 word−`28`、W=`18`/H=`28`，保留 VX。空中普通没有额外投技。

| 根 | 条件 | 成本 | Action／Pose | 额外写入 |
| --- | --- | --- | --- | --- |
| `19216` | Ground R3，Changed B 或 A | 8 | `2d/a8` | 无 |
| `19240/19284` | Ground R1，无 Changed 门槛；Held.bit1 选择后者 | 16 | `2e/a4` 或 `2e/a6` | 无 |
| `192c8/192f6/19324` | Ground R4，主 child.Size=0；Changed A>B>C | 4 | `2f/a0,a1,a2` | VX/VY 分别 `0/48000`、`28000/0`、`10000/10000`；Gravity 最高 byte=0/1/2 |
| `19364` | Air R4，主 child.Size=0；signed wrap(Ground.high−128)≤Y.high；Changed A/B/C | 4 | `30/ac` | VX/VY=`10000/10000`；Gravity 最高 byte=3 |
| `193a4/193bc/193d4` | Ground R6，Health≤111、主 child.Size=0；Changed A>B>C | 50 | `31/a3` | Gravity 最高 byte=0/1/2；仅清 Aux48 高 word |
| `1926a/192ae` | 调用者选择的无成本后续 | 0 | `03/a5` 或 `03/a7` | 保留速度、Aux48、Gravity、Facing 与 Energy |
| `19408/19426/19444` | variant 0/1/2 的无成本后续 | 0 | `32/a0,a1,a2` | 与 Ground R4 相同速度和 Gravity byte；保留 Aux48 |

成功特殊与后续构造清 Request、Stages，执行完整 Reset。Energy 和 Health 使用 signed word；Energy fraction、Gravity 低 24 位、Aux48 未写的低 word 均保留。

## 原版依据与先测记录

原 ROM SHA-256 为 `c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06`。未观察 core 为 `1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da`，观察 core 为 `e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306`。静态依据复用直接 Ghidra 选择器、构造和 motion 清单，绑定见 [静态报告](testdata/fighter5-attacks-static-binding.json)。REA 不支持 raw 68000，本轮未冒称 REA 反编译。

[失败场景](FIGHTER5_ATTACK_FAILURES.md) 先于测试与实现建立。原 CPU 夹具完成后冻结 [测试](fighter5_attacks_test.go)，SHA-256 为 `4d687d9aa63d1c2d33e091a4af0b749bde08775f046b24f635b491d37ca63008`。[先测记录](testdata/fighter5-attacks-test-first.json) 绑定生产不存在、所有先行夹具与原始 red。首次显式文件检查同时缺少共享新增 child type；加入 motion 的纯类型声明依赖后，[独立缺符号 red](testdata/fighter5-attacks-own-symbol-red.jsonl) 仅缺少本模块状态及三个接口，SHA-256 为 `eb9cbd0b0c431df54365872433200008fe3f3eb3b82705bc8af41ac6c40e3262`。

首轮生产 SHA-256 为 `c92ece3a1159fb48e91f37566f3e6c942764a4ad4439f392e0af056f49c0760b`。实现后未改冻结测试、原 CPU 夹具或断言。

| 数据集 | 完整调用 | 范围 |
| --- | --- | --- |
| [普通自然](testdata/fighter5-attacks-natural-manifest.json) | 1,964 | ground 861、ground-special 875、air 114、air-special 114；74 次 IRQ |
| [特殊自然](testdata/fighter5-attacks-special-natural-manifest.json) | 1,462 | ground 661、ground-special 669、air 65、air-special 66、final-2 1；十特殊根各成功一次，96 次 IRQ |
| [私有代表](testdata/fighter5-attacks-oracle-manifest.json) | 34 | 四选择器、五后续、四对象地址、成功与成本拒绝 |
| [私有边界](testdata/fighter5-attacks-boundaries-manifest.json) | 33,280 | 每地址 8,320；十特殊根、五后续、全部输入 byte、16 个 throw 槽、signed 成本／HP／高度、脏别名 |
| [单初态流](testdata/fighter5-attacks-sequences-manifest.json) | 108 组／1,728 步 | 九类入口，每组仅改变声明的基础按钮 |

每次调用执行到实际调用者返回，核对 `exit_sp=entry_sp+4`。保留完整 64 KB RAM 差异、前后 SHA、栈写入、IRQ 入口／出口与每一步字节变化；外部差异由实际 IRQ 链逐字节解释，未解释变化为 0。Go 同时比较完整 actor、三个 child、其他 world、FX、throw、游标、位置小数、Energy fraction 与 RNG。所有调用的共享 RNG 调用数为 0。

私有脏存储不证明自然角色资源初始化，也不替代后续 motion、子对象、碰撞或完整对局验收。Manifest 的 `frames_match_original_core` 仅证明私有输入前的被动自然 warmup 与原 core 一致。

## 基线与复现

普通 [物理输入](../../../docs/research/md-yuyuhakusho/source5-next-inputs.json) SHA-256 为 `6dd2eea8f585b62ad886da67d1123c61e06bedb97da4fdd9fd768b8441969601`；唯一 [未观察原 core 3602 基线](testdata/fighter5-attacks-reference-manifest.json) 的 frames SHA-256 为 `f43682106819d7847243a459c9bac6b60ddc63d5f5f9bb9a58c633a8154d7c2b`。control 由主流程一次生成，本模块复用其输入、manifest 和逐帧 RAM/video，不另产 control。

特殊 [固定物理输入](../../../docs/research/md-yuyuhakusho/source5-special-discovery-inputs.json) SHA-256 为 `e6ca2bbb16821f13c2508661b3b1b4ebddcad141f40ed7ef99fac0fea2f54af9`；唯一 [特殊未观察原 core 3602 基线](testdata/fighter5-attacks-special-reference-manifest.json) 的 frames SHA-256 为 `8f813685d06dfbe1ca8d9615cd027aef04b130790efe5e6dd66b8395d795bed3`。相同固定输入由本模块冷启动复放，3602 步 RAM/video 均一致。

| 帧 | 特殊根 | Energy 前→后 | Action／Pose | HP |
| --- | --- | --- | --- | --- |
| 1808 | `19240` | 223→207 | `2e/a4` | 223 |
| 1879 | `19284` | 207→191 | `2e/a6` | 223 |
| 1953 | `19216` | 191→183 | `2d/a8` | 223 |
| 2055 | `192c8` | 183→179 | `2f/a0` | 223 |
| 2117 | `192f6` | 179→175 | `2f/a1` | 223 |
| 2181 | `19324` | 175→171 | `2f/a2` | 223 |
| 2247 | `19364` | 171→167 | `30/ac` | 223 |
| 3179 | `193a4` | 167→117 | `31/a3` | 93 |
| 3335 | `193bc` | 117→67 | `31/a3` | 70 |
| 3387 | `193d4` | 67→17 | `31/a3` | 51 |

`19444` 在 3419 帧自然进入 `32/a2`，Energy 保持 17、Aux48 高 word 保持 32。`1926a/192ae/19408/19426` 在此场景未观察到自然调用，仅有私有与单初态流覆盖。每根的完整 RAM 前后 SHA 与状态见 [成功根记录](testdata/fighter5-attacks-special-successes.json)，不将私有覆盖冒称自然执行。

[采集器](../../../scripts/yuyuhakusho/capture_fighter5_attacks.py) 使用 48 个显式 hook point，超过 64 时直接失败。仅接收原 core 未观察 control，完整复放 3602 步，3600 帧之后只排空已进入的调用。私有恢复始终使用 `restore_refresh_cycles=True`。

普通范围三个顶层测试在显式文件清单下通过：[native race](testdata/fighter5-attacks-initial-native.jsonl) **133.718 秒**、[Wasm](testdata/fighter5-attacks-initial-wasm.jsonl) **30.150 秒**。新增特殊自然使用同一冻结子项通过：[native race](testdata/fighter5-attacks-special-native.jsonl) **6.787 秒**、[Wasm](testdata/fighter5-attacks-special-wasm.jsonl) **1.398 秒**。对应命令与环境保存在相邻 `*-command.json`。两轮之间实现、冻结测试和普通夹具均未修改。

最终正常包级验收也通过全部三个攻击测试及四个完整返回子项：[native race](testdata/fighter5-attacks-native.jsonl) **139.721 秒**、[Wasm](testdata/fighter5-attacks-wasm.jsonl) **32.383 秒**。首轮生产、冻结测试与先行夹具保持相同 SHA；[独立验证汇总](testdata/fighter5-attacks-validation.json) 绑定全部六份检查、五组夹具、自然范围和清理记录。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter5_attacks.py \
  --reference-manifest games/yuyuhakusho/engine/testdata/fighter5-attacks-reference-manifest.json \
  --out .local/md-yuyuhakusho/fighter5-attacks-capture

PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter5_attacks.py \
  --reference-manifest games/yuyuhakusho/engine/testdata/fighter5-attacks-special-reference-manifest.json \
  --natural-only --natural-name fighter5-attacks-special-natural \
  --out .local/md-yuyuhakusho/fighter5-attacks-special-capture

GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
  GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
  go test -race -count=1 -run '^TestFighter5Attacks' -json ./games/yuyuhakusho/engine

GOOS=js GOARCH=wasm \
  GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
  GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
  go test -count=1 -run '^TestFighter5Attacks' -json \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

最后两条为已通过的正常包级复现命令。其他来源 5 模块的组合与完整世界验收由对应范围维护。

[清理记录](testdata/fighter5-attacks-cleanup.json) 仅清理本采集器的临时副本；正式原 CPU 夹具、原始 red、先测 SHA、测试日志与命令均保留，共享缓存和其他工作者文件未删除。
