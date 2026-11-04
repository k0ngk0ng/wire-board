# 幽助空中释放：实现前验证范围

本阶段恢复来源 3 动作 `22` 的 `1AFC8→15AFC`，含完成恢复 `1863A`；不推进共享尾段或普通空中物理。复用现有 `1313E` 主弹、`132DE` 蓄力附件和 `134C2` 释放光效，补充它们在真实来源 3 初始化／自然输入和父子连续调用中的证据，不复制现有子对象规则。原菜单已确认幽助，来源号与平台席位分开维护。

实现前必须验证的失败场景：

- 原来源 3 的动画／形状／参数指针必须从真实菜单冷启动取得，不能只改其他人物的 tag。普通与特殊真实输入各自绑定双内核 RAM／画面基线。
- `Completed` 恰为 1 时先执行完成恢复，优先于帧 2／ticks 0 的建立门槛；其他非零完成 byte 不等同于完成。
- 主槽忙不建立任何对象。主槽空闲时先建立主弹，再检查 `+300` 释放槽；释放槽忙必须保留已建立主弹，父 Power 保留。
- 主弹只写 Pose 为 parent.Power 的 signed word 右移 5 后低 byte，必须保留主弹原 Power。原动作不查姿态表，不能额外限制父 Power 到 0..255。
- 完整／高 word 写入分开：Ground、父横／纵速高 word 偏移、坐标小数、速度完整 long、属性进位／掩码、Layer 与 TargetBit byte 回绕均保留原宽度。
- 释放光效只写原初始化字段，不能额外写 Facing、速度或旧时钟；成功只清父 Power，不套用 `17` 的跳帧 5／清 ticks。
- `1863A` 的 ExtraJump byte 递增回绕先于动作重置；重力复制完整 long，纵速分别算术右移 1／2 后相加，负奇数和边界值不能替换成先乘三再除四。
- 横速符号只在 Facing 等于 1 时保留参数值，其他 byte 均取相反数；最后进入动作 `02`、姿态 `07`、EnabledStages `14`，保留其他非拥有字段。
- 本接口不分配 FX、不读动画表，不因无关池游标或没有被读取的数据拒绝正常动作。仅不支持的动作种类需要原子拒绝。
- 现有主弹、蓄力附件和光效须使用真实来源 3 父子上下文，核对暂停、旧计时字段、接触／边界删除、父帧条件、有限序列和部分 clear；不能把来源 0 证据改标签后冒充。
- 原 CPU 每次比较完整父／三个子槽、FX 池与游标、shape 缓冲、声音和 RNG，私有调用另查全 RAM 改写范围与返回 SP。连续流只初始化一次，使用真实只读动画时钟／原 callbacks，不逐步回灌期望人物、子槽或 RNG。

先保留原 CPU 自然／隔离／连续材料，再写测试并取得本阶段缺实现失败，最后实现 `StepFighter3AirMotion(ChuSpecialMotionState, JumpParameters)`。完整人物、世界调度、全新对局和平台 E2E 由后续组合完成。本阶段不改现有 Go、asset bundle、PROGRESS、其他报告或 8799 服务资产。

## 已验证结果

| 证据 | 数量 | 实际范围 |
| --- | ---: | --- |
| 真实菜单自然流 | 175 次 | `22` 19、主弹 77、蓄力附件 48、释放光效 31 |
| 原 CPU 隔离调用 | 4,908 次 | `22` 4,764；现有 `17` 准备 36，以及三种子回调各 36 |
| 单初态父子流 | 24 条、6,272 个阶段 | 16 条空中释放和 8 条地面蓄力；全部父动作恢复，已建立子对象均到清除 |
| 独立规则测试 | 3 项顶层测试 | native `-race` 22.714 秒，Node/Wasm 5.743 秒，全部通过；完整 engine `go vet` 通过 |

连续流的 6,272 个阶段包含 952 次只读时序记录刷新，其余 5,320 次为原 CPU 时钟／动作／子回调。记录刷新只按双方各自的当前 pose／index 读取相同不可变数据，写 Duration／Flags；不当作 CPU 执行或完整共享尾段。每条流只初始化一次，后续先运行父时钟／动作，再按 `+100→+200→+300` 调度已建立子槽。同轮新建的后续地址会立即运行。父体退出所测动作后保持恢复状态，剩余子对象继续到实际删除；碰撞和活跃 FX 回调不在此流内。

