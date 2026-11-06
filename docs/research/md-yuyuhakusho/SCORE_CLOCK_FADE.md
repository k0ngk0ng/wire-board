# Fade-start 与 header prefix 的局部时钟

`scoreclock.ServeFadeStart` 和 `BeginFadeHeader` 已恢复两个固定局部的 master-cycle 成本与有序写点。91 个声明私有 profile、154 个定时 effect 在两份固定 native core 上一致；观察 core 的真实 RAM-write hook 逐项校验 effect 时间。Go 原生竞态、Wasm、vet 和与唯一纯 fade owner 的接缝检查均通过。

`LevelScan` 和 `FullClear` 是尚未完成的语义边界，不按零成本补齐后续 body。这里没有恢复整段动态 fade、完整 04 或整 tick 时基。

## API 与稳定 work 合同

```go
start, err := scoreclock.ServeFadeStart(clock, command, scoreclock.Limits{})
prefix, err := scoreclock.BeginFadeHeader(clock, scoreclock.FadeHeaderWork{
    Steps: rootFade.Steps, Remaining: rootFade.Remaining,
}, scoreclock.Limits{})
```

`ServeFadeStart` 只接受 01／05，从四指令 canonical leaf 开始，包括一次 RTS，不包括 dispatcher table、外层 JSR 或后续工作。`BeginFadeHeader` 从原 PC 相对 caller 开始，包括 18 CPU cycles 的 JSR，按原分支完成 idle／timer return，或停在 level scan／full clear 的入口。

`FadeHeaderWork` 是调用前唯一 Root FadeState 的即时两字段投影，在应用纯规则的 Writes 之前取得，不持久保存。本 profile 明确限定 SR2700、无额外 IRQ、无外部辅助 stall，两个 counter 在局部服务期间稳定。它不提供 live field 读取接口，也不能把这个 snapshot 合同扩展为任意异步场景。生产包不依赖 rootcontrol、score、chip 或 PCM owner。

结果 `FadeClockResult` 只有 `Clock`、`Next`、最多三个 `Effects` 和 `EffectCount`。每个 `FadeTimedEffect` 只有 `At`、具名 `Field` 和 `Value`。Field 枚举为 FadeRemainingStore、FadeStepsStore、FadeDACFlagsClear，不接受原地址、opcode、instruction／cycle program，也不保存另一份 clock 或 counter state。

| Next | 实际边界 | 后续责任 |
| --- | --- | --- |
| `FadeDone` | 当前 start leaf 或 header 已支付自己的返回 | 继续外层 driver；不表示 tick 已完成 |
| `FadeLevelScan` | 原 first-FM test 之前，包含选 FM 行与固定 scan setup | 后续 6FM＋3PSG 动态 body 另行处理；没有支付其返回 |
| `FadeFullClear` | 原 04 entry 之前，包含已发生的 terminal 分支 | 从 04 的第一项语义操作继续；没有执行或支付 04 |

计时服务不修改 Root 或 score。orchestrator 在 detached transaction 中按 At／原顺序各消费一次 effect，再采用返回 Clock／Next。纯 fade 规则的临时 Writes／Action 用于核对语义，不能重复应用同一写入。

start 的 DAC flag clear 已是第三个定时 effect，应在该时刻对接 score 的清 flag action，不能在 leaf RTS 之后再清一次。header 的 FadeMusicStep／RequestFullClear 则分别与 LevelScan／FullClear continuation 对应；counter effect 之后的固定 setup／branch 也已经包含在返回 Clock 中。

## 固定成本与写点

下表为没有 refresh 时相对传入 origin 的 master-cycle 偏移。实际调用在每个原固定操作边界检查 refresh，不能统一在末尾加总。

| 局部路径 | 结束偏移 | 定时 effects | 固定操作数 |
| --- | ---: | --- | ---: |
| start 01／05 | 448 | Remaining@0、Steps@112、DACFlagsClear@224 | 4 |
| header Steps=0 | 420 | 无，Remaining 原值保持 | 5 |
| header Remaining 非零 | 658 | Remaining−1@434 | 8 |
| header 非末步 | 840 | Steps−1@448、Remaining=3@644；Next=LevelScan | 11 |
| header 末步 | 630 | Steps=0@448；Next=FullClear | 8 |

start 四条指令均为 16 CPU cycles：两次 immediate byte 到 absolute word、一次 CLR.B absolute word、一次 RTS。初算将 CLR 视为 12 已被固定源表纠正，未进入冻结 golden。Remaining／Steps 的值分别为 01 的 3／40、05 的 32／50；相同值重复 store 仍产生真实 effect。

header 的 Remaining1→0 只做 timer return，不进入 level scan。terminal 只发 Steps=0，不发 Remaining reload。短分支未跳转为 8 CPU cycles，长分支未跳转为 12，跳转为 10；每项沿冻结 `service.cost` 的 refresh 边界执行。

