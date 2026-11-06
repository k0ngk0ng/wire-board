# 已消除 parser 工作的只读 sidecar 研究

正式 schema 2 的 39,045 个逻辑事件、115 条声部已提炼出有序 parser Work 研究条目，并与保留的原版读取／有限 flow 证据核对。本阶段保持 `production_resource:false`，没有改 Go、catalog、payload 或 42 goldens，没有新增 CPU 捕获，也没有给出实际 cycles 或完整时钟通过结论。

绑定的是正式**解压原 JSON** SHA-256 `735d012d3d3140e76f8f756f0ca757bdc4b8deeb5a66750dd4610e41e06af830`，不是压缩文件或重排后的 JSON。来源、测试、研究文件摘要见 [AUDIO_SCORE_FLOW_EVIDENCE.json](AUDIO_SCORE_FLOW_EVIDENCE.json)；范围先行约束见 [AUDIO_SCORE_FLOW_FAILURES.md](AUDIO_SCORE_FLOW_FAILURES.md)。

## 形状与状态归属

每条研究记录绑定 `{score_id, voice_id, section: prefix|cycle, event_index}`，其有限 Work 只描述已展开原路径的工作，不选择下一事件或写音乐状态。读取、计数及顺序来自原 parser，但原 PC、ROM offset、原字节、跳转目标、原栈／计数器和逐帧时刻都不进入条目。原共享 loop／return 字只在研究 reader 和 private provenance 中用于核对，不成为生产 owner。

| Work family | 结构与分支 | 已证明的边界 |
| --- | --- | --- |
| `token-read` | control／pitch／duration 分类；每个实际 token 一次 | 与原读取地址、值、role、顺序逐项核对；地址仅在 provenance |
| `control-dispatch` | retained action 或已消除 flow action | 不携带目标，不驱动音乐控制流 |
| `jump/call/return` | 已消除的有限 parser 工作 | 按静态展开顺序保留；原返回值和栈不进条目 |
| `loop` | first 与 taken 语义分支 | 不保存源 loop slot、count 或 counter byte；实际原分支另行核验 |
| `parameter-read` | 有序字段 role，可对同一值出现两次 | 静态指令证据，不冒充逐 operand 新运行观察 |
| `duration-probe` | take／reuse | 显式音高后的真实 lookahead 不能因复用而省略 |
| `duration-scale` | 有界加法次数 | 正式数据只出现 0、1；0 次加法仍不等于整个 family 零成本 |
| `pitch-resolve-site/note-commit-site` | 静态语法调用站点与早期／最终阶段 | body 的 live 分支与服务价格不由 sidecar 决定 |

当前每个变体最多 12 个 Work。它们是语义 family，不能直接当作 12 条原机器指令或固定总成本。未来 clock 须为已验证 family 明确提供架构操作和现场分支规则；未知 Work 必须报 Unsupported，不能默默按零成本处理。

研究选择器也已先取得 unknown Work 的真实 red，再增加严格字段和 profile 校验：未知 family、原目标字段、超出当前 0／1 域的加法次数及未知参数 role 均拒绝；合法条目的输出保持逐 byte 不变。这不等于已经提供这些 family 的时钟价格。

同一源 token 若将来派生多个 typed event，需要显式避免重复 fetch。本正式目录没有相关 E3 co-emit，当前提炼器遇到未恢复 retained token 会拒绝；不能外推为通用 parser VM。

## 循环入口不能任选一圈

对 prefix 和连续 3 轮 cycle 提炼后，只有下列 8 个 `cycle[0]` 键的首次／回绕 prelude 不同：

- `88`：voice 6、7。
- `95`：voice 1、2、5、6、7、8。

这些键在回绕时比首次多 `token-read → control-dispatch(jump) → jump`，其余 Work 相同；其余键在已查三轮中恒定。39,045 个键的首次保留事件与正式 producer provenance 一一对应，三轮源事件顺序也逐项一致。

与主 driver／phase-clock owner 对齐后，研究选择器采用临时 `Entry`：

| Entry | 从现有逻辑游标取得的条件 |
| --- | --- |
| `continue` | prefix 中的事件，或循环的非首项 |
| `from-prefix` | prefix 耗尽并首次转入 cycle index 0 |
| `cycle-wrap` | cycle 耗尽后转回 cycle index 0 |

