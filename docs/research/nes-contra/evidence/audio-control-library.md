# 原版声音逐 slot 控制库

日版有效声音 ID 1–129 已完成逐 slot 采集，共 **205 个独立案例、298,300 次原版音序更新**。其中 149 例有限释放、53 例控制状态循环、3 例直接 DPCM 请求；没有未决案例。205 例均以另一组独立 FCEUX 进程复录，完整状态、请求、初始 RAM、返回寄存器及结果一致。

该结果用于独立音频控制器的生命周期与内部请求调度，不导出新 PCM，也不表示浏览器混音、跨轨同步或自然游戏流程已经验收。

## 来源与分析边界

原文件为 `/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip` 中的 `Contra (J).nes`。ROM SHA-256：`6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe`。采集前后归档摘要一致，原文件只读。

沿用此前已经保存的 REA／Ghidra 证据，不重复开启分析会话：

| 入口 | 本次用途 | Evidence ID |
| --- | --- | --- |
| `$87A2` | 请求、ordered entries、slot 优先级 | `ev_5554287f70ce87694caf2b0e255945c28784299ecbf9b7ca1acec11107b001ee` |
| `$8719` | 原驱动初始化与清理 | `ev_e08468d95ffdf4089bc624a0f88f5e2d3dfb5b3d5c14a12676026f0450989006` |
| `$80D5` | 按原顺序更新 slot 0–5 | `ev_6a2ed3b2bf0081386bb276de49f9405700f628e02dba6091e997a365f97b84e1` |
| `$8103` | 单 slot 更新及暂停分支 | `ev_a7532f1f8f35f419f16cdaeb63a4edb14c94e61cd7b4c7f22633eed8ab70d043` |
| `$88DA` | 直接 DPCM 请求路径 | `ev_c3722e649e46f1bc7830fd901d45522ef7d53cd830862173081706b9aa4b480f` |

五项完整记录原样保存在 [audio-control-library-rea.json.gz](audio-control-library-rea.json.gz) 中。静态入口用于确定观察点；本库中的寿命、周期及请求来自实际原程序执行，没有从反编译输出估算时间。

## 隔离方法

每个 `(ID, targetSlot, context0194)` 独立启动 FCEUX。开发 NSF 使用完整音频 bank 2／3 及固定 bank 14／15；只在核验为 `FF` 的 padding 写入 init／play 包装，反向覆盖为空白后与原 bank 逐字节比较。先执行 120 次静默原版更新，再经 `$F9BC` 请求目标 ID，保存返回 A/X/Y/P/S/C、初始六个活动 ID 和目标音序指针。

请求返回后仅清一次其他 slot 的 `$0106+slot` 活动 ID，并保存五项地址及前后值。之后不再清理其他 slot，因此目标打击乐音序正常发出的 slot 5 请求仍按原程序运行。暂停输入为 0，`$0194` 默认 0；`$77/$7A/$7B/$7E` 的 slot 5 另采 `$0194=1`。开发计数器 `$05F0/$05F1` 进入正式采样后冻结。

每次 `$80D5` 返回固定包装位置后保存完整 2 KiB RAM，再按 A/X/Y/P/S 顺序保存 5 字节寄存器，共 2,053 字节。PC 由固定钩子位置约束；没有为寻找循环而忽略不同的 RAM 字节。

本实验刻意隔离初始其他 slot，回答指定共享初态下的独立生命周期。全声音并播时的请求拒绝、抢占和暂停须继续由原控制规则处理，不能把这些隔离初态写入正式游戏。

## 坐标与结果含义

- `tick=0` 是初次请求后的第一次原版更新。`releaseTick` 是更新后目标 ID 首次消失的零基 tick。例如普通枪 slot 4 为 52：tick 0–51 更新后仍活动，在 tick 52 更新时清除，即第 53 次更新。初始年龄为 0、先判断当前年龄再递增的控制器可以直接使用该值。
- `released` 只表示被测 ID 在该 slot 结束；同 tick 的结束回调可以启动新的 ID。
- 两个完整状态在 tick `p` 与 `q` 后相等，候选周期从 `p+1` 开始，周期为 `q-p`。采集器继续执行一个完整周期，逐 tick 比较全部状态及请求事件后才输出 `loop` 和 `releaseTick=-1`。
- 若采样上限内没有释放且未证明周期，结果必须为 `unknown`、`releaseTick=null`。本次 32,768 tick 上限没有触发该分支；最长案例实际记录 6,529 tick。
- `$7F..$81` 为 `direct-dpcm`，没有普通 slot，不伪造寿命。该状态不表示 DPCM PCM 播放长度已由本实验确认。

