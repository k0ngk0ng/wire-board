# 声音 driver 的局部端口时序与待补语义操作

给定 canonical YM 请求入口、当前 main cursor／refresh deadline 和外部 bus／chip 状态后，独立 Go 服务已重建原版端口握手。48 个声明私有状态在两份 native core 上逐操作相同，Go／Wasm 直接使用 `chips.Interactive` 产生 busy 反馈，472 个有序访问、16 个 level-cache 写点及最终时刻／refresh deadline 全等。

完整 driver tick 仍没有独立 master-cycle 赋时。当前 typed score 消除了原有限循环／调用／返回解析路径，`Output.Intents` 和 before／after 状态不足以恢复这些成本；`ClearPrimaryPriority` 汇总 bool 也不能确定原实际写点。本轮没有把原 PC 或逐 tick 时间表送入运行时。

## 已恢复的 canonical YM 服务

[audio/scoreclock](../../../games/yuyuhakusho/audio/scoreclock/API.md) 从 bank 0／1 的 canonical register 服务入口开始，执行寄存器范围判断、必要的 level-cache 路径、临时中断屏蔽、Z80 bus 申请与 BUSACK 读改写、mailbox 检查、两次 busy 等待及地址／数据写、释放 bus 和返回。

原 busy 不能用一个「YM 写成本」代替。同一起点和相同请求，未 busy 的服务为 1,995 master cycles，seed busy 后为 2,261；前者两次 status 读，后者四次。真正的 busy deadline 属于 chip owner，服务在对应时刻调用 `ReadStatus`，只测试 bit7。地址与数据间保留原分隔操作，两个 bank 都从 status port 0 读状态。

每次从 68k 访问 YM／ZRAM 额外增加 7 master cycles；BUSREQ 控制访问是不同接口，不能一并加上该延迟。main refresh 在每个固定语义操作的原检查边界生效，不能统一追加到函数末尾。[cost-source.json](../../../games/yuyuhakusho/verification/score-clock/cost-source.json) 将本服务的常量绑定到固定 core 的架构成本与原静态清单。生产只保留固定语义控制流，不包含该研究报告中的 PC、opcode、通用寄存器或可执行指令数组。

本服务接受外部 bus 的同步后 main cursor，用于以后与独立 PCM owner 产生的 bank 占用协同。它不保存第二份辅助播放器、mailbox、busy 或全局时钟。level effect 也只报告原写点，不保存原 level RAM 影子。

## 原生 fixture 与真实失败

先行[失败清单](SCORE_CLOCK_FAILURES.md)、无时间意图 red、Go 缺服务 API red、完整 fixture 和测试源码保留在[验收索引](../../../games/yuyuhakusho/verification/score-clock/validation.json)。缺 API 后实现期间没有修改冻结测试或期望。

原生 oracle 从明确的 quiet 检查点构造 private origin、refresh、held bus、mailbox0 与可选 seeded busy。固定原 core 自己执行每条原指令；测试工具逐次启动原 core，PC 与步骤只保留在研究证据。每例分别在不装 recorder／装 recorder 下比较原执行边界、完整 serialized state、refresh 和 RAM；两份固定 native build 的端口事件也相同。

初稿在 seed busy 后使用 native savestate 恢复，实际失败。固定 `sound_context_load` 只恢复 `fm_cycles_start` 并令 count=start，没有恢复 `fm_cycles_busy` 或临时 FM buffer pointer。修订 oracle 因而每次从同一声明 quiet fixture 重新构造 seed，并将 native fixture 的 busy0／buffer 起点明确重建。旧脚本与真实失败保留，没有修改 `reference.py`、core 源码／二进制或游戏代码，也没有把这个私有重建写成 native 半帧恢复已经完备。

当前 48 例只证明 bus 已持有、mailbox 为 0、服务尾部没有额外 IRQ 插入的范围。未就绪 bus／mailbox 的有限预算与失败传播已有先行测试，其完整原生成功路径、异步 PCM 占用及恢复中断边界仍须另外验收。调用者还必须在 detached bus／chip owners 上执行并成功后提交；服务失败不暗中回滚外部 owner。

## 实际 pause 与 priority 边界

