# PCM 命令 Go 状态机的先行失败场景

本清单先于 Go API 测试及实现。父分工的原生观察、参考模型和旧 `pcmplayer` 保持只读。

- 单槽预装、双槽描述转交和实际起播的先后错误；仅从已初始化播放器开始却宣称完整启动。
- 普通／音乐描述协议、已接受目标与 sampled `StartDual` 被新层重新仲裁；音乐 pan、普通 routing 与 restore pan 混用。
- 替换、模式切换、另一槽追加、同资源新请求、自然结束或忙临界区顺序错误。
- 外部 hold 到来时原子操作过冲、release 的 15-cycle 对齐、跨帧持有和恢复位置错误；将整字节统一平移。
- bank／metadata／dummy 的读取、DMA effect、当前 frame 相位、超前读取和尾部源长度处理错误。超出 payload 的预取仅产生总线事件，禁止读取邻 ROM 或用零填充推进解码。
- 在 reference frame 边界保存恢复时丢失 typed phase、必要 latch 或尚未输出的混音结果；使用隐藏协程栈、历史输入重放、CPU／RAM／通用寄存器 VM。
- 不变资源建立第二 payload owner；快照或调用者可以修改设备 canonical 状态；伪造 ID、未知 descriptor／协议、错误目标被接受。
- 输入乱序／重叠／过期／大小超限、跨 frame 错误归属、时刻溢出或坏 DMA 在失败时部分提交。
- snapshot 身份、phase／part／loop、资源闭包、源游标、非法 busy／flags 或互斥状态不校验；非法状态导致 panic、无界循环或部分结果。
- 只比较 ACK 或最终状态，未逐 frame 比较原声音／总线事件、PCM 与 typed DSP；expected 来自实现或 Python 模型。

先冻结实际 original-only 12 组完整连续向量和 API tests，取得缺少 API 的真实编译 red。实现使用固定 Go 语义函数和静态原子延迟分区；对原程序 PC 的引用只在研究导出器分类原 native 事件，生产无 PC／指令／固件／ROM 地址。
