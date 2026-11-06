# 普通 PCM 发布的先行失败场景

- 把状态读取与发布误当同一总线持有区间；源程序读取 3／5 个状态字节后释放总线，仲裁后再次申请。发布必须保留先前采样的 StartDual，不能重采当前 ModeFlags。
- single、强制 A／B、空闲声部选择、两声部均忙和 choice bit 的阈值／读取顺序混用；busy 64／128 都表示占用，只有 0 空闲。
- 新建仲裁规则或持续 status／choice owner。实际决定继续由 resourcebridge／engine.SelectPCMCommand 产生，Root 独立持有唯一 ChoiceBits，PCM Device 持有实际 flags／busy。
- 拒绝时发布 metadata 或更新 choice，强制／空闲选择时提前读取 choice，或在正确阶段之外应用请求。Root 选位写入、header 工作、二次 BUSACK、metadata、释放和 return 均按真实阶段计时。
- 7F diagnostic／duck 或无效资源误进正常发布；descriptor metadata 必须与实际 PCM library 一致，不能由调用方伪造。
- 遗漏 single 的 8 次、dual 的 10／12 次 metadata 写入、ZRAM 额外访问、header 解析和 refresh 成本。总线持有且中断屏蔽时只整体提交 canonical metadata，不复制原指针／ROM／RAM。
- 时间越过未 flush 帧、scheduled hold／metadata overlap、外部失败、预算和 clock overflow 被吞掉；错误／重复 Apply、提前 Finish 或失败后重入必须拒绝。

已有 13,036 条原版仲裁记录用于语义依据；新计时对照覆盖全部 122 个正式 ordinary descriptor 和正常资源可达的仲裁分支形状。原版只作固定私有 profile 研究，测试、失败计划及 oracle 先于实现冻结；不以该证据替代世界 IRQ 或整场声音验收。
