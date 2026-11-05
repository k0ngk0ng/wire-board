# 来源 6「阵」攻击选择与立即初始化

本轮恢复 `1738c / 174ce / 17654 / 176be` 四个选择器、七个特殊根及两个普通 `13` 构造。普通／真人特殊自然流、原 CPU 完整返回、边界矩阵和单初态流均通过 native race 与 Wasm 对照。范围限于选择及立即初始化，不据此认定持续动作、子对象演进、碰撞或完整对局已完成。

## 来源与状态合同

原版 ROM SHA-256 为 `c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06`。未观察原 core 的 SHA-256 为 `1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da`，观察 core 为 `e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306`；两文件保持分开。

静态依据为既有直接 [Ghidra 选择器清单](../../../docs/research/md-yuyuhakusho/static/fighter-action-selectors-focused.txt) 和 [构造器清单](../../../docs/research/md-yuyuhakusho/static/fighter-action-constructors-focused.txt)。REA 不支持本目标的 raw 68000 路由，本轮复用清单，不称为 REA 反编译结果。

独立 API 为 `SelectFighter6Attack(Fighter6AttackState, ChuAttackMode)`。`Fighter6AttackState` 在 `ChuAttackState` 上增加 `AlternateVelocity int32`，只视图化调用者既有 Aux48 long。`19498 CLR.W +48` 清 big-endian 高 word，保留 low16；没有新增 canonical 存储。Energy fraction、共享 RNG 和未访问的存储由调用者持有。

地面前快移进入 `0e/0b`，后快移进入 `11/0c`，VX 都为 `40000`。横向 A 先提交投技，再进入 `13/83`；下向 A 进入 `13/86`，两个普通构造都完整写 VX=`40000`，经 `19c00` 更新 Facing。选择器闭包不产生 `0f`，其他模块接受 `0f` 的共享私有范围不能当作本选择器自然输出。

## 空中普通攻击的副作用顺序

1. `175c0` 先检查方向与 Pressed A，方向非 0／1 时提交额外投技。B+A 同时输入仍提交。
2. Held C + Pressed B 强写 Request 8。
3. Request 8 花费 8 点 Energy，成功进入 `11/0c`；否则继续。
4. Pose≠11 且 Request 7 花费 8 点 Energy，成功进入 `0e/ae`；否则继续。
5. 按方向表选择普通攻击，B 优先于 A。

空中快移不创建地面 FX，也不清 Request 或未写的 EnabledStages。成本不足是合法 fallthrough，不能撤销先前投技或 Request 8。存储错误仍按 API 合同整体原子失败。

[原 CPU 顺序证据](testdata/fighter6-attacks-order-evidence.json) 记录 Energy=0／7 两个代表调用：throw 游标分别从 224→256、256→288，Request 从 0→8，Energy 不变，最后进入普通 B。两次都完整返回调用者，经过 `175e0` 后到达 `188ee`。

## 七个特殊根

| 根 | 条件 | 成本 | Action／Pose | 额外写入 |
| --- | --- | --- | --- | --- |
| `19474` | Request 3；空中另有高度门槛 | 8 | `33/ac` | VY=`-60000`；Aux48 高 word 清零 |
| `194aa` | 仅空中 Request 4 | 8 | `34/ad` | VY=`60000` |
| `19518` | Request 1，Changed B | 16 | `35/a0` | 无 |
| `19542` | Request 1，Changed A | 24 | `36/a2` | 无 |
| `1959a` | Request 2，Changed B | 24 | `37/a4` | 无 |
| `195b2` | Request 2，Changed A | 32 | `37/a5` | 无 |
| `1956c` | 仅地面 Request 6，Health≤111，Changed B+A 同时为 `0x50` | 200 | `38/a8` | VX 清零 |

Request 1、2 均以 B 优先于 A。成功特殊构造清 Request、EnabledStages 并执行 `19c1c` 重置，保留 Facing；没有 ChildSize 门槛。空中 Request 3 的比较为 signed word：先将 Ground 高 word 减 `0x80` 并回绕，再与 Y 高 word 比较，不能替换成 int32 距离判断。

命令准备的 slot 3／5 同用 `1eab7 [2,0,2,ff]`、window=8。后执行的 slot 5 以 Request 6 覆盖 Request 4。冻结测试复用既有原 CPU 来源 6 命令流 3 组／342 步，并验证清零初态 DD 最终为 Request 6；Air 不把该请求改成 Request 4。`34` 私有入口覆盖不证明真人 DD 自然触发；真实 AI 的 Request 4 自然证据由主流程另取，不改命令表或自然流。

## 先测后实现与完整数据

[失败场景清单](FIGHTER6_ATTACK_FAILURES.md) 先于生产文件建立。CPU 夹具生成后冻结 [fighter6_attacks_test.go](fighter6_attacks_test.go)，SHA-256 为 `2e0c058e423a137da0ee61ad0dc0241769bcf50eca3030b86e1df0a91c45cca2`。[真实 red](testdata/fighter6-attacks-red.jsonl) 记录缺少本模块状态和函数的构建失败；[先测记录](testdata/fighter6-attacks-test-first.json) 确认当时生产文件不存在，并绑定 13 项先行证据。

新生产文件 [fighter6_attacks.go](fighter6_attacks.go) 的 SHA-256 为 `f4c4b0e5cfcdb6b3693347baf18b20d922513a2beed45c6fd9047d1d0a37d14d`。首轮实现即通过完整普通范围对照，后续未改生产或冻结测试。先测记录中的 special pending 为创建时的历史状态。

