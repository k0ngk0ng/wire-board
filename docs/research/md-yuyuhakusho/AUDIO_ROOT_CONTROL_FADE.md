# Root fade 两个计数的纯决策

`audio/rootcontrol/fade.go` 只恢复 fade-start 与每次 fade 调用的两个 byte 计数。5 条 canonical-v2 原流的 785 个 frame、746 次决策在 Go 原生竞态与 Wasm 中持续重放通过，50 个序列化恢复点的后续状态也相同。它不执行 score mutation、04 bulk clear、pause 或端口写入。

## 唯一状态与调用方式

```go
// Root.State 未来只持有这一份 FadeState；不再散存相同两个字段。
type FadeState struct {
    Steps     uint8 `json:"steps"`
    Remaining uint8 `json:"remaining"`
}

start, err := rootcontrol.StartFade(current, command) // 只接受 01／05
next := rootcontrol.AdvanceFade(current)             // 一次实际 fade 调用
```

本轮没有创建完整 Root.State 或 Controller。最终 Root state 嵌入一份 `FadeState`；snapshot／restore 直接保存这两个值，不引入另一份 live counter owner。所有 byte counter 输入都有明确原语义，包含 Steps=0、Remaining 非零的情形。

`FadeResult` 是当前 detached transaction 的临时结果：终态 `State`、最多两笔 `Writes`、`WriteCount` 和 `Action`。每笔 `FadeWrite` 只有具名 counter 与 value。调用者按 Writes 顺序更新唯一 FadeState，随后触发 Action；返回的 State 可核对最终值，不持久保存结果／历史或另一份 Root 视图。

| 调用／分支 | 有序 writes | writes 后的 Action |
| --- | --- | --- |
| Start 01 | Remaining=3，Steps=40 | `StopDACFlags` |
| Start 05 | Remaining=32，Steps=50 | `StopDACFlags` |
| Steps=0 | 无；Remaining 原值保持 | `FadeNoAction` |
| Steps 非零，Remaining 非零 | 只写 Remaining−1 | `FadeNoAction` |
| Remaining 原为 0，Steps−1 后仍非零 | Steps−1，Remaining=3 | `FadeMusicStep` |
| Remaining 原为 0，Steps−1 后为 0 | 只写 Steps=0 | `RequestFullClear` |

Remaining 从 1 变 0 的同一次调用只减 remaining，不提前发 music-step 或 full-clear。terminal 不写 Remaining，不重装 3，也不修改其他 Root 字段。`RequestFullClear` 交给 orchestrator 在原 04 阶段执行有序 bulk clear；不能拿本结果在 tick 末统一归零 priority／selected／queue／pause／voices。

`StartFade` 的 current 值可被新命令替换；即使两个计数已等于目标值，仍输出原两笔写入。其余 254 个命令都返回 `ErrFadeUnsupported` 与零值结果；04、PCM 和音乐请求由各自阶段处理。本 API 没有可注入的 duck／mode／queue／voice profile 字段。

`StopDACFlags` 和 `FadeMusicStep` 是供 Root 对接 score 的具名 action，不由此包调用或复制 score。Stop action 只请求 score 清 DAC music flags，不能清 owner／参数或停止已有 PCM。caller 在 pause／gate 跳过 fade 时不调用 `AdvanceFade`，此包没有第二份 pause byte 或 tick 计数。

## 原数据期望与测试先行

已有 canonical-v2 的原／观察 core 在全部 785 个 frame 上完整相同。本轮只读这两份保存结果，按事件边界提炼：

- `3893a` 是一次 fade 调用前的真实 Root 五字段。
- `3894c` 的 counter-return、`389a6` 的 fade-step-return 给出真实后值。
- terminal 使用 `38a2c` 的 reset-request 入口，尚未执行 full clear，不能换成帧末全清结果。
- `38916/38928` 的 start-before 与同帧真实 root 给出 40／3、50／32；remaining 在 steps 之前的顺序另由原字节模板绑定。