到期时，下一 refresh deadline 设为当前 phase 加 896，再支付 14 master cycles，然后发生当前语义 effect 与固定成本。所有写入都是 main RAM 路径，不增加 YM／ZRAM 的 7 master cycles 延迟。输入 clock 不在包中持久保存。

默认最多 4,096 项固定操作，上限为 65,536。未知 start command 返回 `ErrFadeClockUnsupported`；无效 clock／limit、后续 refresh 溢出和预算耗尽均返回零值 result，不泄漏部分 effect。这里没有实际外部写入；预算耗尽使用现有 `ErrBudget`。

## 原生证明与先行测试

[先行失败场景](SCORE_CLOCK_FADE_FAILURES.md) 先于新实现与测试。先绑定 21 个原 opcode 的架构成本，再以原函数执行取得 private golden；测试通过 overlay 实际触发缺少 `FadeClockResult` 等 API 的 red 后冻结，最后才增加 Go 实现。测试和期望未在实现后修改，见 [test-first.json](../../../games/yuyuhakusho/verification/score-clock-fade/test-first.json)。

原生矩阵有 28 个 start profile 与 63 个 header profile，每个分支覆盖七个 origin／refresh 相位。包含原值已经等于 start 目标的重复 store、Steps=0／Remaining=255、timer 到零、byte 边界、非末步与末步。63 个 Done、21 个 LevelScan、7 个 FullClear 全部停在各自声明位置；后续 level body 和 04 指令没有执行。

两份固定 core 共执行 364 次 recorder 开关 profile。每份 core 的完整 serialized state、RAM、refresh、每步边界和结果不受 recorder 影响；两份 core 的固定步骤、结束 clock、Root 字段和 effect 也相同。

原 core 的 effect 值取自真实 store 后 RAM，写点按已绑定的 16-CPU store 架构成本从实际步骤结束时间取得；观察 core 再用真实 `HOOK_M68K_W` 逐项校验这 154 个 At／field／value，而非用 Go 时钟生成 native 期望。PC、原地址、步骤只在研究 provenance 中保留，portable fixture 没有原 PC／RAM program。

Go／Wasm 各通过 6 个顶层测试，包含 91 个 native 边界子场景和 91 个 pure fade owner 接缝子场景。后者核对语义 writes／action 与定时 effects／Next 一致，生产包没有因此导入或复制 rootcontrol。另验证 caller／返回合同、明确未完成边界、254 个未知命令、固定预算、初始／后续溢出与有界临时结构。

## 产物与复跑

[validation.json](../../../games/yuyuhakusho/verification/score-clock-fade/validation.json) 汇总来源、环境、red、Go／Wasm／vet 与清理范围。portable fixture SHA-256 为 `d09ef4a56b2df03012b34823e71e482ed99258f68a768a50b47e9066de262a51`；完整 native 研究产物 SHA-256 为 `d2f8f36f3ab59b5cf3a84c29487c24c41539ee31b5d81d140edb63b49fdaae4f`。

```sh
PYTHONDONTWRITEBYTECODE=1 nice -n 10 python3 scripts/yuyuhakusho/score-clock-fade-verify.py \
  --mode native --out .local/md-yuyuhakusho/score-clock-fade-go-review
PYTHONDONTWRITEBYTECODE=1 nice -n 10 python3 scripts/yuyuhakusho/score-clock-fade-verify.py \
  --mode wasm --out .local/md-yuyuhakusho/score-clock-fade-go-review
PYTHONDONTWRITEBYTECODE=1 nice -n 10 python3 scripts/yuyuhakusho/score-clock-fade-verify.py \
  --mode vet --out .local/md-yuyuhakusho/score-clock-fade-go-review
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/score-clock-fade-cost-source.py \
  --out .local/md-yuyuhakusho/score-clock-fade-cost-review.json
PYTHONDONTWRITEBYTECODE=1 nice -n 10 python3 scripts/yuyuhakusho/score-clock-fade-capture.py \
  --out .local/md-yuyuhakusho/score-clock-fade-native-review
```

Go runner 对全部 Go 调用设置工作区内的 telemetry、cache、module cache、临时目录；Wasm 使用 `GOMAXPROCS=1`，不运行 `go telemetry`。native 原采保持 nice 单进程，并与 Root 浏览器／FPS 保护及其他原采协调。需要重建 portable fixture 时，capture 显式增加 `--fixture games/yuyuhakusho/audio/scoreclock/testdata/fade-clock-native.json.gz`。

本范围不修改旧 leaf、纯 fade 规则、score、engine、原 canonical 流或总索引，没有提交。稳定 work 的局部证明不覆盖额外 IRQ／aux stall、完整动态 level scan、04、整 tick、world→IRQ、PCM 或产品声音播放。
