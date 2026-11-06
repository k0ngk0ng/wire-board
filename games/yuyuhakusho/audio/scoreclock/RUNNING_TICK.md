# 完整稳定运行拍计时

`AdvanceRunningTick` 串联实际 Root gate、control-idle、tempo、fade-idle、空队列检查、FF no-dispatch、18 个声部扫描及 return。每个实际声部通过统一 `BeginVoice` 的私有共享入口执行；所有 header／voice／恢复服务共用一个临时 clock、refresh 和预算。默认整拍上限为 65,536 个固定语义操作，显式较小预算在整个调用中有效。

输入明确要求：初始 queue 为空、selected 为 FF、audio-pause 为零、没有待恢复 pause payload、fade steps 为零及已审计 idle projection。selected 为零会进入原版重初始化，不可当成 FF no-dispatch。未支持的 dispatch／header、非 idle fade／control 和 pause 入口在访问外部 owner 前拒绝，既有全局 RequireCost 未计时错误继续保留。

外层扫描保留 DAC 后标记清理、FM／PSG 两套循环、primary／secondary domain 切换、DBF repeat／final 成本，以及最后一次 return。DAC held-note 覆盖 2、零值回绕和 255 的等待路径；DAC track-end 已从整个 driver 返回，随后不再重复执行扫描或最终 return。

Tempo work 来自实际 transaction 的当前观察，counter 与 scan 只由 canonical score driver 推进。Actual primary 的正常发声分支使用 voice-local bank；sibling 恢复和 Root priority／queue 继续使用唯一 effect lease。声部中产生的新 queue 请求保留给后续拍，不在当前拍尾再次消费。返回的 Intents 只作诊断，不重放。

本接口使用给定主时钟起点，尚不负责 world IRQ、声音请求生产时刻、完整 dispatch／pause／fade 调度、联合 checkpoint 或平台播放器。传入的 Root／score／bus／chip／PCM owners 必须是 detached 分支，只有成功后才能联合提交；错误可能留下外部访问，接口不代为局部回滚或自动 flush。

整拍原版和 actual Root／mixed E2E 见 [score-clock-running-tick](../../verification/score-clock-running-tick/)。
