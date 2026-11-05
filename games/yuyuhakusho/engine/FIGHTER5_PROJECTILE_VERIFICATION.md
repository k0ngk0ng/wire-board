# Source5 子对象原版证据与验收

MD《幽游白书：魔强统一战》日本版 source5 仙水的三个子对象回调 `1414e`、`143e4`、`146ba` 已恢复。四项冻结测试在 native race 和 Node.js/Wasm 下全部通过。原 CPU 证据包含 191 次自然调用、6,363 个边界和 16 条单一初态连续流程，共 10,150 次调用。完整人物／世界组合验收由主流程独立完成。

## 行为与字段所有权

`1414e` 和 `143e4` 包含完整初始化、字节动画时钟、移动前的尾迹分配、full long 移动、反射、命中／边界清除、AN 提交和投影。反射通过 `+FE` 父对象引用清除父对象的 `HitStopTicks`（`+69`）和 `RetainTicks`（`+75`）。`143e4` 的 pose19 读取父对象 `Held` 低 byte（`+71`）调整 VY。向上输入优先，速度阈值包含原版允许的最后一步越界。

`146ba` 使用 `1479c` 的九个 type／delay 项，跟随自身逻辑 ID 减 `0x200` 的父对象。父坐标完整复制，VX／VY 的高 word 用作投影偏移。表终止先于跟随，父 pose 离开 `a3` 时清除；此回调不执行通用 AN，不额外写入生命值。

VX、VY、Gravity 分别复用 Body 的完整 long；Aux48 是唯一 `+48` full long。Owner 是现有物理 `+FE` word 的临时视图，canonical 字段仍为 `WorldObject.DragonBank`，在 source5 上解释为父对象 ID。`Visual` 借用 `Fighter8ProjectileState` 的同地址字段视图，不调用 `StepFighter8Projectile`。清除复用已验证的视觉前缀处理，并按真实偏移清除 source5 的额外字段。

每次实际 draw 都保存当时的完整 `WorldDraw`。反射直接 rts 时没有 draw；清除先应用 `ApplyObjectClearPrefix`，再合并窄视图，未清前缀之外的字段和 fraction 保留。子对象、父对象和特效池在错误时一并保持原值。

## 原 CPU 对照与自然证据

| 固定控制组 | 输入 SHA-256 | 帧数 | 冷 hook 重放 |
| --- | --- | ---: | --- |
| 普通 | `6dd2eea8f585b62ad886da67d1123c61e06bedb97da4fdd9fd768b8441969601` | 3,602 | 每帧 RAM／video 全等 |
| 特殊技 | `e6ca2bbb16821f13c2508661b3b1b4ebddcad141f40ed7ef99fac0fea2f54af9` | 3,602 | 每帧 RAM／video 全等 |

两组均使用 Root 唯一未带 observer 的原 core 控制组。普通输入没有触发特殊子对象，原始 `ordinary-oracle` 的 `pass:false / records:0` 保留。规范 `oracle` 合并两组自然观察，其中 linear 为 139 次、follower 为 38 次、steered 为 14 次；包含 179 次 draw、7 次清除和 5 次无 draw 的反射直接返回。

Root 的固定特殊技输入只自然观察到低生命值招式的最终 C 后续 `19444`；其余后续的自然覆盖不在本报告中声称完成。

私有上下文来自普通控制组第 2,400 帧的真实 `8400` 对象，实际 source tag 为 5。恢复 checkpoint 时使用 `restore_refresh_cycles=True`。私有几何条件和脏数据与自然记录分开标识，没有注入角色，也没有把私有上下文称为自然。

| 规范夹具 | 数量 | 完整 64 KiB 审计 |
| --- | ---: | --- |
| oracle | 191 次调用 | 未解释写入 0、RNG 调用 0 |
| boundaries | 6,363 次调用 | 未解释写入 0、RNG 调用 0 |
| sequences | 16 流／3,596 次调用 | 未解释写入 0、RNG 调用 0 |

边界分为 linear 2,763、steered 2,833、follower 639、linear_stop 64、steered_stop 64，包含 192 次父字段更新和 128 次无 draw／无 clear 的反射返回。连续流程包含 880 次真实时钟、880 次完整 AN record、各父构造器及三种子对象，并推进到所有子对象清除；没有逐步回填期望值。

采样器逐次保留全部 RAM delta，显式注册 IRQ 的 24 位及实际 32 位入口，并独立记录声音队列消费者 `37c56`／`37cac`。本范围内实际 IRQ 和外部声音消费者调用均为 0，没有统一掩盖未知地址。

