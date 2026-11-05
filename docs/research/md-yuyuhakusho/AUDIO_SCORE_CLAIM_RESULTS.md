# 混合声部与 canonical 音序 oracle

最终 oracle 为 42 个场景、126,141 帧、1,504,150 个有序 score intent。它复用原有 23 个隔离定义，并新增 19 条混合原版流共 3,331 步。原内核与观察内核逐步比较的完整 RAM、视频、928-byte driver、PCM 摘要和采样帧数全部一致。独立 Go driver 和 DSP 的通过状态由对应 owner 另行验收，本页不将源归一化结果表述为独立实现已经通过。

最终索引为 `.local/md-yuyuhakusho/audio-score-claim/canonical-v3-final/index.json`，SHA-256 为 `cac5d869aab3fd4f45eb476ae541919a39c10394922107ba897714f955d4f263`。所有旧版本、真实失败、来源及修正谱系均保留。完整源绑定见 [AUDIO_SCORE_CLAIM_EVIDENCE.json](AUDIO_SCORE_CLAIM_EVIDENCE.json)。

## 原版覆盖与状态归属

首轮 15 个场景包括 `82 + a0..ab` 的全部 12 条 primary 混合流，以及高优先级替换、低请求拒绝和新音乐清理 primary，共 1,951 步。补齐直接 PSG restore 观察点后，保存 19,532 次 YM 请求、5,036 次 PSG 写、198 次 accepted DAC descriptor、205 次 DAC guard feedback 与 43 次 routing 入口。各内核分别验证静音原点，恢复时显式补回 `refresh_cycles`。

primary 初始化先设置被遮蔽 music 的 claim，再清空并初始化 effect。primary 按当前 tick 执行；music 初始化跳过当前 tick。claim 期间 music 继续推进，只在具体硬件请求处抑制输出。任何一个 primary 声部停止都会清优先级，不等待该 effect 的全部声部结束。

`F2` track-end 清 active／tie 后 keyoff。FM 只在目标 music 仍 active 时清 claim、设置 forced-rest 并加载当前 patch；PSG 清 claim并设置 forced-rest，目标为 noise 时再恢复当前 noise latch。新音乐调用的 `StopPrimary` 不同：它会在 music 尚未初始化时也执行恢复，不能套用 `F2` 的 active 条件。

900 步补证在 `82` 第 776 步触发 `a4`。第 781 步，隐藏 music FM5 选择由 patch 3 改为 patch 2，loaded algorithm／feedback 仍为 `2/7`；第 854 步释放后，按当前 patch 2 恢复为 `4/0` 并设置 forced-rest。tone 选择与 loaded algorithm 具有不同的演进条件；feedback 归芯片状态，canonical score view 不重复持有。

源中另有 secondary FM4／PSG3 及其与 primary 的层叠路由。本范围没有启动 group2，不能由 primary 结果推导全部 group2 行为。

## 首次音乐启动前的 fallback

`a4@0 → music82@10` 的 160 步补证确认：`StopPrimary` 对尚未初始化的 music FM5 行读取 bank 0、patch 0。该行 channel 仍为 0，26 条请求实际写到 YM bank 0／channel 0；前 25 个完整请求值逐 byte 等于原 ROM 前 25 字节，最后写 `B4=0`。不能按固定 slot 名把它改写成 FM5 硬件通道，也不能省略这段原行为。

独立 typed fallback 为 `.local/md-yuyuhakusho/audio-score-claim/inactive-capture-fixed/fallback-patches.json`，ID 为 `patch-e4ea52de08ee0808a301`，文件 SHA-256 为 `79e7d6f85ab60a53d3af44ff4f98a307a4c73c2514c17357603c80c843820c06`。ROM 首字节为 `FF`，algorithm／feedback 因此为 `7/7`。地址、向量字节和完整请求留在 provenance，生产候选仅含拆分参数。

先行检查预设的 `0/0` 被实际原读否定。旧 checker／失败保留；接受版 `score_claim_inactive_source_check.py` 仅按固定原字节修正这项预期，保留全部 26 条请求、顺序和未初始化行断言。独立补采入口漏导入名称的实际失败与修复前源也保留。

## DAC 外部反馈

普通 PCM 命令 `7d` 由根 PCM owner 处理，不伪装成 score 定义。score DAC blocking 仅测试 pending 最高位；pan redirect 测试整个 byte 是否非零。两个条件不能合并，也不作为 score 持久状态。

两条自然顺序均命中 pan redirect：`7d@0 → music88@10 → E0@11`，以及 `music88@0 → 7d@1 → E0@1`。两次在原 `37b66` 均读到 pending `80`，随后实际执行 `37b70` 的 pan 命令，同帧没有 YM bank 1／`B6` 写。每条流 160 步，原版与观察版全部一致。

portable 的 `frame.pan_feedback` 为 `{pan_redirect:true}`，输出为有序 `dac-pan` 与 typed stereo。反馈在原 guard 处读取，不由最终请求反推。两条新增流证明重定向分支；已有隔离 `88` 的 YM bank 1／`B6` 请求保留非重定向路径证据。第一条流先行预设未重定向的失败保留，source-bound 接受版仅更正实测分支预期，所有 guard 与路由断言保留。

## 版本与投影修正

