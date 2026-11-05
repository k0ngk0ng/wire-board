# 音序参数只读提炼

基线 `54a5b9ae`。本阶段新增只读 helper 与独立源检查，不改已有音乐研究、encoder、有限音频数据或平台。候选数据只包含语义字段，原地址、表范围和摘要另存研究 provenance；这些结果不构成 DSP 或声音播放验收。

## API 与数据边界

实现为 `scripts/yuyuhakusho/score_parameter_tables.py`，结果使用不可变 dataclass。FM patch 以 `to_score_data()` 导出完整驱动参数；旧 `to_data()` 仅保留冻结的 YM 有效字段视图。完整 pan 数据使用 `read_score_pan_patterns()` 与 `decode_score_pan_automation()`。具体输入契约见私有 `.local/md-yuyuhakusho/audio-score-parameters/API.md`。

| 参数 | 提炼结果 |
| --- | --- |
| FM patch | algorithm、feedback 和 4 个 operator；包括 detune、multiple、TL、volume carry bias、rate scale、attack、AM、decay、sustain、sustain level、release 与 carrier。operator 按寄存器 slot 0、4、8、12 的顺序归一，源写入次序为 0、8、4、12。仅按已提供的 bank/index 读一个 25 字节 patch，不推断 bank 长度。 |
| Pitch envelope | 全部 8 条表转换为 typed 前缀／循环 tick。signed offset、scale-add、keyoff 及计算 bypass 分开表达，运行时无需原控制字节或游标。 |
| PSG 音量 envelope | 全部 11 条表转换为 level／无写／keyoff tick；保留 hold 与重复 keyoff 节奏。候选 level 写入仍受 voice owner 的 note-off、claim 和 gate 抑制。 |
| E4 | 3 个 named stereo shared window、5 个 named mode 和 step/period 定义；可变 elapsed 由单独运行状态持有。仅输出 stereo 变化，低 LFO 位由原 owner 保留。完整 3 个窗口均有索引 0..3 的实际读取。 |
| E9 | AM enable slot、global LFO enable/rate、AMS/FMS 和 pan OR-enable；AM 写入按原源次序返回。仅包含 YM 硬件有效字段。 |
| FM3 | 已证 4 个 offset 值及 music/effect domain。原频率 port A9、A8、AA、A2 对应 register slot 0、1、2、3，与 FM patch 归一顺序相同。 |

YM 忽略且不参与驱动控制的保留位不复制到候选数据，例如 E9 第二参数 bit 3；若比较原始 YM 请求字节，须使用独立研究旁证，不能声称 typed 参数恢复了这些未保存位。TL bit 7 虽被 YM 忽略，却影响驱动加法 carry，因此以 `volume_carry_bias` 单独保留；不另存 raw TL owner。未写入的 SSG 参数不虚构。

## 特殊路径

- pitch hold 在同一 tick 回读前一个值，随后每 tick 重放；PSG 音量 hold 则交替产生无写与重放前值。
- pitch keyoff 设置 note-off 并绕过 base pitch、detune 和 modulation 求和，返回值为 `131`；是否在实际曲目中落成紧随其后的频率写入，仍待父任务的原读／YM 事件 oracle 核实。
- PSG 音量 keyoff 将位置退回前值，保留前值／keyoff 的重复节奏，不将其提炼成一次性结束。
- E4 mode 1 仅在音符边界推进；mode 2 在音符边界重置，期间单次推进；mode 3 在音符边界重置，期间循环；mode 4 每次 driver tick 重置。period 为 0 时保留 8 位环绕语义。实际 initial step 可大于 active limit；原读取还会跨越相邻名义 pattern，完整窗口分别为 `right,left,right,both`、`right,both,left,both`、`both,left,both,right`。
- FM 普通音量更新在 base level 作为 signed byte 为负时整体无写；否则对 carrier 的完整 driver TL 加法，若 carry 则跳写。initial patch load 则写全部 operator，carrier 的加法按 byte wrap 处理。
- E9 原程序把 `A0` 加 9，却通过 `A1` 从 bank 开头读 4 字节。helper 保留此源行为；调用者须提供原读 oracle 实际观测到的 bank，不能改为读取当前 patch 的 decay 组。

