# Score control 动作原版窗口

已从现有 Root control action／canonical-v2 只读提炼 94 个动作窗口：StopDACFlags 2 次、FadeStep 88 次、ResetScore 4 次。没有新 CPU 捕获、Go 执行、catalog／payload 修改或旧 42 goldens 变更。所有 original expected View／IO 都来自保留的原程序状态和请求，未由 Go 生成。

portable fixture 为 [native-actions.json.gz](../../../games/yuyuhakusho/audio/score/testdata/control-oracle/native-actions.json.gz)，SHA-256 `3ea26b82248967cc6e98d5b85dee38bdc174603a1d7dd2ddedad1f759bb05225`，34,401 bytes。源绑定与检查见 [AUDIO_SCORE_CONTROL_ORACLE_EVIDENCE.json](AUDIO_SCORE_CONTROL_ORACLE_EVIDENCE.json)。本分工不宣称 score Go action 实现已通过。

## Owner 投影与合法 setup

每个窗口只有 `before/after = {tempo_remaining, tempo_period, voices[18]}`；Root 的 priority、selected、queue、fade、pause、duck 和 choice 不进入 score 期望。owner 由原 actual selected／已恢复 header 定义取得；envelope cursor 使用既有 typed prefix／cycle 投影。原序列 PC、循环／返回字和驱动 RAM 只留 private provenance，不进入 GoState 或 fixture。

4 条实际有动作的 case 提供合法 whole-tick setup：从 `score.New` 开始，按原 accepted score dispatches 和先前 score actions 顺序调用。动作位于当前 `Advance` 前；其 native before 的 Tempo 与全部 score 行逐 byte 等于前一完整 native frame。该证明成立于当前 music82 的 Tempo0／control-idle 样本；不把它外推成非零 Tempo 的完整 orchestrator 调用位置证明。

fixture 的 setup 是合法状态重建路径，不把整段前缀 IO 当作此分工的 original golden。虚拟 Go `Ticks` 和 typed event cursor 不存在于原 RAM：三类动作都保留 Ticks，Stop／Fade 保留事件游标，Reset 随 voice 清空游标。调用者应比较自身 before／after Snapshot，而非从源 PC 人工填入。

## 三类动作的原证据

| 动作 | 窗口 | Score 期望 | Root 期望 |
| --- | ---: | --- | --- |
| StopDACFlags | 2 | 只清 DAC 四个 typed flags，保留 owner、channel、参数及其他 voice；无 IO | fade counters 由 Root 自己更新 |
| FadeStep | 88 | 音乐 rows 1～9 的 base level／active 变化；968 YM＋42 PSG，原顺序保留 | 不修改其他 Root 字段 |
| ResetScore | 4 | 只清 Tempo 与 18 voice；无 score IO，Ticks 保留 | FullClear 原 31 YM＋4 PSG 全部由 Root 编排 |

Stop 原采没有紧随 CLR 的完整 driver after。其 after 明确标记为「原实际 DAC after row＋其余 native before 字段＋单 flags-store 源投影」；没有把帧末已经推进过的其他 voice 冒充 immediate after。两个原样本均为 Active `true→false`，其余 NoteOff／Claimed／Tie 原本为 false；四位全部清除的规则由原整 byte CLR 指令证明，不声称三位 true 的原生分支已覆盖。

Fade 的 before／after 是原函数窗口直接采样。私有逐 byte 对照确认只有音乐行的 level／active 改变；Tempo、原序列／loop 位置、note wait、gate、modulation、pan 和 envelope cursor 均保持。PSG 使用 base level 直接进入 level-write，不额外推进或叠加音量 envelope。

FullClear 的 before／after 和全部硬件请求来自原窗口。portable 刻意将这些 35 条写放入 `root_hardware_intents`，`score_intents` 为空；ResetScore 不是旧 `04` 全系统实现的别名。不能从 fixture 中把 Root 硬件清音或 Root 字段清零归给 score。

## 已覆盖与未覆盖

原 FM before level 范围为 14～84，所有 88 个窗口中没有 FM 失活；原 PSG 跨过 level15 的失活共 6 次。该样本不能证明 FM `127→128`、`255→0`、carrier TL carry 跳写、primary claimed 下 fade 抑制、active-fade pause 或非零 Tempo 分段位置的完整覆盖。pause case 仅有 idle fade，无本次动作窗口，未算入 action cases。

## Fixture 形状与复跑

`cases[].setup` 给出 accepted score dispatches 和按 whole tick 排列的 actions；每个 `windows[]` 包含动作、当前 whole tick、owner 投影、`score_intents`、`root_hardware_intents`、after authority 和合法 before 边界证明。两组请求分别从 0 密集排序，不能混并为 score 输出。

[失败清单](AUDIO_SCORE_CONTROL_ORACLE_FAILURES.md)和缺产物 checker／red 先于 exporter。重复导出的 fixture 与 private provenance 逐 byte 一致，检查保持原内容通过：

```sh
python3 -B scripts/yuyuhakusho/score_control_check.py \
  --fixture games/yuyuhakusho/audio/score/testdata/control-oracle/native-actions.json.gz
```

原数据仍在工作区时，可向新的研究目录重建：

```sh
python3 -B scripts/yuyuhakusho/score_control_export.py \
  --out .local/md-yuyuhakusho/score-control-oracle/review
```

没有运行 Go、写 telemetry 或启动模拟器。仅清理本次重复导出 88,176 bytes，原窗口、真实 red、首份 portable／provenance 全保留；既有 Root 数据、score Go、Observed 与旧 goldens 均保持原状。
