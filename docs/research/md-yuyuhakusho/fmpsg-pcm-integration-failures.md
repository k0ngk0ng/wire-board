# PCM 命令与完整芯片组合的先行失败边界

本项不修改已冻结的 PCM Device、Reset、Interactive 或 DSP。先从原 quiescent checkpoint 和相同 PCM 初始化过程被动读取完整 YM／PSG／FIR 初态，再验证两层：原 physical-port 流直接进入完整芯片；独立 Go Device 从资源、描述、typed命令／hold／DMA生成端口后再进入芯片。给定原端口输入的证明不能替代 Device 自主输出的证明。

## 必须先覆盖

| 场景 | 可能失败 |
| --- | --- |
| 完整原生初态 | 把有限 DAC-only 初态或冷 reset 拼成完整 YM／PSG；遗漏feedback／phase／envelope／noise／busy／FIR |
| native checkpoint恢复 | 原savestate不存完整fm_ptr／count／busy，前场景残留污染新初态；不得拟合字段使输出通过 |
| 真实地址与data | 挪地址到data cycle，reset-between-address-data的原2A变2B；旧latch反例必须保留 |
| 实际reset调用序 | sort／clamp把7183080 DAC与7183006 reset交换，跨7183008tick；原frame8／byte56反例必须保留 |
| reset witness | 用Device.Time／上个address或任意YM／PSG访问冒充已完成DAC；普通input回时合同被放松 |
| frame分组／lookahead | 按raw Cycle/F重新分帧，丢原group和已完成原子操作ahead；帧尾副效或FIR尾被裁掉 |
| metadata／bank／dummy | 已计入后续原时刻的45wait／DMA effect再加一次，或把非chip事件推进成额外采样 |
| 音乐／dual／替换／pause／reset | 只验single自然结束，忽略完整23场景、非零声像及reset后latch |
| 分层误标 | 把原事件时表驱动的Native A写成独立Device B；B不能读取原未来事件时刻作为输入 |
| save／restore | Device或chips在真实frame/半帧恢复后改变事件、PCM或完整typed状态 |
| Native／race／Wasm | 浮点或int宽度、slice别名、源码／夹具身份变化被忽略 |

Go E2E 与负向假设入口先于组合验收：data-only补地址和按raw cycle排序必须取得实际失败，不把旧20组排序无差异误标文件当red。正确模式不得修改原时刻、裁切首尾或跳过失败帧。所有临时输出、缓存和Go telemetry限工作区，原生采集nice单进程并与phase_clock协调。
