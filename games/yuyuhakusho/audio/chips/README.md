# FM／PSG 固定参考配置合成

`chips` 使用独立 Go 算术，把有序 YM2612／PSG 端口输入转为连续立体声 PCM16。Native／Wasm 在同一原生 oracle 上逐采样、逐状态一致。此包不执行 CPU、固件、原核心、ROM、RAM 镜像或原事件时间表。

固定配置为离散 YM2612、集成 MD PSG、FM preamp100、PSG preamp150、两路 HQ 重采样、44,100 Hz、stereo、filter0。它重建固定参考 core 的行为；没有声称对所有真机版本或其他滤波配置通用。

```go
stream, err := chips.NewStream(
    chips.Config{Profile: chips.ReferenceMixedHQ44100, MasterOrigin: origin},
    typedInitialState,
)
err = stream.Queue([]chips.Event{
    {Cycle: at, Kind: chips.YMPortWrite, Port: 0, Value: 0x28},
    {Cycle: at, Kind: chips.YMPortWrite, Port: 1, Value: 0xf0},
    {Cycle: at, Kind: chips.PSGByteWrite, Value: 0x90},
})
frame, err := stream.FrameBoundary()
pcm := frame.PCM // interleaved int16, 735 or736 stereo samples
saved := stream.Snapshot()
```

`MasterOrigin` 是参考音频时钟原点；`Cycle` 相对此原点，不是视频帧内相对时间。`Queue` 可分批输入且复制 slice，保留同 cycle 顺序，不提前推进芯片。`Snapshot` 深拷贝未消费队列，可在半帧分批之间或完整帧边界恢复。每帧最多 4,096 笔输入，原 raw lookahead 上限为 1,008 master cycles。非法输入和失败帧不提交部分芯片／FIR 状态或 PCM。

每次 `FrameBoundary` 提交 896,040 master cycles。FM data 写和有效 status 读先生成 `chipTick<rawCycle` 的芯片采样，再访问端口。FM 地址写不提前推进芯片；两个数据端口均使用最后共享地址 latch。忙标志按 42 个 master cycles 的网格，持续 32 格。`YMStatusRead` 的读回在 `Frame.Reads` 中返回，属于给定输入流的结果；这不是可在帧内阻塞等待的 firmware 交互接口。

FM 和 PSG 的帧尾处理不同。FM 原 raw lookahead 采样不进入本帧 PCM，但已推进的 oscillator／envelope／timer 状态保留；参考 FM buffer 随 flush 清空。PSG 边沿已直接提交 FIR，超前边沿必须进入后续帧的 tail。每笔 PSG delta 独立插值，不能先合并同 cycle delta，否则会改变整数取整。

`YMState` 只保存芯片参数和独立动态状态：相位、包络、反馈历史、延迟节点、CSM、定时器、LFO 与延后生效的 keyscale。phase increment、包络输出、率索引、路由及 LFO 波形均临时推导，不持久化原寄存器镜像或第二份原生状态。PSG 状态归属见 [PSG 接口](psg/README.md)。统一 `State` 另保存最后已提交 FM 输出、FIR integrator／18-tap tail、输入 ahead、忙标志 deadline 和未消费队列。

低层 `NewYM`／`WritePort`／`WriteRegister`／`Generate` 支持原芯片采样检查。`Generate(n)` 在每 1,008 个 master cycles 对应一个原芯片采样，返回交织 int32 原输出；其调用边界包含参考 timer-B 批处理语义。跨平台比较使用相同的调用分块，不能把独立低层调用随意合并后宣称 timer 状态等价。

本项给定端口输入及时间，只证明完整芯片合成与重采样。独立音序 driver、PCM／DAC cadence／prefetch、总线时序、世界声音触发及平台播放接线由各自 owner 完成；不要将本包通过写成游戏声音已接通。验证、真实失败及复跑说明见 [合成研究](../../../../docs/research/md-yuyuhakusho/FMPSG_DSP.md)，来源见 [SOURCE.md](SOURCE.md)。
