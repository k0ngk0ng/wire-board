# Typed 音序 driver 失败场景

此清单和 `driver_test.go` 先于主 driver 实现。参数子包拥有独立的先行失败与原版 fixture；不以参数级通过替代下列连续验收。

- Catalog 含未知事件、未知音色／包络／采样、重复 ID、倒退时间、零长度循环、短 pitch 表、越界声部、未终结有限轨道或过大资源，仍被接受。
- 构造、恢复或推进失败后部分提交状态或 chip intent；调用者修改 catalog、快照或输出 slice 能修改当前 driver。
- 音乐初始化当前 tick 继续推进，效果初始化多等一 tick，或 `8f` DAC 结束未退出当前 tick。
- tempo 只暂停音符而没有保留 gate／modulation／pan／envelope 的实际 driver tick；效果错误套用 music tempo。
- gate0、gate1、8 位环绕、tie、rest、previous pitch、早期 cleanup 与实际 key-on／off 顺序错误。
- 软件 modulation 的初始半步、delay、rate、翻向空 tick、signed delta、16 位累积与 note／tie reset 错误。
- E4 note-only、一次／循环／每 tick reset、period0 与共享窗口读取错误。
- PSG envelope 在已 note-off、claimed 或 tie gate 关闭时错误写入；hold／keyoff 节奏错误；换 envelope 误把 typed phase 当原 byte cursor。
- FM 普通 level 更新使用当前 patch algorithm 代替已加载 carrier algorithm，或丢失 TL carry／signed no-write。
- claim 在错误声部上发生、隐藏 music 停止推进、效果替换时误停其他槽、stop 没有当前参数 forced-rest 恢复。
- 把 root queue／priority／pause、PCM pending 或 DSP registers 再保存为 score 的第二份 owner。
- DAC pending=1 被当作阻塞，未区分 high bit；错用 ordinary PCM 表的 sample ID；外部 feedback 被永久复制进 state。
- 返回意图丢顺序、漏请求、把 PSG latch/data 交换、YM bank/channel 错误；以宽泛 mask 隐藏未知差异。
- 用原 PC、源地址、逐帧时间表、CPU／RAM VM 或每 tick 注入原 score state 取得一致。
- 只比 note outcome 或摘要，不比较每 tick 全部 canonical state 及完整有序 chip intent。
- 单一初态通过但 snapshot／恢复、Go／Wasm 或 caller ownership 不一致。

原 PC、有限计数／调用栈和 duration multiplier 在提炼时消除；root queue／priority／当前遍历 phase 不属于本包。原 envelope byte cursor须映射为 typed prefix／cycle位置。排除范围必须固定说明，不能随差异扩大。chip-effective YM 投影只允许明确已证的硬件保留位，仍比较全部请求的数目和顺序。
