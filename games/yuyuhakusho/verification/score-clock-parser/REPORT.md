# 正常音序语法成本验收

新增 `PriceParserWork`、`FinishParserControl` 已恢复 token 分类、中央控制分派入口、有限 flow 和时长倍率的固定架构操作。它们只计算当前语义工作，不驱动音序状态，不执行原 PC、opcode、ROM 或原 stack，也不把原逐帧时间表放入生产资源。

同一份原程序在既有两份 core 上完成 148 个有限私有 profile、592 次原生执行。每个 profile 的只读记录前后完整 native 状态、RAM、寄存器及两 core 的每步时钟均一致。保留的 860 个原指令边界与 71 个源位置仅作研究证据；生产按具名语义规则逐操作检查 refresh。DAC／FM token 路径、四种 loop 分支、call／return、倍率 1／2 及 parser 回边均覆盖。

Go 测试先于新增实现冻结，真实缺 API 的 red 保留。原生竞态和实际 Node/Wasm 各通过 35 项顶层测试、1,017 条记录，无跳过，包含全部旧声音时序回归；静态检查通过。本批按游戏内部改动范围验收，没有扩大运行其他游戏或全仓库行为矩阵。

实际 score／parser 资源的 39,045 个键、39,053 个变体全部经真实 loader 及逻辑 Entry 选择核对。63,728 个结构性 Work 已有非零价格；另外 116,103 个现场音高、参数、lookahead／commit 等 Work 继续明确返回未定价，不会静默按零处理。资源原有 `unpriced` 标记、原音符、采样、优先级、旧 goldens 和旧实现均未修改。

原 oracle 及命令来源见 [native-evidence.json](native-evidence.json)，冻结源码与产物见 [validation.json](validation.json)。离线审计使用 `scripts/yuyuhakusho/verify_parser_clock.py --audit`；复跑使用 `--mode native|wasm|vet --out <新的仓库内目录>`。原生有界复采入口为 `score-clock-parser-capture.py`，仍须使用新 `.local` 目录及相同固定 core／原版资源。重复 native 夹具和两个已退出 session 的空目录已清理，原 core／ROM 与唯一验收夹具保留。

本批不证明整个正常 driver、独立世界／IRQ、音高／资源参数 body、采样发布或最终混音的完整时序。平台试玩构建没有切入这些未完成的音频组件。