| 版本 | 覆盖 | 修正 |
| --- | --- | --- |
| `canonical/` | 39 场景、125,661 帧 | 初始投影；首次音乐启动前的 1 条补证另存。 |
| `canonical-v2/` | 40 场景、125,821 帧 | 只修正 pan period／elapsed，其余字段、请求和排除类别完全相同。 |
| `canonical-v3/` | 40 场景、1,499,361 intent | 只补 5 条真实直接 PSG restore 写。 |
| `canonical-v3-final/` | 42 场景、126,141 帧 | 追加两条 pan feedback，前 40 场景逐项不变，最终 1,504,150 intent。 |

pan 投影先行 source-bound red 为 `isolated-8a` 第 1 步 music FM1：v1 错写 `(period,elapsed)=(1,10)`，原指令证明应为 `(10,1)`。`37abe` 将 row `23` 复制到 row `24`，`37acc` 用 row `23` 比较，`37b18` 增加 row `24`，因此 period 为 row `23`、elapsed 为 row `24`。v2 对 138,164 个启用 pan 的 voice-frame 核对原状态，92,443 处发生有效交换；没有修改已独立验证的 Go helper 来迎合错误 oracle。

旧 60 点观察器漏了 `39186/3889a/3890c` 的直接 PSG restore 写。63 点观察器仅重跑 15 mixed observed，对旧 original control 的 RAM／视频／driver／PCM／样本数全部一致；去掉新增点后，所有旧 60 点事件逐字相同。新增均为 `39186` 的 `E7`：`a1@94`、`a7@80`、`ab@46`、高替低流 `@98`、低拒绝流 `@94`。初始精确列表只含 3 个已知例子的失败和完整 5 条的 source-bound 接受版分别保留。

23 个隔离定义没有重采。全 122,810 原状态与源分支独立证明：122,107 音乐帧中 primary／secondary inactive；703 效果帧中 music 从未初始化、music PSG3 channel 始终 0，不能满足 `3917e` noise-channel 条件。这个有界结论不以旧 60 点 trace 缺少事件来推断。

## 冻结 schema 与排除项

每个 `.jsonl.gz` 首行为 `{kind:"initial",id,commands,state}`，其后每行含 `step`、`dispatch_sound_id`、`clear_primary_priority`、`state`、密集排序 `intents`、`external_intents`、`dac_feedback`、`routing`、PCM 摘要／样本数；pan 补证保留 `pan_feedback`。dispatch ID 来自实际 header 入口，无 header 时为 0，是外部 dispatcher 输入而非逐 tick 注入状态。普通 PCM 命令单列 external intents。

18 个 voice slot 保留 active、note-off、claim、tie、硬件声部、transpose、base level／pitch、tone、pitch control、note wait、gate、modulation 定义与计数、detune、pan、loaded algorithm 和 noise latch。envelope cursor 使用 `{phase,index}`，phase 为 `prefix/cycle/ended/disabled`；实际源 byte cursor 全部成功映射，原 cursor 只在 private provenance。

全部版本固定排除：源 PC／序列地址、共享 loop／return 字、已解析的 multiplier、仅供原 cleanup 诊断的 last duration、外部 dispatcher 的 priority／queue／selected、遍历派生 phase、归 DSP 的 loaded feedback、无用 padding／源指针，以及禁用后会在再次启用前完整覆写的 pan 参数。没有为 Go 差异扩大排除范围。

## 复跑与保留

[失败场景](AUDIO_SCORE_CLAIM_FAILURES.md)和各接受契约先于对应新采集／投影取得真实 red。正式检查使用 `score_claim_inactive_source_check.py`、`score_claim_pan_projection_check.py`、`score_claim_restore_source_check.py`、`score_claim_restore_projection_check.py`、`score_claim_pan_feedback_source_check.py`、`score_claim_isolation_path_check.py` 和 `score_claim_final_check.py`。旧历史 checker 有意保留已被原读否定的假设，不作为正式接受入口。

最终检查命令：

```sh
python3 -B scripts/yuyuhakusho/score_claim_final_check.py \
  --index .local/md-yuyuhakusho/audio-score-claim/canonical-v3-final/index.json \
  --out .local/md-yuyuhakusho/audio-score-claim/final-check-result.json
```

重采入口分别为 `score_claim_capture.py`、`score_claim_cache_capture.py`、`score_claim_inactive_capture.py`、`score_claim_pan_feedback_capture.py`，均要求 `--out` 为新的 worktree 内目录。`score_claim_restore_capture.py` 只重跑 observed 并复用旧 original controls。生成链为 `score_claim_normalize.py`（39 条）、`score_claim_normalize_extra.py`（首次音乐启动前 1 条）、`score_claim_project_v2.py`、`score_claim_project_v3.py`、`score_claim_finalize.py`；具体输入与来源由各版本 index 绑定，CLI 可用 `--help` 复核。

本分工未修改旧 producer／内核、公共平台、README、PROGRESS 或 Go 生产包，未提交。只清理逐 byte 确认重复的失败 control 及空目录，共 23,642 bytes；真实失败、修复前源和所有版本化 oracle 保留。Python 禁止字节码输出。静态路线为固定 ROM 与保存的 direct Ghidra 68000 输出；REA raw 接口不支持 68000／Z80，没有以其他 CPU 冒充导入。
