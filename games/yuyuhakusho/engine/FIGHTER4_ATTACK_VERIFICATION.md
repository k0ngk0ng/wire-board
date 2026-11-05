# 来源 4「飞影」攻击选择与后续构造

本轮恢复四个选择器 `16d86/16ec8/16f3e/16fcc`、九个特殊根，以及 motion 调用的两个无成本构造 `19092/191ba`。普通与特殊自然输入、私有边界和单初态流均通过 native race／Wasm 完整对照。本模块测试限于选择／构造返回；另行发现的双真人吞龙完整原版生命周期独立记录，不混入选择器或原单人场景的验收范围。

## 接口与原版依据

独立实现 [fighter4_attacks.go](fighter4_attacks.go) 复用 `ChuAttackState`，没有新增 canonical 字段：

- `SelectFighter4Attack(ChuAttackState, ChuAttackMode)`。
- `FinishFighter4Rush(ChuAttackState)`：`19092`，进入 `03/a1`、Stages=0、执行 Reset，保留 Request 与 Energy。
- `BeginFighter4DragonFollowup(ChuAttackState)`：`191ba`，进入 `2a/a7`、Request=0、Stages=0、执行 Reset，不扣 Energy。

两个后续构造不写 MovementSource／GravityParameter。motion 在调用前对 MovementSource 的变更由调用者保留，不能从本模块的私有后续构造夹具推断吞龙资源或参数路径正确。

原 ROM SHA-256 为 `c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06`。未观察 core 为 `1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da`，观察 core 为 `e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306`，两个文件分开保留。静态依据为既有直接 [Ghidra 选择器](../../../docs/research/md-yuyuhakusho/static/fighter-action-selectors-focused.txt) 和 [构造器清单](../../../docs/research/md-yuyuhakusho/static/fighter-action-constructors-focused.txt)；REA 不支持 raw 68000，本轮没有冒称 REA 反编译。

## 分支合同

地面前快移为 `0e/0b`、VX=`3c000`，后快移为 `12/0c`、VX=`38000`。Request 8 优先；Request 7 且 Pose≠11 才前快移。普通攻击仍以 B 优先；空中普通不额外投技。

| 根 | 条件 | 成本 | Action／Pose | 额外写入 |
| --- | --- | --- | --- | --- |
| `1904e` | Ground R1，无 Changed 门槛 | 8 | `25/a0` | VX 按 Facing.bit0 为 ±`28000` |
| `190a8` | Ground R2，无 Changed 门槛 | 32 | `03/a2` | 无 |
| `190d2` | Ground R3，Changed B | 16 | `26/a4` | 无 |
| `190f0` | Ground R3，Changed A | 24 | `27/a5` | A 优先于 B |
| `1911a` | Ground R4，Changed B | 24 | `2c/a8` | VY=`-68000`、Gravity=`5000`、VX 按 bit0 为 ±`18000` |
| `1913c` | Ground R4，Changed A | 24 | `2c/a8` | Gravity=`3200`；其余同前，A 优先 |
| `19190` | Ground R6，Health≤111、主 child.Size=0、B 或 A 任一 | 200 | `29/a6` | 无 |
| `191d4` | Air R3，Changed B | 16 | `28/ac` | 无 |
| `191ec` | Air R3，Changed A | 24 | `28/ad` | A 优先于 B |

成功特殊构造清 Request、Stages，执行 `19c1c` 重置并保留 Facing。Health／Energy 为 signed word；Energy fraction 保留。R1 与 R4 的速度看 Facing 最低位，不能将任意非零 Facing 都当成负方向。

## 先测后实现

[失败场景](FIGHTER4_ATTACK_FAILURES.md) 先于代码建立。生成原 CPU 夹具后冻结 [测试](fighter4_attacks_test.go)，SHA-256 `946cdf969aa7b6fedd85cac2b44201e9331a4a9470bf54cc30a96053e45ba9e6`。[真实 red](testdata/fighter4-attacks-red.jsonl) 缺少选择函数与两个 helper；[先测记录](testdata/fighter4-attacks-test-first.json) 确认生产文件不存在，绑定 12 项先行证据。

首轮实现即通过普通范围，生产 SHA-256 为 `ca87b180ace1aaff0c9e48ec2cc21f51c21d67351f88f5fe7eab1dd8753c54a6`。冻结测试及先行 CPU 夹具没有为实现改动。

| 数据集 | 完整调用 | 范围 |
| --- | --- | --- |
| [普通自然](testdata/fighter4-attacks-natural-manifest.json) | 1,928 | ground 912、ground-special 926、air 45、air-special 45 |
| [特殊自然](testdata/fighter4-attacks-special-natural-manifest.json) | 1,225 | ground 493、ground-special 646、air 42、air-special 44；九个特殊根各自然成功一次 |
| [私有代表](testdata/fighter4-attacks-oracle-manifest.json) | 29 | 四个选择器、两个后续构造、四地址、成功与成本拒绝 |
| [私有边界](testdata/fighter4-attacks-boundaries-manifest.json) | 22,428 | 九特殊根全覆盖；两个后续构造各 1,080；Facing 0/1/2/3/128/255、成本、HP、主 child、FX／throw |
| [单初态流](testdata/fighter4-attacks-sequences-manifest.json) | 84 组／1,344 步 | 四个选择器与两后续构造，只有声明的按钮外部变化 |

