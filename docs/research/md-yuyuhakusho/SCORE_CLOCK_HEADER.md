# Header 的三个固定调用边界

本范围恢复 control-idle、tempo、fade-idle 三个局部调用的 master-cycle 成本。每个调用都包含原 caller、对应 callee 和一次返回；控制与音乐字段仍由外部唯一 owner 持有。170 个 private profile 的 330 个 routine 停点在两份固定 native core 上一致，Go 原生竞态、Wasm 与 vet 均已通过。

## API 与 owner 合同

```go
next, err := scoreclock.CallControlIdle(clock, scoreclock.ControlWork{
    Mode: mode, AuxMode: auxMode,
}, scoreclock.Limits{})
next, err = scoreclock.CallTempo(next, scoreclock.TempoWork{
    Enabled: enabled, Reload: reload, ActiveMusicMask: activeMusicMask,
}, scoreclock.Limits{})
next, err = scoreclock.CallFadeIdle(next, fadeSteps, scoreclock.Limits{})
```

`ControlWork` 只接受 `Mode=0`、`AuxMode=0`。非零值返回 `ErrHeaderUnsupported`；非零 mode 的业务语义和渐变没有在本层恢复。`CallFadeIdle` 只接受 `fadeSteps=0`，其余值同样明确返回 unsupported。

`TempoWork` 是 score owner 在当次操作中导出的临时语义：`Enabled` 表示本次 tempo period 非零；`Reload` 表示递减后实际触发重装；`ActiveMusicMask` 的 bit 0 至 bit 9 按音乐 slot 的原次序排列。只有 reload 才携带 mask；disabled 不得同时声明 reload 或 mask，mask 上限为 `0x3ff`。本层不读取或修改 period、remaining、NoteWait，不保存第二份 tempo／voice 状态。

音乐 observer 可在 tempo 的一次 `OpEnter` 提供当次 work，并一次调用 `CallTempo`；随后 counter／scan／leave 操作用于检查语义，不能再次收取同一成本。control 与 fade 由 Root 唯一 owner 提供即时 work；本层不因普通音乐路径而假设它们 idle。

| 调用 | 无 refresh 时的 master cycles | 固定操作数 |
| --- | ---: | ---: |
| control-idle | 938 | 10 |
| tempo disabled | 392 | 4 |
| tempo enabled、未 reload | 560 | 6 |
| tempo reload、全 inactive | 3,346 | 50 |
| tempo reload、全 active | 4,326 | 60 |
| fade-idle | 420 | 5 |

表中每种路径均包含一次原 caller 和一次返回。reload 的中间 mask 按原 slot 次序处理，每个 active slot 比 skip 路径增加固定成本；不能以 popcount 代替逐 slot 的 refresh 边界。

## 原成本与边界

原 caller 的 ROM opcode 为 `4eba`，属于 PC 相对 JSR，成本为 18 CPU cycles。初始概括中的 absolute-long JSR／20 CPU cycles 与原字节不符，已按 ROM 和固定架构 dispatch 表纠正。三个独立停点分别对应 caller 后的下一阶段：control 后、tempo 后与 fade 后。

tempo reload 固定扫描 10 个 slot，保留每个 active 测试、条件分支、必要的 NoteWait 增加、ADDA 与 DBF 边界。DBF dispatch 基数为 12 CPU cycles，原 handler 对回边减 2、末轮加 2，实际分别为 10 和 14。本层只用具名固定语义分支，不接受任意 CPU cycles、opcode、ROM、原 PC 或调用者指令 program。

所有操作复用冻结 `service.cost` 的 refresh 规则：到达当前 deadline 时，先将下个 deadline 设为当前 cycle 加 896，再增加 14 master cycles，然后支付当前固定操作成本。三次连续调用采用前一调用返回的 `Clock`，不会重置 refresh deadline。所有字段访问均属 main RAM 路径，不添加 YM／ZRAM 的 7 master cycles 延迟。

默认最多 4,096 次固定操作，上限为 65,536。无效临时 work、超过预算、起始 cycle 无效及后续 clock／refresh 溢出均返回错误和零值 clock；有限预算耗尽返回 `ErrBudget`。本服务只计算调用者传入的临时 clock，没有外部写入，也不回滚或复制 score owner。

