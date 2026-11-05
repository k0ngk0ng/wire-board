# Typed 音序 driver

`audio/score` 从 typed catalog 连续推进 gate、tie、软件 modulation、声像、包络和声部覆盖／恢复，返回有序 YM／PSG／DAC 与队列意图。运行时没有 ROM、原 PC、CPU 解释器、原 RAM 镜像或逐帧原时间表。正式接口与数据在 [API.md](../../../games/yuyuhakusho/audio/score/API.md) 和 [audio-score.json.gz](../../../games/yuyuhakusho/data/audio-score.json.gz)。

原生 `-race` 与 Node／Wasm 各 69 项测试记录全部通过；每端逐 tick 匹配 42 条原版流、126,141 个 driver tick 和 1,504,150 个有序声音意图，`go vet` 也通过。最终执行结果、源 SHA、fixture 身份及排除范围由 [audio-score-driver/validation.json](../../../games/yuyuhakusho/verification/audio-score-driver/validation.json) 记录。这里验证的是音序与请求层；master-cycle 赋时、FM／PSG 合成、总混音和平台接线由对应层继续验证。

## 数据与状态

正式数据使用 schema 2，覆盖 11 首音乐和 `a0..ab` 的 12 个效果，共 115 条 header 声部、31,875 个音符定义、102 个去重 FM patch，以及 6 个 DAC descriptor／音频 payload。包解码时核对 ID、引用、音频 SHA、参数范围、事件时间、循环及有限结束；解码上限为 16 MiB，超限和隐藏尾随 JSON 均明确拒绝。

各轨仅持有一份前缀／循环事件位置和剩余等待；原有限 loop／call／return 已在研究提炼时展开。音乐 tempo 在 driver tick 开始时只补偿活动音乐轨道的等待计数，gate、modulation、pan 和 envelope 仍按实际 driver 分支推进。效果不套用音乐 tempo。

transpose 仅作用于新音符；当前 base pitch 是发声音符的 owner。当前选中的 tone 与上次实际加载的 carrier algorithm 是不同阶段：隐藏期间可以选择新 tone，普通音量更新仍遵循原 cached algorithm 语义，恢复时再按当前 tone 加载。DSP 寄存器不在 score 中保存第二份镜像。

快照使用 typed 值结构；构造、恢复和推进失败不提交部分状态或意图。恢复核对 voice owner、通道、事件位置、envelope phase、tempo、modulation、noise latch 与 DAC 依赖。快照和恢复均无调用者可变 slice 别名。

## 原版特殊行为

| 行为 | 实现与证据 |
| --- | --- |
| `8f` DAC 结束 | 第 165 步退出整个当前 driver tick，后续声部不推进；不发布 PCM stop，也不写 YM key-off。当前样本由 PCM owner 自然结束。 |
| `95` 的 FM `F5=74` | 仅改变 instrument 选择，不加载音色。旧 note-only 候选统一命名为 volume-envelope；schema 2 改为 `fm-patch-select` 并补相应 patch 依赖。 |
| 隐藏期间换音色 | `a4@776` 覆盖音乐 FM5；音乐在第 781 步仍换 tone，但 cached algorithm 保持旧值；第 854 步按当前 tone 恢复并 forced-rest。 |
| 首次音乐前停止效果 | 原版会读未初始化 bank 0 的参数，并按未初始化通道 0 发送 26 条 bank 0 请求。使用源证据提炼的 typed fallback，未省略这个行为。 |
| PSG 恢复 | primary PSG3 结束后恢复当前音乐 noise latch。三处直接写入口独立复查，补齐原观察点遗漏的 5 条 `E7` 请求。 |
| DAC 声像重定向 | mailbox 非零时写 DAC pan；为零时写 YM bank 1／B6。两条自然运行均实际命中 pending `0x80`，没有通过私自写 score 状态伪造。 |
| PSG 扩展音高 | 保留实际到达的索引 121～127 对应的 7 个 divider；不拷贝未到达的相邻 ROM。 |

声像自动化只输出临时声像值，不改写手动配置的 pan owner。禁用后按原行为恢复手动值；已输出的芯片状态由 DSP 持有。

## 验证方法与证据修正

每条流从同一静音初态开始，只输入 Root 已接受的 score dispatch 和实际外部 PCM 反馈。Go 自行推进全部状态，逐 tick 比较 18 个声部的规范化字段，以及完整有序声音意图。没有逐 tick 注入原 score 状态。外部 ordinary PCM dispatch 独立保存，不冒充 score 输出。

原版与观察版的 RAM、视频、928-byte 驱动状态、PCM 摘要及样本数量先逐步全等，再使用观察结果。23 个既有隔离流复用原证据；混合补点只重放观察版，并逐步对照原控制，未重新录制原流。参数、混合 oracle、主 driver 和 DSP 的结论分别保存。