新的自然只读端口 ledger 复用旧 pause 控制，3,602 步 RAM、寄存器、视频、PCM 与样本数逐步相同。只保留 pause／resume 周边及八个既知 priority 清理窗口，共 54 帧、9,688 条 IO／状态事件；没有重采或更改已完成的 851,037 条 bank 证据。原窗口与精确边界见 [natural-boundaries.json](../../../games/yuyuhakusho/verification/score-clock/natural-boundaries.json)。

- 第 3,001 步，世界先写 `F807=1`，driver 入口读到 1，随后实际写 `FF`，再发生静音相关的 YM 服务。
- 接着 39 个 `FF` driver 入口没有 main CPU 的 YM 端口访问，外部世界／IRQ 的持 bus 状态保持 3。不能仅从一个 leaf service 的最后一次释放推断整个帧的总线状态。
- 第 3,041 步，世界写 `80`，driver 实际清为 0 后释放 bus，再恢复 level。held Z80 的旧 uint32 frame cursor 已发生回绕，原 release 直接写 ceil15 的当前恢复时刻；不能与旧游标取 max。
- `390f6` 的 primary priority 清理位于 keyoff 与后续 restore 写之间。音乐清理还有 `38808`、`38a3c`、`38a66`／`38a6e` 等实际写点，不能统称总在第一笔 YM 前或最后一笔 YM 后。

这些是已观察窗口的真实顺序，不是新输入下的独立时刻预测。暂停确认／恢复属于 Root 的唯一 pause owner；当前 score 自身没有复制它们。

## 完整 tick 所需的操作合同

下一版 score 可以在正常 `Advance` 的同一语义执行中输出临时操作，或提供可中断的 detached transaction。操作不能持久保存原 PC、指令、RAM、循环栈或另一份规则状态，也不能把原实测耗时直接列入 catalog。

| 操作组 | 需要保留的语义信息 |
| --- | --- |
| driver gate | normal／pause-start／pause-hold／resume，以及有序 pause 字段写点和中断屏蔽／恢复边界。 |
| tempo／dispatch | countdown 是否重装、实际活动 music 声部数量、dispatch 清理／初始化的阶段顺序。 |
| voice scan | 全部 18 个 slot 的活动／跳过分支、FM／PSG／DAC 类型、claimed／tie／note-off 分支。 |
| note／gate／参数 | countdown、gate、note lookahead、调制、pan、envelope 是否读取／循环／结束，以及对应有界迭代次数。 |
| 有限 parser 路径 | typed event prefix／cycle 对应的有限循环回边、末轮和调用／返回语义次数；由唯一事件 cursor 推导，不保存原控制栈或跳转地址。 |
| ownership | claim、restore-forced-rest、patch／level／pan 恢复的有序语义阶段。 |
| Root 字段 effect | 每个 `PriorityClear`／queue replace 的真实操作位置，与 chip 意图的相对顺序；不能只用 tick 汇总 bool。 |
| 外部反馈 | 原 PCM Blocked／Priority／PanRedirect 的各自读取点；不能把入口快照复用于不同时刻。 |
| chip service | canonical bank/register/value 请求、相应外部 bus／mailbox 查询和即时 chip status。 |

扁平意图不能反推出已经消除的 F6／F7／F8／F9 控制路径，也不能知道一次 YM 等待期间 PCM feedback 是否变化。若反馈会影响后续 driver 分支，需可中断的协同 transaction，或先在 detached owner 上按语义阶段模拟；不能用未来 oracle 值填平。

## 复跑与保留范围

Go／Wasm 的命令、来源 SHA、先行 red、native 重采和清理范围均由验收索引绑定。重新采局部服务和有限自然窗口：

```sh
PYTHONDONTWRITEBYTECODE=1 nice -n 10 python3 scripts/yuyuhakusho/score-clock-capture.py \
  --out .local/md-yuyuhakusho/score-clock-review
PYTHONDONTWRITEBYTECODE=1 nice -n 10 python3 scripts/yuyuhakusho/score-clock-natural.py \
  --out .local/md-yuyuhakusho/score-clock-natural-review
```

本 scope 不修改 `phaseclock`、score、chips、PCM player、engine 或平台页面，不更改总索引和进度百分比。PSG store 的独立语义成本也尚未纳入本服务。实际游戏声音、完整 driver 赋时、世界 producer 到 IRQ 的独立排程与发布仍未由此完成。