提炼器没有递减 counter、重装 remaining 或运行 Python／Go fade 规则来生成 golden。Action 从实际 exit boundary 命名，写入值从 native after 值取得；priority、selected、choice 在这些局部边界也逐项保持相同。原 PC、event index 和源 RAM 只在研究 provenance 中保留，portable fixture 仅包含 typed counter／action 与测试用局部 Root 上下文。

原流覆盖如下：

| 原流 | frame | 关键范围 |
| --- | ---: | --- |
| music82-fade01 | 225 | 01、40 步、末次 full-clear 请求 |
| music82-fade05 | 300 | 05、初始 32、后续 3、50 步、末次请求 |
| ordinary73-clear04 | 80 | fade idle；队列 04 不由 StartFade 处理 |
| mixed82-a7-clear04 | 80 | primary priority 上下文，fade helper 不清其他 Root 字段 |
| mixed82-a7-pause | 100 | 41 个 pause-hold frame 不进入 fade；其余 59 次 idle fade |

合计 744 次 Advance 与 2 次 Start：654 次 no-action、2 次 StopDACFlags、88 次 FadeMusicStep、2 次 RequestFullClear。两条正常 fade 流分别在 frame 192、261 请求 full clear。

[失败清单](AUDIO_ROOT_CONTROL_FADE_FAILURES.md) 先于新代码。冻结外部测试通过 Go overlay 的空 package 声明实际触发缺失 `FadeState/StartFade/AdvanceFade` API 的 red；共享生产包没有临时加入未实现测试。实现后测试按原字节复制，没有修改 golden 或测试内容。记录见 [fade-test-first.json](../../../games/yuyuhakusho/verification/root-control-fade/fade-test-first.json)。

提炼前的原字节检查还拦住了一个手抄模板错误：BEQ.W 位移被写为 `00da`，ROM 实际为 `00d8`。修正后才产生并冻结 fixture；真实 source-template red 和摘要保留在本轮证据中。

除了原流，先行针对性验证还覆盖 Steps=0／Remaining=255 保留、remaining1→0、terminal 单笔写、byte counter 边界、重复 start 的实际写顺序、全部未知命令拒绝，以及持久 state 恰为两个 uint8、不含 queue／voices／history 的结构合同。

## 验收与复跑

验收索引为 [validation.json](../../../games/yuyuhakusho/verification/root-control-fade/validation.json)。fixture SHA-256 为 `a88e122d490fd41839c944b403aadb29f309e56f8a981d81d41e1b37548cca28`，冻结测试 SHA-256 为 `88b9483be3b4ade09102ab1e43626a534df302a1f3a482ccdce44aef2a9b9e22`。

```sh
PYTHONDONTWRITEBYTECODE=1 nice -n 10 python3 scripts/yuyuhakusho/root-control-fade-verify.py \
  --mode native --out .local/md-yuyuhakusho/root-control-fade-go-review
PYTHONDONTWRITEBYTECODE=1 nice -n 10 python3 scripts/yuyuhakusho/root-control-fade-verify.py \
  --mode wasm --out .local/md-yuyuhakusho/root-control-fade-go-review
PYTHONDONTWRITEBYTECODE=1 nice -n 10 python3 scripts/yuyuhakusho/root-control-fade-verify.py \
  --mode vet --out .local/md-yuyuhakusho/root-control-fade-go-review
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/root-control-fade-export.py \
  --out games/yuyuhakusho/audio/rootcontrol/testdata/fade-native-streams.json.gz \
  --provenance games/yuyuhakusho/verification/root-control-fade/extraction.json
```

Go runner 对所有 Go 调用设置本工作区的 telemetry、cache、module cache 和临时目录，Wasm 使用 `GOMAXPROCS=1`，不运行 `go telemetry`。E2E 直接读取 portable fixture，不需要 ROM 或 native core；最后一条提炼命令只读已有 canonical-v2／ROM，并应逐字节重建同一 fixture 和 provenance，不执行 CPU。

当前 scope 没有修改 score、engine、冻结 leaf、原 native 数据、Root 总状态、pause payload 或总索引，也没有提交。pause 样例仅包含 idle fade，不是 active-fade pause orchestration 证明；Action 之后的真实 score mutation、完整 04 顺序、chip IO、整 tick 赋时、world→IRQ 和产品声音仍由各 owner 分别验证。