## 先行测试与原生矩阵

[失败场景](SCORE_CLOCK_HEADER_FAILURES.md) 先于测试与实现。测试源码先保存在 `verification/score-clock-header/header_test.go.txt`，使用 Go overlay 作为新测试文件编译，真实触发缺少 `TempoWork`／调用 API 的 red；既有包在此期间不加入缺失 API 文件。测试源码与失败日志由 [test-first.json](../../../games/yuyuhakusho/verification/score-clock-header/test-first.json) 绑定。

原生矩阵包含 170 个 private profile、330 个 routine 停点。单独调用覆盖 control 5 例、fade 5 例、tempo 80 例；连续组合覆盖 80 例，并核对三个中间停点。reload 包括空、全 active、10 个单 active 位置和两组交替位，共 14 个 mask。

原生工具从 quiet world 检查点构造明确的 SR2700、origin、refresh、控制 idle 与 tempo／voice 临时状态。原 core 执行未修改的原 caller 和 callee；C recorder 仅读取真实 native PC／cycle／refresh 指针。recorder 开关对照检查完整 serialized state、RAM、refresh 和逐步边界，再比较两份固定 core 的结果。NoteWait 的 8 bit 环绕和 remaining 的原效果也由 native fixture 核验。

两份 core 共完成 680 次 profile 执行；全部 recorder 对照与跨 core 比较通过。Go 原生竞态与 Wasm 各通过 6 个顶层测试、170 个 native 子场景，并逐一核对 330 个原停点；非 idle 的全部单字节值、无效 tempo work、固定预算和溢出路径也通过。测试内容保持冻结，未在实现后修改。

[validation.json](../../../games/yuyuhakusho/verification/score-clock-header/validation.json) 绑定 native、Go 原生竞态、Wasm、vet、冻结测试、来源 SHA 与清理结果。portable fixture 的 SHA-256 为 `7a930091b4fc86e6e02af4246fbd67cdab9059078be9e377c0bc19853f535ae8`；包含原 PC／步骤与 native 效果的完整研究产物 SHA-256 为 `9149ce9f94f88abcfad0c79ebe404bd62b186a605e69a8201471dc1d6df2a8cc`。

## 复跑与范围

```sh
PYTHONDONTWRITEBYTECODE=1 nice -n 10 python3 scripts/yuyuhakusho/score-clock-header-verify.py \
  --mode native --out .local/md-yuyuhakusho/score-clock-header-go-review
PYTHONDONTWRITEBYTECODE=1 nice -n 10 python3 scripts/yuyuhakusho/score-clock-header-verify.py \
  --mode wasm --out .local/md-yuyuhakusho/score-clock-header-go-review
PYTHONDONTWRITEBYTECODE=1 nice -n 10 python3 scripts/yuyuhakusho/score-clock-header-verify.py \
  --mode vet --out .local/md-yuyuhakusho/score-clock-header-go-review
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/score-clock-header-cost-source.py \
  --out .local/md-yuyuhakusho/score-clock-header-cost-review.json
PYTHONDONTWRITEBYTECODE=1 nice -n 10 python3 scripts/yuyuhakusho/score-clock-header-capture.py \
  --out .local/md-yuyuhakusho/score-clock-header-native-review
```

Go runner 为所有 Go 调用设置工作区内的 telemetry、cache、module cache 和临时目录；Wasm 使用 `GOMAXPROCS=1`，不运行 `go telemetry`。native 重采保持 nice 单进程，并与其他原采／浏览器性能窗口协调。需要重建 portable fixture 时，为 capture 命令增加 `--fixture games/yuyuhakusho/audio/scoreclock/testdata/header-native-calls.json.gz`。

本范围不修改冻结的 YM、PSG、tick、score 或总索引。private SR2700 fixture 不是自然 driver／IRQ 调度证明；非 idle duck／fade、queue、dispatch、完整 tick、PCM、产品声音播放和实体硬件仍须另行验证。
