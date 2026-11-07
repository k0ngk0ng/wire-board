# 逻辑世界帧前置：先行失败范围

- 帧计数、慢动作、输入归一化、暂停切换、暂停确认、逻辑计数、重置弦、随机数和反馈的顺序错误；跳过前置分支时仍推进 RNG／按键或逻辑计数。
- 把实测总耗时或原 PC 序列作为运行时，漏掉数据相关分支、逐通道循环、16-bit overflow、动态 DIVU 或内部 refresh 跳过。
- 在 hook 夹入声音 IRQ 后用旧 Clock.AudioPause 或 SoundQueue 副本覆盖 live owner；VINT 的 Palette.Flags 更新被返回的 World 副本丢失。
- 归一化读取与工作 hook 的开始／完成语义错位；把旧采样结果始终锁在整个调用入口，或把未计价的归一化按零成本跳过。
- 原帧门槛为 slowGate 和入口 slowFlag，重置检查独立于归一化 count／pauseMask；count0 的暂停 do/while 仍检查 channel0，simultaneous Start 只能切换一次。
- 暂停确认只在真实 bit7 转换时写，hold 路径必须保留中途变更；反馈暂停／slow hold 请求与 decay，但最终显示输出仍按当前幅度及逻辑奇偶。
- normalize／work／资源或后续对象失败时泄漏前置、控制器、RNG、音频、主时钟或世界状态；没有请求的工作区间误当零耗时。保持旧纯值 API、测试和原生预期。