不增加持久圈数，不复制源 stack。jump prelude 在下一次**实际 parse advance**时处理，不能在 note wait 期间预付。未知 Entry 或人工构造 `Cycle=true/Index=0` 却没有转换历史的检查点，没有可据以选择的成本分支，研究接口明确拒绝而不选择首变体。

## 原读取与有限 flow 对照

复用原 23 定义、115 条声部、122,810 driver tick 的既有原读记录，没有重跑模拟器。全部比较通过：

| 原证据 | 比较数量 |
| --- | ---: |
| token／duration-lookahead 读取 | 199,477 |
| control-before 与原共享 words／stack | 26,130 |
| control-after 的下一 parser 位置 | 26,105 |
| 早期／最终 note cleanup／commit 站点 | 127,691 |

实际 flow 包含 loop 3,388 次、jump 942 次、call 263 次、return 246 次。loop 分支为 first-taken 619、repeat-taken 2,163、repeat-exit 606；本窗口没有 first-exit 例子，不能声称该分支已有原生覆盖。记录窗口可能位于展开子程序内部，因此 call／return 总数不必相等。没有未完成的 parser-read 窗口后缀。

这证明原读取、流分支、下一 parser 位置及 commit 调用站点与研究模型一致，不证明 refresh、bus 等待、IRQ、参数服务或 DSP 成本已恢复。

## 参数顺序与重复读取

- F0 的读取顺序为 delay、rate、delta、steps；steps 的后续半步处理不属于另一次参数读取。
- E4 正常模式为 mode、pattern、start step、limit、period、elapsed seed。最后两项是同一个 operand 的两次读取；disabled 只读 mode。
- E8 分别读取 remaining seed 与 gate definition。
- EA 分别读取 tempo period 与 remaining seed。
- F3 首次读取建立 noise latch；unclaimed 时直接 PSG 写还会再读同一 operand。后一次由现场 claimed 分支决定，不固定塞进 sidecar。

这些字段 role 按保存的原指令顺序核验。原 hook 提供 token 和 lookahead 读取，并非所有参数的逐 byte hook，因此重复参数读取保持「静态指令证明」的证据级别。缺 sidecar、F0 错序、E4 period／seed 错序均先保存真实 red。

## 不能静态替代的部分

claimed／tie、note-off、gate、envelope／pan／modulation 的启用和有效性由 Operation 现场状态提供。bus、mailbox、chip busy、IRQ、refresh deadline 和全局时钟仍归原 owner。header 初始化、Root dispatcher 和世界音效请求不属于当前 event-key 的纯 parser 工作。

原 instrument seek 在加载或音量更新时按源 instrument index 多次推进。正式 ToneID 去重后不保证唯一对应这个成本来源，不能按 ToneID 猜次数，也不能复制原 index／RAM 状态来补齐。本阶段明确保留该资源服务缺口，未擅自扩成完整参数服务成本。

研究文件为 `.local/md-yuyuhakusho/audio-score-flow/first/research-sidecar.json`（原首次／回绕研究变体）和 `research-sidecar-logical-entry.json`（临时 Entry 选择形状）。二者均无生产 loader，尚不作为运行时资源。原路径、原字节及 native 对照保留在 private provenance。

## 复跑

所有输出须留在工作区；提炼器要求新的输出目录：

```sh
python3 -B scripts/yuyuhakusho/score_flow_extract.py \
  --out .local/md-yuyuhakusho/audio-score-flow/review
python3 -B scripts/yuyuhakusho/score_flow_entry_check.py \
  --input .local/md-yuyuhakusho/audio-score-flow/review/research-sidecar.json \
  --out .local/md-yuyuhakusho/audio-score-flow/review-entry-check.json
python3 -B scripts/yuyuhakusho/score_flow_native_replay.py \
  --out .local/md-yuyuhakusho/audio-score-flow/review-native.json
python3 -B scripts/yuyuhakusho/score_flow_native_check.py \
  --report .local/md-yuyuhakusho/audio-score-flow/review-native.json
```

没有 Go telemetry、模拟器捕获、临时 profile 或仓库外写入。保留缺失／错序 red、入口选择 red、研究条目、原证据绑定与检查结果。Operation 的最小 observed revision 与后续正式 sidecar 版本由各自 owner 独立推进。
