# 桑原子对象：实现前验证范围

本阶段恢复新回调 `12DD4`，包含初始化 `12DDA`、持续 `12E54`，止于绘制入口 `1580` 或完成 `3556` 前缀清除。状态使用 `Fighter2ProjectileState{Projectile ChuProjectileState; HitStopTicks uint8}`，canonical 仍由 `WorldObject.HitStopTicks` 持有，不新增相同字段。已恢复的 `12C96` 只补来源 2 的创建／姿态 `13` 和生命周期上下文，不复制其实现。

生产实现前需取得原 CPU 材料并验证这些失败场景：

- 真实菜单取得来源 2 父体、动画／参数指针和自然输入，不能只改其他角色标签。观察内核逐视频步对照同输入未观察基线。
- 初始化和生命值 255 写入先于暂停／慢动作；保留旧 FrameIndex、Flags、Completed、HitStopTicks 及所有未写字段。
- 初始化只夹取 X 高 word，保留小数；两个边界比较均读取原 X 高 word，反向边界不能被通用 clamp 改写语义。持续阶段不按速度移动、不读取父体。
- 暂停不推进时钟、不消耗停顿、不改震动或投影；已经完成的初始化写入仍保留。
- 非零停顿先递减 HitStopTicks，Shake 大于 1 时才递减；按新的停顿奇偶对 ScreenOffsetX 加／减新 Shake，word 回绕保留。该分支冻结时钟并跳过完成／尘土判断，仍提交形状、声音和投影。
- 停顿从 1 降至 0 的本次调用仍冻结；不能套用人物零停顿前置而引入移动或提前删除。
- 零停顿先清 Shake、推进原时钟，再检查 Completed 恰等于 1。姿态 9、ticks 恰为 0 时独立两次分配 `1FCE8` 尘土，保留单侧成功、满池游标和旧槽小数。
- 动画姿态 9／12／13 的全部只读记录逐条用原 CPU 验证，原动画持续流不得靠回灌中间态推进。来源 2 创建的散射弹使用姿态 13、原属性掩码与旧字段。
- 部分清除仍按 Size 的 16-byte 前缀，只有覆盖 `+69` 才清 HitStopTicks；缺少动画、非法游标／缓冲或晚期错误须原子回滚初始化、尘土和其他输出。
- 原始子对象、父体、三子槽、完整 FX／游标、shape 缓冲／游标、声音及 RNG 保留字节证据；私有调用另核对完整 RAM 改写范围，记录自然 IRQ／声音消费者，不把异步写入掩屏。

先保存自然／隔离／单初态材料和测试红灯，再实现 `StepFighter2Projectile`。父动作、完整世界、全新对局和浏览器／平台 E2E 另行组合验收；本阶段不改现有 Go、bundle、PROGRESS 或 8799 资产。

## 原 CPU 材料

[真实特殊输入](testdata/fighter2-projectile-inputs.json)保留原菜单的前 1,800 个视频步，随后使用 DD+B、下上+B、QCF+B，再补 A 版本。实际观察到三类父动作和两个子回调，没有改写 RAM 或强行设置自然动作。普通及特殊观察内核分别逐帧匹配共享的 3,600／3,602 步原内核基线；特殊流入口截止 3,600，额外两步只排空挂起调用。

| 证据 | 数量 | 范围 |
| --- | ---: | --- |
| 自然完整回调 | 124 次 | `12DD4` 73、`12C96` 51；全部来自特殊流 |
| 原 CPU 隔离调用 | 1,724 次 | 新回调 1,708、现有散射 16，包含逐条数据验证 |
| 单初态生命周期 | 24 条 | 原来源 2 创建器 16 次，子回调 896 次，全部到实际删除 |
| 公共只读记录 | 23 条 | 姿态 9／12／13 分别为 10／5／8 帧，左右朝向各一次原 CPU 完整调用 |

896 个连续子回调包含新对象 752、既有散射 144；加 16 次真实创建器，共 912 次原 CPU 调用。八条 pose9 流用于公共分支，其余 16 条从实际来源 2 的 `1AC58/1ADC8` 创建前状态开始，先核对父动作的完整建立结果，再沿同一状态调度子回调到清除。父体在创建后保持不变；没有把该流当作完整世界。显式停顿／震动与暂停输入均列在每步 `external_input`，未回灌中间状态或时钟。

每次比较完整 1,024-byte 父／三个子槽、2,048-byte FX 池、1,024-byte shape 缓冲及其游标、声音、RNG、参数和环境。私有调用另检查完整 64 KiB RAM 写入与返回 SP。自然／隔离／连续材料均没有独立队列消费者、IRQ 改写或未解释范围外写入，所有规则输出保持逐 byte 比较。新回调不读取父体；完整父体／其余子槽字节仍作为不变性检查。

[独立语义包](../data/fighter2-projectile-rules.json)只含 `animations` 数组的 23 条 Key／Data，大小 7,262 bytes，SHA-256 `a919578d7f0bbb3656e1802ca9f9e6327c6e1bd7d789c3367a63a7a6f1d6d891`。没有原 CPU 用例、RAM、指令、像素或声音载荷。[数据来源报告](../data/fighter2-projectile-rules-manifest.json)绑定固定 ROM、采集器及三组夹具。现有散射规则继续由 `StepFighter0Projectile` 执行。

静态指令来自[直接 Ghidra 的有限清单](../../../docs/research/md-yuyuhakusho/static/fighter2-projectile.txt)，不是自然执行覆盖率。REA 不支持 raw 68000，未声称使用 REA 反编译。

## 先行测试与复跑

原 CPU 材料和测试先于生产文件；[实际红灯](testdata/fighter2-projectile-red.jsonl)包含本阶段自己的 `Fighter2ProjectileState` 与 `StepFighter2Projectile` 缺符号，同时如实保留并行父动作暂未定义的编译诊断。[先行记录](testdata/fighter2-projectile-test-first.json)绑定当时测试与夹具。验证范围是完整回调、创建器到删除的连续流、运行数据与 CPU 记录一致，以及原子错误／暂停初始化；完整平台 E2E 仍由后续组合完成。

四项正常 package 测试全部通过：native `-race` 7.571 秒，Node/Wasm 1.853 秒，完整 engine `go vet` 通过。父动作冻结后另外复核创建器到生命周期测试，native 3.054 秒、Wasm 0.538 秒；该次定向检查只排除当时仍处于缺实现红灯的独立 selector 测试，不扩大所测规则范围。完整命令、源码／数据摘要及日志在[验证报告](testdata/fighter2-projectile-validation.json)。本阶段的生产实现未经过规则修正，原 CPU 结果首次对照即一致。

停顿分支保留 FrameIndex／Ticks，同时允许动画提交刷新 Duration／Flags。它在 HitStopTicks 从 1 降至 0 的本次调用仍继续冻结并投影，下一次零停顿调用才恢复普通时钟／完成判断。独立对象的停顿不会引入人物输入、移动或动作前置。

从 worktree 根目录执行：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter2_projectile.py \
  --out .local/md-yuyuhakusho/fighter2-projectile-replay
mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json ./games/yuyuhakusho/engine -run '^TestFighter2Projectile'

GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
GOOS=js GOARCH=wasm \
go test -count=1 -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
  ./games/yuyuhakusho/engine -run '^TestFighter2Projectile'
```

采集器复用 selector 任务保存的固定原版参考帧，不重复采控制组；输出目录、Java／Go 缓存和私有材料都留在 worktree 内。规范夹具、红灯、验证日志及独立语义数据作为复跑产物保留。
