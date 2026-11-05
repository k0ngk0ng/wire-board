# 完整 FM／PSG／DAC 合成层验收

独立 Go `audio/chips` 已在固定原生参考配置下，把有序 YM2612／PSG／DAC 端口输入合成为连续 PCM16。Native 和 Wasm 各自通过相同原生 oracle：29 组低层 YM 场景、416 次操作、616,921 个立体声芯片采样；5 组完整混合场景、192 帧、141,304 个立体声 PCM16 采样；每次检查的完整 typed 状态均相等。PSG 另有 5 场景、1,616 个检查点及 51,139 笔独立边沿证明。

这些数字证明给定输入与时间时的芯片／重采样行为。独立音序状态、PCM 时钟、总线、世界声音触发、平台流式播放接线和扬声器录音不在本项完成范围。完整索引为 [fmpsg-ym/validation.json](../../../games/yuyuhakusho/verification/fmpsg-ym/validation.json)，接口为 [audio/chips/README.md](../../../games/yuyuhakusho/audio/chips/README.md)。

## 参考配置与 owner

参考配置为离散 YM2612、集成 MD PSG、44,100 Hz、stereo、FM preamp100、PSG preamp150、FM／PSG HQ、filter0。低通实际关闭，不保存无用低通状态。原参考 core 和 hooked core 对同一私有原生输入的原始输出、完整 scalar 状态均相等；混合检查还确认所有非音频 guest RAM 和视频保持原样。

`YMState` 保存参数、oscillator 相位、包络、反馈两步历史、连接延迟节点、定时器、CSM 和 LFO 时钟。延后生效的 keyscale 与原参数确有不同电路阶段；其余 phase increment、率索引、EG 输出、连接指针和 LFO 波形临时推导。生产无原寄存器 RAM 镜像、原生 context、ROM、CPU 或任意 PC。PSG 独立状态及统一 FIR 各有唯一 owner。

FM 接线用八种小型连接图表示，而非保留原函数／指针结构。对数正弦和指数表按方程生成；PM 只保留 bit8 的四分之一波数值，其他频率位独立缩放并截断。重采样复用 `audio/dsp` 唯一 step-kernel；新增的值返回接口不修改旧 DAC 算法。来源、数学式与 notice 见 [SOURCE.md](../../../games/yuyuhakusho/audio/chips/SOURCE.md)。

## 原生检查范围

| 层 | 已验证内容 |
| --- | --- |
| Raw YM | 六通道、八算法与反馈、DT／MUL／TL／KS／AR／DR／SR／SL／RR、共享地址与频率 latch、同 cycle 顺序、零长度调用 |
| 动态电路 | 包络三分频与 4095→1、八种 SSG、运行中改形与 key-off、全 LFO 速度、AM／PM、三音模式、Timer A／B、CSM |
| DAC 与离散输出 | 第六通道相位暂停但包络持续、enable、pan、正负 ladder、9-bit 载波量化和各通道限幅 |
| 完整混合 | FM＋PSG＋DAC 共同进入一个 HQ FIR；逐帧 PCM、typed YM／PSG、integrator、18-tap tail、忙标志和 ahead |
| 恢复与边界 | 每帧与半帧未消费输入恢复、slice copy、非法状态／端口／时间／容量、overflow 原子拒绝 |
| 回归 | 原有 DAC DSP 6 项检查与 PSG 3 项检查；Native race、Wasm、vet |

FM 与 PSG 超前处理有实测区别。FM raw write 可能先把 oscillator／envelope 推进到帧尾之后；flush 丢掉 FM buffer 中超前输出，但不能撤销芯片状态。PSG 边沿已直接插入 FIR，超前边沿必须进入后续 tail。PSG 同 cycle 的不同 delta 也不能先相加，逐笔整数插值与合并后插值可能差 1 LSB。

原生 busy flag 使用 42 个 master cycles 的网格，写入后保持 32 格。两个数据端口均使用最后共享地址 latch；离散 YM 的 status 只在 port0 有效。混合 oracle 包含有效／无效 status 读，并对全部读回值和 busy deadline 比较。

## 先行测试与真实失败

[FMPSG_FAILURES.md](FMPSG_FAILURES.md) 在生产前列出了失败场景；Go 原生轨迹测试先取得缺实现的真实 red。完整混合测试也先取得独立缺接口 red。kernel 值返回合同先 red，再新增单个 `dsp/kernel_view.go`；测试实际修改返回数组，确认不能污染原表。

首次 raw oracle 直接载入 dylib，没有初始化可选每通道音量，六路默认全为零。原版与 hook 输出相等是真实观察，但不能用于证明有声 FM 输出；该 fixture、旧 producer identity 和第一次 Go 失败均保留为 `uninitialized-config`。修正只使用既有 reference frontend 初始化真实 config，不跑 gameplay 帧，再执行相同 YM 私有输入。已写测试的输入／状态断言未放宽，Go YM 算法没有为该修正改变。不能把初版全静音原数据写成完整声音覆盖。

完整混合首轮 PCM 已相等，但两个场景 busy deadline 失败：首差为 `896266 != 897414` 和 `896462 != 897582`。错误来自把 busy 时钟猜成 7 master cycles；原生状态和源码常量指出应为 42。先行原生测试保留原 red，修正该独立时钟后 192 帧全部通过，没有修改原 PCM／状态或降低断言。

## 复跑

所有缓存和临时文件限本 worktree。原 oracle 可直接重放，无需运行模拟器：

```sh
TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry" \
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -json -count=1 ./games/yuyuhakusho/audio/chips/... ./games/yuyuhakusho/audio/dsp
```

Wasm 使用同一原生夹具；最小环境及完整命令记录在验证索引，避免 Node／Go Wasm 启动参数空间上限。重新采集必须输出到工作区内的新目录，保留冻结夹具：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/fmpsg_capture_ym.py \
  --out .local/md-yuyuhakusho/fmpsg-review/raw
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/fmpsg_capture_mixed.py \
  --out .local/md-yuyuhakusho/fmpsg-review/mixed
```

新原始捕获必须与固定原版及 hook 双路一致；不能使用 Go 输出充当新的 original。生成数学表后进行 byte identity 检查。没有上传资源、提交、部署或接线世界声音。