## 静态证据与范围限制

`docs/research/md-yuyuhakusho/static/fighter5-projectile{,-follow,-slave}.txt` 是直接 Ghidra 12.1.4 的 bounded 反汇编；REA 不支持 big-endian raw68000。`fighter5-projectile-manifest.json` 绑定 ROM、工具、探针及各文本 SHA。静态证据没有被称为运行观察。

相邻停顿入口 `142b4`、`1458a` 已按原 CPU 私有上下文各验证 64 次，包含 hit-stop 递减、shake 无符号上限、奇偶偏移、AN 提交和投影。自然控制组未观察到这两个入口。完整 ROM 的直接 32 位 big-endian 常量搜索也没有找到这两个地址的指针；这一 bounded 阴性结果不证明不存在相对／间接调度，主世界不得据此虚构入口调度。

`testdata/fighter5-projectile-data.json` 导出全部 33 个实读 typed AN 帧及 `1479c` 小表，包含原地址和小段数据来源。最终运行时使用 typed 规则和不可变小数据，不携带 ROM、原 CPU 或整 RAM。

## 测试先行与可重复结果

先写 [失败场景](FIGHTER5_PROJECTILE_FAILURES.md)，再收集自然／边界／连续流程，随后冻结 `fighter5_projectile_test.go` 和所有规范夹具。测试 SHA-256 为 `276b79212a2c31a3187a7bbe0e1274898841516ede1a053770054d4308767750`。`testdata/fighter5-projectile-frozen-red.jsonl` 是实现不存在时真实缺失 `Fighter5ProjectileState`、`Fighter5ProjectileParent`、`StepFighter5Projectile` 和 `StepFighter5ProjectileHitStop` 的失败记录；最初缺符号记录另存 `fighter5-projectile-red.jsonl`。

[test-first 清单](testdata/fighter5-projectile-test-first.json) 绑定测试、失败场景、采样器、静态 manifest、typed 数据、三组 gzip 和对应 manifest 的 SHA。实现后逐一复核，冻结文件全部未变，没有修改断言或期望状态求绿。

| 检查 | 结果 | 产物 |
| --- | --- | --- |
| Go 1.26.2 native race | 四项通过，22.488 秒 | `testdata/fighter5-projectile-native.jsonl` |
| Node.js 24.15.0／Wasm | 四项通过，5.796 秒 | `testdata/fighter5-projectile-wasm.jsonl` |
| go vet | 通过 | `testdata/fighter5-projectile-vet.txt` |
| 最终采样器独立双控制组重采 | 三组 gzip、manifest 和 typed 数据逐字节相同 | `testdata/fighter5-projectile-reproducibility.json` |

在本 worktree 根目录设置缓存与临时目录后执行：

```sh
export GOCACHE="$PWD/.local/go-cache"
export GOMODCACHE="$PWD/.local/go-mod"
export GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp"
export TMPDIR="$GOTMPDIR"
go test -race -run '^TestFighter5Projectile' -count=1 -json ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm go test -exec="$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run '^TestFighter5Projectile' -count=1 -json ./games/yuyuhakusho/engine
go vet ./games/yuyuhakusho/engine
```

CPU 重采使用两个固定控制组，输出保存在本 worktree 的独立目录：

```sh
python3 -B scripts/yuyuhakusho/capture_fighter5_projectile.py \
  --out .local/md-yuyuhakusho/fighter5-projectile-replay \
  --baseline docs/research/md-yuyuhakusho/source5-next-inputs.json games/yuyuhakusho/engine/testdata/fighter5-attacks-reference-frames.json games/yuyuhakusho/engine/testdata/fighter5-attacks-reference-manifest.json \
  --baseline docs/research/md-yuyuhakusho/source5-special-discovery-inputs.json games/yuyuhakusho/engine/testdata/fighter5-attacks-special-reference-frames.json games/yuyuhakusho/engine/testdata/fighter5-attacks-special-reference-manifest.json
```

原 ROM、原 core、hook core 和 bridge 的精确 SHA 见规范 manifest；CPU 重采只用于研究对照。

已清理本 owner 的 Ghidra scratch、重采副本、采样临时目录、重复调试日志和一个自有 pyc，共 5,264,214 字节。规范证据、原始／冻结 red、native race／Wasm／vet 结果全部保留；共享 Go 缓存、共享临时目录、ROM、工具和其他 agent 文件未清理。逐文件清单见 [清理记录](testdata/fighter5-projectile-cleanup.json)。
