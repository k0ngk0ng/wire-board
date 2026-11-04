# 来源 1「藏马」攻击选择与立即初始化

本轮恢复 `163fe / 16540 / 165b6 / 1668c` 四个选择器及到达的立即构造。普通与特殊物理输入的完整原 CPU 返回、私有边界和单初态连续流均通过 native race 与 Wasm 对照。本证据覆盖选择和初始化，不代表持续招式、碰撞、画面合成或完整对局已经完成。

## 原版范围与 API

- 原版 ROM SHA-256：`c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06`。
- 原 core SHA-256：`1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da`；被动观察 core 为 `e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306`，两者文件分开保留。
- 既有 [Ghidra 选择器清单](../../../docs/research/md-yuyuhakusho/static/fighter-action-selectors-focused.txt) 与 [构造器清单](../../../docs/research/md-yuyuhakusho/static/fighter-action-constructors-focused.txt) 提供静态依据。REA 不支持本目标的 raw 68000 路由，本轮沿用既有直接 Ghidra 清单，不将其称为 REA 反编译结果。
- 独立实现为 [fighter1_attacks.go](fighter1_attacks.go)，API 为 `SelectFighter1Attack(Fighter1AttackState, ChuAttackMode)`；`Fighter1AttackState` 仅在 `ChuAttackState` 上增加 `Power int16` 视图，对应调用者既有 `+7c` word。

普通空中 A 攻击的方向 `0/2/4/6/8/10` 选择 Pose `95`，其余方向选择 `94`。空中特殊入口 `1668c` 只有 RTS；无论其他模式是否会拒绝同样的 Pool／Throws 存储，本入口都保留全部状态。

| 特殊根 | Request／Changed | 成本 | Action／Pose | 额外写入 |
| --- | --- | --- | --- | --- |
| `18b7a` | 3，无按键门槛 | 16 | `03 / a5` | 无 |
| `18ba4` | 1，B | 16 | `1c / a0` | Power word 清零 |
| `18bbc` | 1，A | 24 | `1c / a1` | Power word 清零 |
| `18bea` | 2，B | 16 | `1b / a2` | VelocityX long=`40000` |
| `18c04` | 2，A | 16 | `1b / a2` | VelocityX long=`60000` |
| `18c36` | 4，C | 24 | `1a / a3` | VelocityX 高 word=`30`，保留 low16 |
| `18c4e` | 4，B | 24 | `1a / a3` | VelocityX 高 word=`70`，保留 low16 |
| `18c66` | 4，A | 24 | `1a / a3` | VelocityX 高 word=`b0`，保留 low16 |
| `18c96` | 6，A，Health≤111 | 200 | `1d / a6` | 无 |

Request 1～4 要求 ChildSize 为零；Request 6 不检查 ChildSize。Request 1、2 的优先级为 B>A；Request 4 为 C>B>A。Health 和 Energy 按 signed word 比较。成功后清 Request、EnabledStages 并执行 `19c1c` 重置；特殊构造保留 Facing。Energy 的 low16 不属于选择器写入范围。

## 先测后实现

[失败场景清单](FIGHTER1_ATTACK_FAILURES.md) 先于新生产文件建立。原 CPU 夹具生成后冻结 [fighter1_attacks_test.go](fighter1_attacks_test.go)，其 SHA-256 为 `feea08162613cb478fa622d48212bb36471a1f813914623008f9793403345968`。

[真实 red](testdata/fighter1-attacks-red.jsonl) 记录缺少 `Fighter1AttackState / SelectFighter1Attack` 导致的构建失败；[先测记录](testdata/fighter1-attacks-test-first.json) 绑定当时的失败清单、采样器、CPU 夹具、冻结测试和 red，确认生产文件不存在。记录中的 special pending 是创建时的历史状态。

首轮实现把普通空中 A 的两个 Pose 常量读反，冻结的完整调用和连续流对照同时捕获该错误。[初始失败记录](testdata/fighter1-attacks-initial-failure.json) 绑定失败时生产 SHA 和原始日志；随后仅修正生产的两个常量，没有改测试或 CPU 夹具。

## 完整返回与自然输入

所有采样都执行原 CPU 完整入口至调用者返回，要求 `exit_sp=entry_sp+4`。对照包括完整 actor、三个 child、其他 world 对象、全部 FX 池、throw 缓冲、两个游标、位置小数、Energy fraction 和共享 RNG，不屏蔽差异。连续流仅改变声明的输入按钮，从单一初态累积执行；每步不注入原版中间状态或 RNG。

| 数据集 | 完整调用 | 范围 |
| --- | --- | --- |
| [普通自然调用](testdata/fighter1-attacks-natural-manifest.json) | 1,640 | 合法菜单来源 1；ground 761、ground-special 773、air 53、air-special 53 |
| [特殊自然调用](testdata/fighter1-attacks-special-natural-manifest.json) | 1,564 | ground 755、ground-special 809；四个专属动作构造根各到达一次 |
| [私有代表](testdata/fighter1-attacks-oracle-manifest.json) | 26 | 四种模式、四个对象地址、九个特殊构造及成本拒绝 |
| [私有边界](testdata/fighter1-attacks-boundaries-manifest.json) | 15,416 | 九个特殊根全部到达；完整按钮 byte、方向、Facing、成本、Health、ChildSize、Power、FX 和投技存储 |
| [连续流](testdata/fighter1-attacks-sequences-manifest.json) | 52 组／1,040 步 | 快移效果池耗尽、普通攻击、特殊构造后的后续无操作与原样空中返回 |