| 数据集 | 完整调用 | 范围 |
| --- | --- | --- |
| [普通自然](testdata/fighter6-attacks-natural-manifest.json) | 1,723 | ground 789、ground-special 808、air 63、air-special 63 |
| [真人特殊自然](testdata/fighter6-attacks-special-natural-manifest.json) | 1,231 | ground 574、ground-special 591、air 33、air-special 33 |
| [私有代表](testdata/fighter6-attacks-oracle-manifest.json) | 37 | 四种模式、四个地址、七特殊根、两普通 `13` 构造与有序副作用 |
| [私有边界](testdata/fighter6-attacks-boundaries-manifest.json) | 23,192 | 全方向／按钮 byte、能量、Health、ChildSize、Aux48、signed 高度边界、投技／FX 游标 |
| [单初态连续流](testdata/fighter6-attacks-sequences-manifest.json) | 64 组／1,024 步 | 空中快移耗尽 Energy 后继续普通攻击、连续投技及各特殊构造 |

所有返回核对 `exit_sp=entry_sp+4`，比较完整 actor、三个 child、其他 world 对象、全 FX 池、throw 缓冲、两个游标、位置小数、Energy fraction 和共享 RNG，不屏蔽差异。连续流只改变声明输入，不注入每步中间状态或 RNG。

私有夹具使用合成调用存储，不能证明自然游玩或角色资源初始化。Manifest 中 `frames_match_original_core` 指私有变更之前的被动自然 warmup 与原 core 一致，不把私有调用纳入自然帧证明。

## 基线与复现

合法普通输入为 [source6-next-inputs.json](../../../docs/research/md-yuyuhakusho/source6-next-inputs.json)，SHA-256 为 `61b140b4ac6820bb6d352bfc98202129fedc4b310367a055df50e1aaffad319e`。共享 [原 core3602 基线](testdata/fighter6-attacks-reference-manifest.json) 的 frames SHA-256 为 `2ed9bd9445a1d561eed662a1a76f739f918f93852a6a20c6abec9a5619d8799a`。

真人特殊输入为 [fighter6-special-inputs.json](testdata/fighter6-special-inputs.json)，SHA-256 为 `686cbd1c0015aa9b0d73d9b92db8dd4b34d1bfef75327a2261f3d82f2fb6f114`。共享 [特殊原 core3602 基线](testdata/fighter6-attacks-special-reference-manifest.json) 的 frames SHA-256 为 `d15e6c9a73e3a2344b4c2a19248dd67ce79d08fa14a7b03a464aa92d6a2ce64f`。

| 帧 | 自然构造根 | Energy 前→后 | Action／Pose |
| --- | --- | --- | --- |
| 1811 | `1743c → 194dc` | 223→223 | `13/83` |
| 1919 | `19518` | 223→207 | `35/a0` |
| 1999 | `19542` | 207→183 | `36/a2` |
| 2077 | `1959a` | 183→159 | `37/a4` |
| 2159 | `19474` | 165→157 | `33/ac` |
| 3388 | `1956c` | 205→5 | `38/a8` |

真人流没有 `34` 自然成功；七个特殊根由私有完整调用补足覆盖。该 3602 帧窗口内 `38` 的持续动作尚未退出，本模块只验证其构造完整返回，不把构造完成写成整个动作退出。主流程的更长世界窗口及 [独立 AI 菜单证据](../../../docs/research/md-yuyuhakusho/SOURCE6_CPU_MENU.md) 分别维护，不混入真人组件数据。

基线由 [capture_fighter6_reference.py](../../../scripts/yuyuhakusho/capture_fighter6_reference.py) 唯一生成，显式断言原 core SHA。观察由 [capture_fighter6_attacks.py](../../../scripts/yuyuhakusho/capture_fighter6_attacks.py) 完成，入口帧上限 3600，后 2 帧只排空已有调用；逐帧核对 RAM／video。私有恢复显式使用 `restore_refresh_cycles=True`。已有相同输入及基线时复用原产物，不再创建另一份 control。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter6_attacks.py \
  --out .local/md-yuyuhakusho/fighter6-attacks-recheck
```

最终正常包级检查包括来源 6 四个顶层测试与既有酎三个顶层测试：[native race](testdata/fighter6-attacks-native.jsonl) **100.933 秒**、[Wasm](testdata/fighter6-attacks-wasm.jsonl) **24.841 秒**，全部通过。先行普通范围的明确文件清单检查为 native race **86.349 秒**／Wasm **19.773 秒**；其 argv 与工作区内缓存位置保存在 `*-command.json`。一次加入未齐备 peer motion 生产文件的 Wasm 构建失败单独保留，未通过修改其他工作者代码解决。

```sh
task_root="$PWD"
mkdir -p "$task_root/.local/go-tmp" "$task_root/.local/tmp"
GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
  GOTMPDIR="$task_root/.local/go-tmp" TMPDIR="$task_root/.local/tmp" \
  go test -race -count=1 -run '^Test(Fighter6Attacks|ChuAttacks)' \
  -json ./games/yuyuhakusho/engine

GOOS=js GOARCH=wasm \
  GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
  GOTMPDIR="$task_root/.local/go-tmp" TMPDIR="$task_root/.local/tmp" \
  go test -count=1 -run '^Test(Fighter6Attacks|ChuAttacks)' \
  -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
  ./games/yuyuhakusho/engine
```

汇总与当前来源绑定见 [独立验证报告](testdata/fighter6-attacks-validation.json)；自有临时采样及菜单试验的清理见 [清理记录](testdata/fighter6-attacks-cleanup.json)。规范夹具、原 core 基线、真实 red、当前通过结果和菜单原版证据保留，未清理共享缓存或其他工作者文件。