两条观察内核冷启动分别逐帧匹配 selector 任务保存的[普通 3,600 步原内核基线](testdata/fighter3-attacks-reference-manifest.json)和[特殊 3,602 步基线](testdata/fighter3-attacks-special-reference-manifest.json)。普通输入未触发本阶段回调；175 次自然记录全部来自[真实特殊输入](testdata/fighter3-special-inputs.json)。特殊流只接受到第 3,600 个视频步的入口，额外两步排空挂起调用。没有重复采集未观察基线，也没有为自然采样改写人物或得分。

所有调用均比较完整 1,024-byte 父／三子槽、2,048-byte FX 池及游标、1,024-byte shape 缓冲及四个游标、声音请求／队列、参数与 RNG。私有调用另检查完整 64 KiB RAM 和返回 SP，无范围外改写。已有子规则直接复用 `StepChuProjectile`、`StepChuAttachment`、`StepFighter0Projectile` 及它们的 canonical 合并路径；没有复制子对象实现。捕获的 26 条动画记录均与已有语义资料逐项相等，没有修改素材包。

`22` 的 Power、偏移和 busy 顺序均保持原 byte／word／long 行为。主弹自身旧 Power 保留，姿态只接收父 Power 右移后的低 byte；父 Power 在成功建立 `+300` 后才清除，父动画帧／ticks 保持。`1863A` 的负奇数纵速、整型边界、Facing 非规范 byte 和 ExtraJump 回绕均由明确私有参数的原 CPU 矩阵验证。本 API 不读取表项或分配 FX，未触发的帧号和无关池游标不产生额外诊断。

## 自然 RAM IRQ 归因

特殊流第 2,565 步的一次 `22` 回调跨越两个 RAM IRQ 阶段。CPU hook 报告的入口为 `FFFFF000`，24 位分析地址为 `FFF000`；观察器登记两个地址表示，未改写 CPU 或 RAM。两次返回分别为 `FFF09A` 和 `FFF0A8`。第一个 IRQ 前后有 86 个 RAM byte 变化，第二个有 1 个；其中 26 个属于所测规则输出区外的最终变化。

原始变化保存在 [oracle 夹具](testdata/fighter3-air-motion-oracle.json.gz)对应 `air/frame=2565` 行。测试对每个范围外 byte 按实际 IRQ 顺序串联 `before→after`，例如 `F517` 为 `0→2→4`，不是单次 `0→4`。全部 26 个变化都有完整链，未解释变化为 0；没有把 IRQ 字节清零或作为 Go 规则输出回灌。人物／子槽／FX／shape／RNG／声音的比较仍逐 byte 完整执行。指令语义复用已保存的[直接 Ghidra RAM IRQ 清单](../../../docs/research/md-yuyuhakusho/static/world-frame-irq.txt)；REA 不支持 raw 68000，不称为 REA 反编译。

## 先行测试与复跑

[真实缺实现红灯](testdata/fighter3-air-motion-red.jsonl)明确报告缺少 `StepFighter3AirMotion`，[先行记录](testdata/fighter3-air-motion-test-first.json)在生产文件不存在时保存测试与已有原 CPU 材料摘要。最终被动 IRQ 注释采样补充了范围外写入的来源，父／子／FX／缓冲的规则结果保持不变。当前代码、夹具、基线、日志与清理绑定见[验证报告](testdata/fighter3-air-motion-validation.json)。本阶段通过封闭规则回调及父子组合，不作为完整世界或浏览器 E2E 交付。

从 worktree 根目录执行，所有缓存和输出留在 worktree：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter3_air_motion.py \
  --out .local/md-yuyuhakusho/fighter3-air-motion-replay
mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json ./games/yuyuhakusho/engine -run '^TestFighter3AirMotion'

GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
GOOS=js GOARCH=wasm \
go test -count=1 -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
  ./games/yuyuhakusho/engine -run '^TestFighter3AirMotion'
```

采集复用固定 ROM、被动内核及两份已绑定原版基线；Go／Wasm 只读取规范夹具和已有只读语义数据。本阶段没有提交、发布、上传或更改本地展示服务。