私有数据明确使用合成的调用存储，只证明这些入口对声明状态的操作。私有上下文不证明自然游玩，也不证明角色资源或动画初始化正确。Manifest 的 `frames_match_original_core` 表示变更私有状态之前的被动自然基线一致；私有调用不属于自然帧基线。

普通物理输入为 [source1-next-inputs.json](../../../docs/research/md-yuyuhakusho/source1-next-inputs.json)，SHA-256 为 `cd3dd66e7ec9d50cc89451afdaa98914c5ca9945bb689f4a5f3b4affa326f1ef`。共享 [3602 帧原 core 基线](testdata/fighter1-attacks-reference-manifest.json) 的 frames SHA-256 为 `4c202b99a50b303c04dbfa545015159e3e3d7100f750a75f7abf350882d8f60e`。入口帧上限为 3600，后 2 帧只排空已有调用；被动观察逐帧保持 RAM／video hash 一致。私有 checkpoint 恢复显式使用 `restore_refresh_cycles=True`。

特殊物理输入为 [fighter1-special-inputs.json](testdata/fighter1-special-inputs.json)，SHA-256 为 `4c4ebc2a611db9d98337010cc5d38eea6cabeaf2bf60826387a0f34733a1fd8d`。共享 [特殊 3602 帧原 core 基线](testdata/fighter1-attacks-special-reference-manifest.json) 的 frames SHA-256 为 `d4eafaf4f9a9484d465aa506e1ca63db9308b6bb2fb8a25fbb7d9cc475d22777`。

主流程提供的 [特殊输入发现报告](../../../docs/research/md-yuyuhakusho/source1-special-discovery.json) 按原样保存，SHA-256 为 `0f4988220ac025189a161c931b1d9f6eb83bb2c8e766d4a6a039a5da8a1de220`。本模块另用完整选择器返回观察输入结果；不以其他模块的持续动作数据代替本模块对照。

| 帧 | 自然构造根 | Energy 前→后 | Action／Pose |
| --- | --- | --- | --- |
| 1816 | `18ba4` | 223→207 | `1c / a0` |
| 1989 | `18bea` | 207→191 | `1b / a2` |
| 2089 | `18c36` | 191→167 | `1a / a3` |
| 2978 | `18c96` | 204→4 | `1d / a6` |

四次自然成功都清 Request。九个特殊根的完整分支覆盖由私有矩阵补足；不把四次自然成功写成九个根全部自然触发。

## 验证与复现

普通范围的明确文件清单检查已通过 native race **58.837 秒**、Wasm **13.596 秒**。首轮包级构建被并行模块尚未实现的符号挡住，保留于 `fighter1-attacks-initial-native.jsonl`；普通范围通过记录对应 `fighter1-attacks-ordinary-isolated-native.jsonl` 和 `fighter1-attacks-ordinary-isolated-wasm.jsonl`。两份 `*-command.json` 保留实际 argv、文件清单和工作区内缓存位置。

原 core 基线由 [capture_fighter1_reference.py](../../../scripts/yuyuhakusho/capture_fighter1_reference.py) 唯一生成；该脚本断言原 core SHA。完整调用由 [capture_fighter1_attacks.py](../../../scripts/yuyuhakusho/capture_fighter1_attacks.py) 采样。已有相同输入与基线时复用产物，不再生成另一份 control。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter1_attacks.py \
  --out .local/md-yuyuhakusho/fighter1-attacks-recheck
```

最终正常包级检查覆盖本模块四个顶层测试和既有酎的三个顶层选择器测试。Native race **78.558 秒**、Wasm **18.819 秒**全部通过。对应日志为 [native](testdata/fighter1-attacks-native.jsonl) 和 [Wasm](testdata/fighter1-attacks-wasm.jsonl)；汇总及来源绑定见 [独立验证报告](testdata/fighter1-attacks-validation.json)。测试使用工作区内已有 Go 工具链和缓存。

```sh
task_root="$PWD"
mkdir -p "$task_root/.local/go-tmp" "$task_root/.local/tmp"
GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
  GOTMPDIR="$task_root/.local/go-tmp" TMPDIR="$task_root/.local/tmp" \
  go test -race -count=1 -run '^Test(Fighter1(Attacks|AirSpecial)|ChuAttacks)' \
  -json ./games/yuyuhakusho/engine

GOOS=js GOARCH=wasm \
  GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
  GOTMPDIR="$task_root/.local/go-tmp" TMPDIR="$task_root/.local/tmp" \
  go test -count=1 -run '^Test(Fighter1(Attacks|AirSpecial)|ChuAttacks)' \
  -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
  ./games/yuyuhakusho/engine
```

[清理记录](testdata/fighter1-attacks-cleanup.json) 保留逐文件校验和。本轮删除 16 个自有重复采样／压缩中间文件，共 **26,653,681 字节**；规范夹具、原 core 基线、初始 red、首轮实现失败及通过结果保留，未清理共享缓存或其他工作者文件。
