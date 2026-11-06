# 有序 YM reset 与已完成 DAC 原子段

新增 `ResetYM` 和 `ResetYMAfterDAC`，支持原 reset 回调在 DAC 原子写完成后才发生、但仍携带较早请求时间的情况。原生七场景共 4,777 次操作、24 帧、17,658 个立体声采样通过；Native race／Wasm／vet 及全部旧芯片／PCM 播放器回归通过。生产只新增 `audio/chips/reset.go`，未修改旧 Queue、Interactive 状态、YM 算法、PSG、DSP coefficient 或 PCM 实现。

输入域与使用方式见 [RESET.md](../../../games/yuyuhakusho/audio/chips/RESET.md)，完整来源和结果见 [validation.json](../../../games/yuyuhakusho/verification/fmpsg-reset/validation.json)。本项仍给定外部事件时刻，不证明世界／IRQ 的独立声音调度或物理扬声器输出。

## 真实反例与错误假设

PCM owner 的冻结私有 CPU 输入在 `7183006` 申请 reset，Z80 的既有 DAC 写先在 `7183080` 完成，随后原 `gen_zreset_w` 调用 `fm_reset(7183006)`。芯片采样点 `7183008` 位于两者之间。release 在 `7185006` 再调用 reset，此后启动流程的普通写已经晚于完成 frontier。

新观察器直接调用未修改的原 `fm_reset` 和端口 API，保留原事件数组顺序。12 帧、8,832 个立体声 PCM 采样及 CPU 控制中已记录的全部 DSP 字段逐项一致。另将同一输入按 raw cycle 排序，两个 native core 都在 `frame=8, first_byte=56` 首差，两个索引均从 0 计数，复现原反例。没有修改任何 raw cycle，未使用数据拟合增益或移动边界。

此前 20 组排序后没有 PCM／state 差异的文件不是 red；owner 已在 `sort-hypothesis-correction.json` 保留旧文件字节、SHA、误标原因和命名订正。新验证只引用真正跨 chip tick 的反例，不借用那些无差异结果。

完整 native YM／PSG／当前 FIR 状态在端口重放中额外采集。旧 CPU recorder 只导出 data 和 reset，没有原 address-port 时刻；此重放在每笔 data 的同一 cycle 明确补足 address。补足的 address 是声明输入，不称为原访问时刻。CPU 控制本身未记录的字段不会被补称为直接 CPU 观察，也不能由当前 data-only Ledger 推断任意半帧 YM latch 接线全等；CPU 录音对照和新 native 端口完整状态证明分别保留。独立私有 warm-address 场景则直接驱动原 address／data／reset，验证 API 的 latch 保留语义。

## 原 reset 语义

原 `sound.c:YM2612_Reset` 只执行 `fm_update(cycles)`、`YM2612ResetChip()`、`fm_cycles_busy=0`。它不调用 `sound_reset` 或 `audio_reset`，不清 PSG、原 FM raw buffer、FM 进度、FIR 或 integrator。

原 warm chip reset 保留共享 address latch 和延后的 operator keyscale。新实现以固定 reset 参数构造新 chip 候选，仅从当前 chip 保留这两个独立阶段；参数、相位、包络、LFO、定时器、CSM 和 busy 按原值重置。完整活动 FM／SSG／LFO／CSM 原生场景逐项验证，不以 muted DAC 的三字段映射代替完整 reset。

同步仍接收原 request cycle。若芯片已经运行到更晚网格，则不再生成采样；原已生成输出和状态不会被回退。特殊输入只保持完成 frontier，没有保存第二个原事件时间或输入历史。

## 有界合同与私有矩阵

| 检查 | 已验证 |
| --- | --- |
| 实际 CPU 反例 | 回退74，跨 chip tick；12帧 PCM／已捕获 DSP 全相同 |
| 私有最大回退 | 明确194端点，另有104；195拒绝属于本 profile合同，不称原硬件非法 |
| 完整 warm reset | address／latchedksr保留，busy清零，活动FM／PSG／FIR按原语义继续 |
| 普通 reset | 同cycle、重复、release、保留地址后的跨data-port写 |
| 帧边界 | +194及+1008完成frontier，回退194后flush、carry与半帧恢复 |
| 失败原子性 | 假completed frontier、过大回退、普通回时、已flush帧、容量／overflow、旧Queue拒绝扩域 |

single DAC 的 `LD(nn),A` 在本固件为 13T×15=195 master cycles，请求在其中发生时完成点与请求的差严格小于195，因此使用194的整数上限。dual 的 `LD(BC),A` 为7T，对应104。该静态边界、实际回退74／104、声明私有194输入是不同类型证据，报告中分别保留。任意 YM／PSG 操作不能借用本域；API调用者必须提供来自 Device实际完成DAC事务的因果。

原先普通 input lookahead 仍为1,008，不由 reset 扩大。末端＋1,008的完整私有组合已经单独 native 验证：保留ahead PSG、FM已生成进度及输出，reset请求仍保留较早raw值，随后显式flush再继续。

## 测试先行与矩阵补证

[失败场景](FMPSG_RESET_FAILURES.md)、Go E2E、原生真实反例及最初七场景fixture均先于新生产代码。缺 `ResetYM`／`ResetYMAfterDAC`／`MaxResetDACBackstep` 的实际 API red、原始测试字节及旧 Interactive／YM 来源保留。首次实现即通过 native，随后旧17+62和PCM播放器也通过。

初始新矩阵的 reset 组合只到帧尾＋194。为直接补齐原有＋1,008 端点与 reset 的组合，后续在同一 E2E 场景追加了 10 个原生操作，没有修改生产、Go 测试或断言。原先全部 fixture、producer 和 race／Wasm 绿日志按原字节保存；`full-ahead-matrix-extension.json` 明确记录这一补证晚于实现，不能把修订后的全部 fixture 声称为生产前所得。补证后重新通过必要的全套检查。

旧 17 项顶层检查和 62 个子场景继续通过，新 reset 增加 2 项顶层检查及 7 个原生子场景；已冻结 PCM 播放器另有 6 项顶层检查、37 个子场景。合计 Native race／Wasm 各 25 项顶层检查、106 个子场景，没有排除任何旧失败。

## 复跑

```sh
TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry" \
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -json -count=1 ./games/yuyuhakusho/audio/chips/... \
  ./games/yuyuhakusho/audio/dsp ./games/yuyuhakusho/audio/pcmplayer
```

无需 ROM／core 即可重放已保存 Go fixture。重新采原生需固定原资源与 CPU 反例，输出到新的工作区目录，使用低优先级单进程；不得覆盖冻结证据：

```sh
nice -n 15 env PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/fmpsg-reset-capture.py \
  --out .local/md-yuyuhakusho/fmpsg-reset-review
```

Wasm最小环境、精确命令、旧来源未修改证明与当前SHA由验证索引绑定。未提交、上传或部署，未修改总索引、进度或其他分工。