所有原请求都保存在证据包中。派生 `prefixRequests`／`loopRequests` 仅包含来自目标 slot、且请求时 source ID 仍为被测 ID 的事件。已经清除 source ID 的结束回调单列为 `endCallbacks`，不重复塞进时间表。

`callerPC` 为栈返回地址减 2。请求通过尾跳转进入时，它指向外层 JSR，不能当成直接发出请求的指令地址；`wrapperCallerPC` 仅在经过 `$F9BC` 时另行记录。`sourceSlot` 和 `sourceCode` 直接读取原版 `$0120` 及 `$0106+sourceSlot`。

## 关键结果

| ID／slot | 结果 | 说明 |
| --- | --- | --- |
| `$1A`／4、5 | release tick 52／34 | 复合枪声不能使用同一整体寿命 |
| `$20`／1、3、4、5 | 233／192／234／216 | slot 3 有 96 个内部请求，各 slot 分别结束 |
| `$46`／0–3 | 2,976 tick 周期 | 完整状态首次进入已证明周期的前缀长度可随 slot 不同 |
| `$6A`／3 | release tick 4,656，之后请求 `$6E` | 结束回调只在此 ID 的 source slot 3 实际出现 |
| `$6D`／3 | release tick 4,656，无回调 | 共用音序不会自动继承 `$6A` 的回调 |
| `$77`／5、`$0194=1` | release tick 194，之后请求 `$62` | `$0194=0` 没有此请求 |
| `$7A/$7B/$7E`／5、`$0194=1` | release tick 194，无回调 | 与 `$77` 的 slot 5 共用音序，仍按原 ID 区分 |

ID 1–126 的 ordered entries 保留原表顺序；全部实际目标 slot 均在 0–5，接受后的指针与对应最后一项一致。本版本未出现同 slot 重复 entry。ID 0 是无效表项；`$82..$FF` 不属于有效声音库，也不能作为重置别名。

## 产物与复验

- [audio-control-library.json](audio-control-library.json)：129 项定义、每 slot 生命周期、请求时间表、返回寄存器、4 个上下文变体及来源摘要。
- [audio-control-library-original.json.gz](audio-control-library-original.json.gz)：完整原始证据。每例保留初始 RAM、原请求 JSONL、结果和 `statesGzipBase64`；后者解码 Base64 后再解 gzip，得到连续 2,053 字节状态记录。
- [首次验证](audio-control-library-v1-verification.json)与[独立复录验证](audio-control-library-repeat-verification.json)：所有案例的原始状态与请求摘要。
- [采集前失败场景](audio-control-library-plan.md)：隔离、循环、回调及边界的验证约定。
- `.local/contra-audio-control-library/v1/`：每例开发 NSF、Lua、身份记录及压缩原始状态。正式游戏不加载这些 NSF。

```sh
# 回读证据包；检查完整状态周期、生命周期、请求分类和五种故障扰动。
python3 -B scripts/contra/audio_control_library.py --verify-only

# 独立重新采集；使用新的 label，旧完成目录只用于校验复用。
python3 -B scripts/contra/audio_control_library.py --ids all --label fresh --jobs 3

# 再启动一组独立进程，逐字节比较；保留主库不被部分进度覆盖。
python3 -B scripts/contra/audio_control_library.py --ids all --label fresh-repeat --repeat-of fresh --jobs 3
```

验证器先于录制实现编写。它拒绝不完整周期、状态单字节差异、请求差异、首尾边界差异，以及把未知寿命写成无限。模拟器使用工作区内独立 profile、runtime 和 tmp，SDL dummy 不向扬声器播放，不启动 WAV 录制器。每例完成即删除私有 profile／临时目录并压缩状态；重复录制的清理范围见 [最终核验](audio-control-library-verification.json)。
