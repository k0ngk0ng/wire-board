# 音乐采样仲裁与发布

`BeginMusicNotePublication` 从有效 DAC 音符的 final commit 开始，连接既有 commit 计时、实际总线申请、现场仲裁、accepted intent 发布及最终 return。`BeginMusicPublication` 从已完成 commit 的 continuation 开始；两个入口复用相同内部实现。前者的所有阶段共享一份临时 clock、refresh deadline 和预算。

请求只携带逻辑 `DescriptorID` 和唯一 score owner 的完整 `score.Pan`。不可变 descriptor 从真实 PCM Device 的既有 library 取值，不能由调用方伪造 rate 或 priority；payload 仍由生产 resourcebridge 借用同一份只读资源。helper 不读取源 ROM、PC、源索引、指针或 RAM。

Begin 保存中断屏蔽状态并只等待 BUSACK，随后读取当前 request marker。bit7 为 1 时直接进入 release 尾部，不读取 priority；否则读取当前 ModeFlags 的高两位并比较 descriptor priority。它不等待 busy mailbox，也不因 marker 1 而误判 blocked。`Feedback` 是此次现场读取的临时结果，不能缓存到下一拍。

Begin 停在 metadata effect 之前。对应 accepted DAC intent 到达时，调用方验证 `ExpectedInput` 并调用一次 Apply；被拒绝的请求不调用 Apply。提前 Finish、错误或重复 Apply、失败后继续均被拒绝。`ServeMusicNotePublication` 是相同阶段的完整调用，不重复返回的诊断 intent。

Apply 按原操作边界计入 marker、六个 descriptor 字段及末尾 track pan 的八次 ZRAM 写入，包括每次额外 7 master cycles。PCM player 此时持总线暂停，原版的 main 中断也被屏蔽，因此这组中间 metadata 对两个 CPU 均不可见；canonical metadata 在末 pan 写入时整体发布，并绑定首 marker 与末 pan 的实际时刻。没有为源地址和指针建立可执行 RAM 写接口，也没有复制 payload。

调用方须保持该屏蔽区间的调度约束。Finish 释放同一总线、恢复中断屏蔽并消费最终 return。helper 不实现世界 IRQ，也不自动 flush 参考帧或提交整场状态；任何失败后均须丢弃相关 detached owners。

PCM 的 `PublishAcceptedAt` 校验实际末写入时刻与 Input.End 一致，Begin／End 属于当前未 flush 帧和当前 open hold，再调用原有 canonical publication。既有不支持的 metadata overlap 与 scheduled-boundary guard 继续拒绝。mixed 只提供代理和原子分支，不新增 marker、rate、priority、pan 或 descriptor 的动态副本。

原版对照覆盖正式六个音乐 descriptor、marker 0／1／128／255、六种 ModeFlags 值、两种完整 pan 及不同 refresh profile，共 288 个场景、两份既有 pinned core、1,152 次执行。原 observer 开关前后的完整状态、RAM 和步骤一致；Go E2E 使用正式四份压缩资源和真实 mixed Runtime，比较现场读值、bus 访问时刻、metadata 首末写入时刻、最终 canonical 字段和结束 clock。完整原始八次写入仍保留在 oracle 中。

声明范围为初始 held bus、busy 9、无额外 IRQ 的固定 profile，不以私有入口证明自然可达性，也不表示整场音频或世界时序已经验收。失败计划、冻结测试和复跑入口见 [score-clock-music](../../verification/score-clock-music/)。
