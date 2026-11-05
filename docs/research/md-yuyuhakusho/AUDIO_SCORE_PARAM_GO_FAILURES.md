# Go 音序参数失败场景

本清单先于 `audio/score/param` 的生产实现。先以实际原读／请求及独立源解释器生成 fixture，编写 Go 检查并保存缺失实现的真实 red，再实现 helper。

- FM operator 源写入顺序 `0,2,1,3` 与归一 register slot 顺序混同；B0 先于各组、TL 最后，以及每组的写入次序改变。
- FM algorithm、feedback、AM、rate scale 或包络字段丢失；carrier mask 与 algorithm 不一致；虚构原版未写的 SSG 字段。
- TL bit 7 被 YM 忽略便丢弃，导致后续音量加法 carry 不同；把 initial 的 byte wrap 与 ordinary 的 signed／carry 跳写混为一谈。
- 将 chip-effective 参数请求表述为完整原 8 位请求重建；忽略位的投影没有逐 register 声明。
- 包络从每 tick 的原状态注入，未验证从初始 cursor 连续推进；前缀到循环、空前缀、循环回绕、结束与 reset 边界错误。
- pitch hold 与 PSG volume 交替无写混同；pitch scale-add、signed offset、word wrap、keyoff bypass，或 volume 重复 keyoff 被省略。
- PSG attenuation 的 byte wrap 与 15 上限顺序错误；没有 level 的无写 tick 却产生写入。
- E4 mode 0～4、共享四步窗口、initial step 大于 active limit、period 0 的 8 位回绕、音符边界重置与持续推进错误。
- keyoff 后仍推进 pan；pan 操作覆盖 caller 保管的低 LFO 位。
- envelope、pattern、mode、slot、范围非法或 cursor 越界时，panic、静默接受，或部分提交输入状态。
- snapshot cursor 仅表示当前 typed 前缀／循环位置，却同时持久保存等价的累计 tick 或原 RAM cursor。
- 新 Go 原生结果通过，Node／Wasm 与原生不同；fixture、源文件与测试 red 没有摘要绑定。
- 本次未实际到达的 pitch envelope、E9、FM3 被冒充原曲实际执行覆盖；参数 helper 通过被冒充完整 driver、DSP 或声音播放通过。

父 driver 负责 gate、tie、modulation、claim、已加载 metadata 与寄存器低位合并。参数包只维护明确的不可变参数与最小 typed cursor／pan state。
