# Typed 音序数据与连续回放失败场景

本清单先于本轮 PSG 扩展音高与声部停止边界修正。既有原读、catalog 和 note 验收的冻结检查继续使用，不能通过缩减原事件或注入逐 tick 状态使结果通过。

- PSG 只保留常规 70 项表，拒绝原版实际到达的索引 121～127；或为补齐数组盲拷贝未到达的相邻 ROM。
- 将扩展读取当作新的控制指令、地址或指针；候选数据只能表达索引对应的频率 divider，来源位置另存 provenance。
- 提炼过程中漏掉前缀／循环内的 transpose，错误使用 FM 的音符偏置，或在移调累加后没有保留 7 位索引语义。
- 轨道循环的 transpose 净变化非零时，仅扫描一轮便声称所有未来索引都已覆盖。
- 扩展项重复、超出 0～127、与常规表重叠或引用缺失，回放仍静默填 0。
- 音乐初始化时推进了当前 tick；效果初始化时反而多等待一 tick。
- `8f` 的 DAC 结束退出整个当前 driver tick，却继续推进后续声部；或误把普通 FM／PSG 停止解释成退出整个 tick。
- 所有声部只共用一份累计 quanta，无法表达 DAC 停止后的局部停顿；或把该停顿与下一 tick 的 tempo 暂停合并而丢失一次。
- DAC 结束时重置共享 tempo；已结束声部继续参与音符调度。
- 比较时漏掉 FM rest／PSG 显式音符产生的早期旧时值 outcome，或混淆最终时值 outcome 的顺序。
- 只比对音符数量，未逐条比较 step、voice、duration、base pitch、transpose 和 level。
- 将 note 投影通过表述为 gate、modulation、claim／restore、总线请求、DSP 或实际听觉输出已经完成。

本轮使用已保存的 23 定义、115 个 header 声部与 127,691 个原版 note outcome。新增边界检查先保存缺失字段的真实 red；实现变更后原检查器保持不变复跑。结果必须绑定 catalog、源文件与原读索引的 SHA-256。
