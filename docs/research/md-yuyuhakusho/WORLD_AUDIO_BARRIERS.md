# 世界声音不能采用固定帧内边界

复用已冻结的 baseline／pause 完整声音写入证据后，固定帧首、manager 后、arena 前三种候选在两个流中均出现反例。原分析器、原报告及原 trace 保持字节不变。本轮没有修改游戏或加入原 PC 时间表。

| 策略 | 保留前 1,799 帧原序后的首差 | 原／候选 selected | 此前相同分派检查 |
| --- | --- | --- | ---: |
| 帧首 | 第 2,071 视频步 | `7C → FF` | 1,766 |
| manager 后 | 第 2,071 视频步 | `7C → FF` | 1,766 |
| arena 前 | 第 2,396 视频步 | `7D → FF` | 2,091 |

完整冷启动域三者都在第 9 步出现 `04 → 00`，因此上表另保持战斗前缀原序，避免只用菜单反例决定比赛方案。既有帧尾控制反例仍分别为完整域第 375 步、战斗域第 1,813 步。

第 2,071 步实际由 collision 对象 `8100` 在声音 driver 进入前请求 `7C`；driver 随后在 arena callback 期间消费该请求。把消费移到帧首或 manager 后会提前至请求之前。第 2,396 步的 `7D` 来自第一个 fighter 对象 `8400`，发生在 arena 已经结束之后、RAM IRQ 中 driver 进入之前；固定 arena 前也会提前。具体原写点及上下文单独保存在 [counterexample-context.json](../../../games/yuyuhakusho/verification/world-audio-barriers/counterexample-context.json)。

这些是**给定原 IRQ 视频步和同一 driver 内部操作组**的有限因果证明。新候选到首次不同 selected／pause 就停止；没有用已分歧之后的原 PCM 凑结果。全流缺少指定 callback 时明确回退帧首，并记录次数。它们不能证明任意输入下的独立时钟、IRQ 接受或真实混音。

后续调度须将主世界的声音实际写点、RAM IRQ／VDP 相位、独立 PCM 的银行访问和 driver 操作成本组合起来。可交换的纯计算区间允许归约；跨越请求、pause 或 priority 写点的区间须继续细化。不能通过换成另一个固定边界、扩大声音缓冲或回放原时间表绕过这个问题。

复跑只读取既有工作区证据与旧分析器所需的固定研究 ROM，不启动模拟器：

```sh
python3 -B scripts/yuyuhakusho/check_world_audio_barriers.py
python3 -B scripts/yuyuhakusho/extract_world_audio_barrier_context.py
```

结果见 [result.json](../../../games/yuyuhakusho/verification/world-audio-barriers/result.json)。检查器先取得缺少探针的真实失败；第一次 shell 重定向早于目录创建而失败，按测试准备错误另记，不把它当成模型语义反例。
