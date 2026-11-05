# PSG 独立分频、噪声与音量边沿

纯 Go `audio/chips/psg` 已匹配固定 MD 参考配置下的连续 PSG 状态和逐笔振幅边沿。Native、race、Wasm 各 3 项顶层测试与 5 个子场景通过，`go vet` 通过；5 个场景共 1,616 个操作检查点、51,139 笔非零边沿与 36,793 个立体声原生 PCM 采样帧。

本证明限定在 PSG 子层。它接收给定的寄存器输入及 master 时间，不独立生成世界声音事件、音序时钟或总线等待；完整 FM／PSG 混音、HQ FIR 接线及平台播放仍由对应范围继续完成。机器索引见 [validation.json](../../../games/yuyuhakusho/verification/fmpsg-psg/validation.json)，先行失败场景见 [FMPSG_PSG_FAILURES.md](FMPSG_PSG_FAILURES.md)，API 见 [包说明](../../../games/yuyuhakusho/audio/chips/psg/README.md)。

## 实际原生配置

从固定原进程读取 `config`、PSG 标量和声道增益，确认 `hq_psg=1`、`psg_preamp=150`、双声道、44,100 Hz。MD 实际选择集成式 PSG：内部周期 240 master cycles、零分频等于 1、反馈插入位置 15、白噪声 XOR taps 为位 0 与位 3。初始 latch 为 3、四声道振幅为 0、四个振荡器为低电平，噪声移位初值为 `0x8000`。

原生配置的噪声寄存器为 16 位；插入位置编号 15 不能误写成 15 位移位寄存器。完整白噪声观察取得 57,337 次移位的实际周期，共运行 74,670 次移位，没有假设周期一定为 65,535。周期噪声另跑完整循环，三种固定噪声速率和 tone 2 联动均在操作矩阵中。

| 场景 | 操作检查点 | 非零边沿 | 重点 |
| --- | ---: | ---: | --- |
| latch-tone-volume-noise | 152 | 1,521 | 所有衰减、音调分频边界、全部噪声控制和相同周期写 |
| raw-byte-fuzz | 1,403 | 572 | 固定种子的原始数据字节，包括无 latch 后续数据 |
| white-noise-complete | 44 | 37,378 | 完整白噪声周期与跨帧连续状态 |
| periodic-noise-complete | 6 | 466 | 零分频、tone 2 联动和周期反馈 |
| frame-ahead-boundaries | 11 | 11,202 | 帧尾、超前 195／1,008 cycles、同周期继续写 |

## 观察器不改变输出

`fmpsg_psg_capture.py` 不修改或重编译原内核。它在实际已加载进程中直接调用导出的 `psg_write`、`psg_config`、`psg_end_frame` 和原生 blip API，读取 PSG 的独立标量状态；不将原 context、指针或 guest RAM 带入生产。

先执行原始批量 API 操作作为控制组，再在相同重置初态下逐 240 周期调用相同增益的 `psg_config` 观察正负沿。每个原操作检查点比较完整 typed 状态、完整 blip 缓冲摘要、offset、integrator 和全部 PCM；另外用每笔观察边沿注入新建的原生 blip 缓冲，再比较相同内容。原固定内核与已有 hook 内核的批量控制也完全相等。所有操作前后的 guest RAM 和视频保持相同。

逐步观察的边沿来自原声道高低电平、噪声反馈和 deferred delta 的实际变化。`PendingDelta` 与波形变化分别保留，未把相同时间的多笔写合并。原 blip 每笔分数插值各自取整，合并会改变低位，因此输出接口也保留逐笔语义。

这里驱动的是私有、明确声明的芯片 API 输入，不是新增的完整原游戏对局证明；随机字节和零频率属于边界输入，不声称每一种都由原游戏自然触发。

## 独立状态归属

`State.Cursor` 保存最后接收的原始输入时间；`NextTick` 保存内部同步时钟，两者在两次内部更新之间不同。`NextEdge[4]` 保存四个分频相位；修改周期并不重置相位，只有噪声切到 tone 2 联动时复制该声道计数器。

`ToneDivider[3]`、`Level[4]` 和 `NoiseControl` 是唯一寄存器语义值，不另存周期、衰减编号或放大后的 `chanOut` 副本。`High[4]` 是振荡器极性，噪声输出位另由 `NoiseShift` 持有；二者不能混为一个值。音量变化在下一次内部更新提交，真正未消费的幅度差由 `PendingDelta` 独立保存。

每次 API 操作在 State 值副本上完成，合法后一次提交。倒序、未知 profile、非法相位／幅度／反馈、时钟溢出或超过两个视频帧的单次推进均拒绝；失败不改变原状态或返回部分边沿。保存恢复覆盖待提交音量和噪声中途状态。

原 `psg_end_frame` 只把局部时间坐标减去帧长；生产采用同一绝对时间线，不维护冗余帧号。已超前产生的 PSG 边沿直接存在 blip 尾缓冲中，不能按 FM 临时输出的规则丢弃。

## 测试先行与来源

失败场景、外部包 E2E 测试及原生边沿夹具均在生产实现之前写入并冻结，真实缺失实现失败保留在 `fmpsg-psg-missing-red.jsonl`。随后生产首轮已通过全部 5 个连续场景，未通过改写原生期望值取得通过。测试之后仅执行 `gofmt`，格式化前的完整测试字节与 SHA 另存。

原生观察器有两次初始化错误，均发生在生产实现之前：第一次错误地把 PSG 标量结构大小写为 232，实际 ABI 为 200；第二次新建 verifier blip 设置速率后未重新 clear，初始 offset 尚是默认比例。两个真实失败日志保留，修正只涉及观察器的 ABI 断言与速率初始化，不修改音频结果。

原 `psg.c` 来源固定于 Genesis Plus GX 提交 `60bee4c724859710ce05672239e5b601ad01382e`，具有非商业许可条件。本生产实现未复制、翻译或链接其实现，采用独立 typed divider／feedback／事件方程；原源文件仅作为本地只读行为 oracle。振幅数据由 `floor(2800 × 10^(-attenuation/10))` 量化，最后一级关闭，随后按固定 150% 整数比例放大。硬件规则参考原文件引用的 [SMS Power SN76489 文档](https://www.smspower.org/Development/SN76489)。

原内核 SHA 为 `1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da`；已有观察内核 SHA 为 `e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306`。原生完整结果保留于 worktree `.local/md-yuyuhakusho/fmpsg-psg-native-verified/`；可移植测试夹具只含 typed 输入、状态和边沿，不含 ROM、固件、CPU 指令、RAM 镜像或原 PC 时间表。

## 复跑

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/fmpsg_psg_verify.py --mode all

PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/fmpsg_psg_capture.py \
  --out .local/md-yuyuhakusho/fmpsg-psg-native-review
```

验证脚本将所有 Go 工具、telemetry、模块缓存、构建缓存和临时目录显式限定在 worktree，并记录实际命令、来源 SHA、日志 SHA 和退出码。当前范围没有上传、发布、部署或修改公共平台。
