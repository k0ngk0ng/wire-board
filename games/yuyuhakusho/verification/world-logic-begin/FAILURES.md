# 真实逻辑世界前置与联合主时钟：先行失败范围

- 从3A2E到3A5A间漏价输入归一化、暂停、RNG或反馈，把未实现对象回调当成完整World。
- 归一化在opcode结束hook之前已锁存读值，getter未先排空主批次与IRQ；或getter和Work重复计当前操作成本。
- 中途VINT改变Palette.Flags／缓存目标显示、声音消费改变AudioPause后被Clock／World副本覆盖。
- 原每帧CPU相位、垂直信号、状态读取、CRAM DMA、门控释放和帧结算错位；后台bank争用未按独立观察的实际目标累计。
- 设备历史与normalized Controllers重复存储；漏掉reserved carry、无设备通道清理、count0、慢动作与重置；输入getter、预算或晚期对象／音频失败泄漏状态。
- 当前暂停／慢动作分支未核验却放宽阈值，或修改既有native端点、旧helper结果与冻结测试来迎合实现。