所有调用执行到真实调用者返回并核对 `exit_sp=entry_sp+4`，不在构造中途截断。比对完整 actor、三个 child、其他 world、FX、throw、两个游标、位置小数、Energy fraction 与 RNG，不屏蔽差异。两个 followup 在本模块中采用独立私有调用；其自然 motion 路径由 motion 模块取证。

私有合成上下文不证明自然游玩或角色资源初始化。Manifest 的 `frames_match_original_core` 表示私有变更前的被动自然 warmup 与原 core 一致，不能当作私有调用的自然帧证明。

## 基线与复现

普通 [物理输入](../../../docs/research/md-yuyuhakusho/source4-next-inputs.json) SHA-256 为 `e00525d883a103b4631332ac793b2569ff0ee384134e8fd58407bbc4123d0352`；唯一 [原 core3602 基线](testdata/fighter4-attacks-reference-manifest.json) 的 frames SHA-256 为 `891f1e493f3e3d8bd34e6bf4789739364c6a3f3dc3d318d846d3b797f581d658`。

特殊 [固定物理输入](../../../docs/research/md-yuyuhakusho/source4-special-discovery-inputs.json) SHA-256 为 `5ba6b13d2bb0b285c4307981a5e903d75a7936de3d2632a9f1bf449189650424`；唯一 [特殊原 core3602 基线](testdata/fighter4-attacks-special-reference-manifest.json) 的 frames SHA-256 为 `666c2a9fff6c47b3b0540275beae5a7a1c0f6ada1e1d690d6a67d1596617b22e`。输入先由合法按钮记录，再从冷启动固定复放，未注入 HP、Energy、Request 或其他中间状态。

| 帧 | 特殊根 | Energy 前→后 | Action／Pose |
| --- | --- | --- | --- |
| 1874 | `1904e` | 223→215 | `25/a0` |
| 1925 | `190a8` | 215→183 | `03/a2` |
| 1999 | `190d2` | 183→167 | `26/a4` |
| 2051 | `190f0` | 167→143 | `27/a5` |
| 2190 | `1911a` | 143→119 | `2c/a8` |
| 2388 | `1913c` | 119→95 | `2c/a8` |
| 2624 | `191d4` | 95→79 | `28/ac` |
| 2653 | `191ec` | 81→57 | `28/ad` |
| 3434 | `19190` | 202→2 | `29/a6` |

R6 时实际 HP=95、主 child.Size=0。该场景另观察到 `19092` 第 1897 帧、`191ba` 第 3489 帧；`2a` 只有 1 帧，未吞龙。完整吞龙由独立双真人输入实现，见 [发现与生命周期说明](../../../docs/research/md-yuyuhakusho/source4-special-discovery.md)。

原 control 由 [capture_fighter4_reference.py](../../../scripts/yuyuhakusho/capture_fighter4_reference.py) 唯一生成，观察由 [capture_fighter4_attacks.py](../../../scripts/yuyuhakusho/capture_fighter4_attacks.py) 完成。3600 帧之后只排空已有调用；逐帧核对 RAM／video。私有恢复显式使用 `restore_refresh_cycles=True`，已有相同输入和基线时复用，不另产 control。

普通明确文件清单检查 native race **84.129 秒**、Wasm **19.287 秒**通过。最终正常包级检查覆盖本模块四个顶层测试和既有酎三个顶层测试：[native race](testdata/fighter4-attacks-native.jsonl) **102.395 秒**、[Wasm](testdata/fighter4-attacks-wasm.jsonl) **24.162 秒**，全部通过。首轮后生产与冻结测试均未修改。

```sh
task_root="$PWD"
mkdir -p "$task_root/.local/go-tmp" "$task_root/.local/tmp"
GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
  GOTMPDIR="$task_root/.local/go-tmp" TMPDIR="$task_root/.local/tmp" \
  go test -race -count=1 -run '^Test(Fighter4(Attacks|AirSpecial)|ChuAttacks)' \
  -json ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm \
  GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
  GOTMPDIR="$task_root/.local/go-tmp" TMPDIR="$task_root/.local/tmp" \
  go test -count=1 -run '^Test(Fighter4(Attacks|AirSpecial)|ChuAttacks)' \
  -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

[独立验证报告](testdata/fighter4-attacks-validation.json) 汇总当前来源绑定。[清理记录](testdata/fighter4-attacks-cleanup.json) 保留逐文件校验和；共享缓存、规范证据和其他工作者文件未清理。