## 验证与证据

先冻结失败清单和检查器，保存缺失实现的 `ModuleNotFoundError` red，再实现 helper。FM3 routing 增补也先产生缺失契约字段的 red。私有目录保留各阶段源码快照、日志与 SHA 绑定。

运行：

```sh
python3 -B scripts/yuyuhakusho/score_parameter_tables_check.py \
  --out .local/md-yuyuhakusho/audio-score-parameters/check-result.json
```

检查器中的原表解释器独立于 helper。首轮冻结检查通过 8 条 pitch 共 4,270 个 tick、11 条 PSG 音量共 22,528 个 tick，以及 E4 共 36,000 次名义表分派；另覆盖合成 reset/hold/jump/keyoff/scale-add、signed/wrap、全部 256 个 E9 第一参数、FM operator/carrier 排序与 FM3 domain 边界。该检查器源文件保持冻结。

完整原读复核运行：

```sh
python3 -B .local/md-yuyuhakusho/audio-score-parameters/native_read_check.py \
  --index .local/md-yuyuhakusho/audio-score-source/full/index.json \
  --probe .local/md-yuyuhakusho/audio-score-source/probe-patch-reads.json \
  --out .local/md-yuyuhakusho/audio-score-parameters/native-full-outcome-proof
```

本次实际范围为 23 个定义、115 条 header 声部、122,810 个 driver tick、2,941,523 个有序事件。以下检查均通过，输入文件 SHA 与 native index 另行绑定：

| 类别 | 实际核验结果 |
| --- | --- |
| FM patch | 2,525 次实际读取、113 个 bank/index 引用；63,125 次完整 8 位 YM 请求的 port、register、顺序和值一致。typed 有效字段单独比较；每次读取另遍历 256 个 level，共 646,400 个 carry／signed 边界。 |
| PSG 音量 | 175,674 次实际读取的 pointer/index/value 及输出一致：116,384 次写入、59,290 次无写。含 3,797 次 hold 与 21,438 次 keyoff。实际 ID 为 1、2、3、4、5、10、11。 |
| E4 | 242 条控制定义、27 组不同五参；5 个 mode 和 3 个 shared window 全到达。79,435 次读取与 B4 请求（含低 LFO 位合并）一致；27 组实参另做共 16,200 次独立源投影。 |
| 未到达项 | 本次 23 定义没有实际 pitch envelope、E9 或 FM3 读取；volume ID 6、7、8、9 也未到达。只保留静态／合成证明，pitch keyoff bypass 的实际写入仍未证明。 |

完整参数补证先保存 `to_score_data()` 缺失的 red，再实现 TL carry 参数；E4 共享窗口也保存真实参数触发的旧范围拒绝和新 API 缺失 red。候选 patch、原始请求与来源记录分别保存，不通过 mask 隐藏未知驱动差异。

来源为固定 Japan ROM，SHA-256 `c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06`；补充指令列表由已安装 Ghidra 的 `68000:BE:32:default` 有界生成，未通过 REA raw 68000 路线。FM3 slot 映射另绑定固定 native `ym2612.c`，SHA-256 `67bca180dd13c61a1e68901035bba871bb0124fe09a4b79192d6242013672a04`。

私有产物包括各阶段 red manifest／源码快照、`parameter-instructions.txt`、冻结检查结果，以及 `native-full-outcome-proof/{typed-patches,native-read-provenance,native-read-result}.json`。候选表和来源文件分开；前者没有原地址、原控制指令、ROM 片段或任意 PC。全部结果是参数与请求级证明，不冒充单初态连续 score 或 DSP 验收。