主 driver 首轮对照发现原证据把 pan period／elapsed 两个字段投影颠倒。静态原指令及原 row 分别证明 `row[35]` 为 period、`row[36]` 为 elapsed；新投影先取得真实 red，再逐 tick 核对，并断言除交换这两个字段外所有状态和请求不变。

后续又发现直接 PSG 恢复写未设置观察点。新增 3 个入口后，旧 60 个观察点事件逐字一致，仅新增 5 条真实恢复写；没有删除 Go 正确输出以迎合漏采。完整版本谱系、原假设失败及源绑定见 [AUDIO_SCORE_CLAIM_RESULTS.md](AUDIO_SCORE_CLAIM_RESULTS.md) 与 [AUDIO_SCORE_CLAIM_EVIDENCE.json](AUDIO_SCORE_CLAIM_EVIDENCE.json)。

先行失败清单位于 [score/FAILURES.md](../../../games/yuyuhakusho/audio/score/FAILURES.md)。主包缺实现、正式数据缺失、DAC pan feedback 缺字段，以及 21 项独立恢复／解码边界均保留真实 red。参数子包有独立原版预期和原生／Wasm 结果，见 [参数验证](AUDIO_SCORE_PARAM_GO_RESULTS.md)。

规范化排除项在原 oracle 索引中固定：PC／序列地址、已消除的有限控制栈和 multiplier、仅供旧 cleanup 诊断的 last duration、外部 queue／priority／selected sound、遍历 phase、score 运算不使用的 loaded feedback、源 padding／指针，以及禁用后必被新定义覆盖的 pan 参数。YM 只按逐寄存器已知硬件保留位做有效字段投影；全部请求数量、bank、寄存器及顺序仍参与比较。

## 复跑

在 `worktrees/yuyuhakusho` 根目录执行。portable 原版 fixture 位于 `audio/score/testdata/`，不需要本地 ROM 或模拟器即可重放 Go 验收：

```sh
python3 -B scripts/yuyuhakusho/verify_score_driver.py \
  --mode native --out .local/md-yuyuhakusho/score-review/native

python3 -B scripts/yuyuhakusho/verify_score_driver.py \
  --mode wasm --out .local/md-yuyuhakusho/score-review/wasm

python3 -B scripts/yuyuhakusho/verify_score_driver.py \
  --mode vet --out .local/md-yuyuhakusho/score-review/vet
```

脚本先核对全部 fixture SHA，保存命令、工作区内 Go cache／telemetry／tmp 配置、冻结源 SHA 和完整输出，并拒绝运行期间发生的源变更。原生启用 `-race`；Wasm 使用当前 Go 工具链的 Node executor。

若需重新提炼正式 catalog，先保留原证据与 fallback 补证，再执行：

```sh
python3 -B scripts/yuyuhakusho/export_score_driver_data.py \
  --fallback .local/md-yuyuhakusho/audio-score-claim/inactive-capture-fixed/fallback-patches.json \
  --out .local/md-yuyuhakusho/score-review/catalog
```

原地址和原字节只进入研究 provenance。旧 schema 1 候选及其 note-only 证明保持原样，不覆盖为新层完成状态。两次提炼的 gzip catalog 和 provenance 必须逐 byte 一致，最终索引记录该结果。

本轮只清理了第二次逐 byte 相同的私有导出，共 5,321,315 bytes。portable fixture、原始捕获、失败、源快照及版本谱系均保留，见 [reproducibility-cleanup.json](../../../games/yuyuhakusho/verification/audio-score-driver/reproducibility-cleanup.json)。

## 后续边界

- Root 保持 queue、priority、pause 和 scheduler 的唯一 owner；`Input.SoundID` 已是接受后的 dispatch。本包返回 replace-first／clear-priority 意图，不再持有一份 queue。
- group2 仅保留本范围实际影响的 claim 位，未实现启动；`ac..ef` 等未恢复 dispatch 明确拒绝。
- 本数据没有实际到达 pitch envelope、E9 或 FM3 的连续路径。pitch 参数有独立静态／合成证明，E9／FM3 不新增无原运行支持的生产入口；跨 envelope 换定义且 tie 保留 phase 仍未由本范围触发。
- 意图只有 driver tick 内顺序，尚无 master-cycle。精确成本层需要临时语义操作轨迹及有限解析工作量；仅凭前后状态与芯片输出不能推导已提炼消除的控制路径。不得用原 PC 或逐帧成本表补齐。
- 本包不合成 PCM、不决定原生媒体缓冲，也没有接入平台声音入口。完整声音验收仍需独立 cadence、芯片、PCM 仲裁、总混音及平台生命周期对照。
