# 音乐与 primary 初始化整拍

`AdvanceDriverTick` 在 `AdvanceRequestTick` 的同一个内部执行器中增加正式 catalog 的音乐与 primary 技能音轨分派。queue、dispatcher、停止旧声部、header 初始化和普通声部扫描共用同一个临时 clock、refresh 与预算；普通 PCM 分派沿用已有路径。原有 `AdvanceRequestTick` 保持冻结的普通 PCM／FF 支持闭包。

`Catalog.HeaderPlan` 返回当次初始化所需的只读投影，保留实际定义中的 slot、channel 和原顺序，调用方可丢弃或修改返回值，不会改变 catalog。计时层不持久保存第二份声音定义、原始 RAM、原版地址或执行游标。

音乐分派先执行 stop-primary、stop-secondary 和 music reset，再装入音乐 header 与声部参数，完成原版 key-off 扫描后结束当前 driver 拍。下一拍才推进新音乐。primary 分派按定义 claim 音乐声部、执行必要的 PSG3 双 mute、初始化参数并处理 secondary claim，随后继续本拍 18 个声部扫描。

stop-primary 复用已验证的 restored patch／pan 服务，但保留自己的 key-off、目标选择和 return 边界。第三路 PSG 在 all-primary stop 中恢复 secondary slot 17，即使该 slot 不活动；普通 track-end 继续使用原音乐 slot。该差异由完整原版切换调用验证，避免多写一次噪声寄存器。

Root 的 priority clear 和音乐 reset 的 priority save／restore、selected reset 使用同步一次性 effect lease，在实际时刻访问唯一 owner。header／音乐声部的连续清零沿用规则层明确的 bulk effect；本 profile 不在其中插入 IRQ、外部 owner 访问或回调，不声称证明逐 long 中断可见性。`DriverResult.Fields` 是当前调用的诊断记录，不用于再次播放。

本页的原版初始化矩阵限定 idle control／fade／pause、没有 pause restore payload，并使用调用方给定的主时钟起点。完整 driver 后续已增加 01／05／04 与活动 fade／pause，当前接口见 [DRIVER_CONTROL.md](DRIVER_CONTROL.md)。selected 0 的重初始化、活动 secondary 音轨以及 world IRQ／请求生产时刻仍未接通。Root、score、queue、bus、chip 与 PCM 使用可丢弃的同一分支，只有成功后才联合提交；失败不提交 score，但可能已经修改外部 owner，接口不自动回滚或 flush。

原版完整调用、先行失败与复跑说明见[验收目录](../../verification/score-clock-dispatch-init/)。本接口通过不代表联合音频 checkpoint 或平台整场播放已经完成。
