# 世界帧前后阶段：实现前验证范围

本阶段组合对象遍历之前的计数、受信逻辑控制器采样结果、暂停、逻辑计数、软复位请求、一次世界 RNG 与反馈，以及对象遍历之后的色盘推进／应用。原版顺序是 `2300` 先推进，再按 `F500.bit0` 调用 `233c`；不能把上一帧色盘当成本帧呈现结果。

控制器硬件探测／总线握手由开发参考内核或平台采样层负责，本接口只接收已归一化的 `ControllerFrame`，不信任客户端自报按下沿。正常步被慢动作门槛跳过时，新的采样结果不得覆盖仍由人物读取的旧控制器状态；视频侧计数仍推进。对象更新期间的 `SlowFlag` 可改变，不能在整个世界轮次固定使用旧视图。

可能失败的场景：跳过／重复视频计数；按新慢动作而非入口旧状态决定采样；暂停切换后还使用旧暂停位；同时 Start 多次切换；复位后仍消耗 RNG 或更新对象；冻结期间丢掉旧按下沿；把硬件归一化游标当作规则输出；先应用再推进色盘；条件关闭仍修改颜色；晚期缺数据部分提交；逐帧回灌期望时钟／种子；把对象遍历前后组合误称整个世界循环。

先保存原版自然完整前后边界与明确私有输入的原 CPU 组合结果，再编写测试并保留实际缺实现失败，最后实现。私有输入的归一化结果是声明的外部采样输入；原内部时钟、RNG、反馈和色盘结果均不得注入 Go。完整世界循环的对象顺序、场景和呈现另行组合验收。

三项测试已在当前原生竞态和 Wasm 全包回归通过：6,576 次自然前后调用、1,440 次隔离调用（含 40 个复位）和 4 条单初态流的 320 次调用。前置缺实现失败和最终源码／日志绑定见[独立对局验收](../verification/independent-session.json)。

## RAM IRQ 证据复跑范围

复跑工具只恢复原版取证过程，不改变游戏规则。执行前需防止这些失败：使用了不同 ROM／内核或输入序列；只比较最终截图而遗漏中间帧变化；按宿主内存字节序提取 68000 代码；把视频帧当成必有一次的逻辑调用；把观察到的 RAM 字节误称全部已执行；复用共享 Ghidra 项目导致覆盖；缓存或输出写到 worktree 以外。验证先逐帧比对冷启动基线，再提取第 2,400 个视频步的 `FFF000..FFF13F`，核对既有 320-byte 证据摘要。可选反汇编使用独立项目及缓存，静态指令不作为动态路径覆盖率。

从 worktree 根目录执行：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_world_frame_irq.py \
  --out .local/md-yuyuhakusho/world-frame-irq-replay --disassemble
```

不传 `--disassemble` 时，只运行原版并核对字节／上下文；不会启动 Ghidra。可通过 `--ghidra-install` 和 `--java-home` 指定已经安装的工具。输出、Java 配置和缓存均留在 worktree 内；Ghidra 的临时项目使用 `docs/research/md-yuyuhakusho/static-work/frame-irq-*`，成功后删除项目。原始 `.local/md-yuyuhakusho/world-frame-irq/irq.bin` 和 `report.json` 保留，脚本禁止用该目录承接新输出。

实际复跑的 2,400 个视频步全部保持原内核 RAM／画面基线一致，提取的 320 bytes 与旧证据一致，SHA-256 为 `5b0cea90c181d6ff10b647f718b7447be6930d907925d3301b0b40344afbeb25`。独立 Ghidra 输出与已保存的指令清单逐字节一致。规范[字节及上下文夹具](testdata/world-frame-irq.json)和[验证报告](testdata/world-frame-irq-validation.json)绑定 ROM、内核、输入、冷启动基线、脚本和静态资料摘要。本工具只用于研究，不进入 Go／Wasm 运行时。

## 战斗 IRQ 与旧包装入口

第 2,400 个视频步的实测上下文是 `F580 SceneMode = 52`、`F582 SceneRequest = 0`、`F510 InterruptMode = 4`、`F512 VideoMode = 2`，四者不能混用。下表为已采字节的静态指令关系；自然调用证据来自前后阶段采集器，不能由本表推定所有分支都已自然执行。

| 路径 | 可核验的指令关系 | 对采集边界的影响 |
| --- | --- | --- |
| 战斗 RAM IRQ | `FFF130` 调用 ROM `32D6`；正常分支 `335E` 调用 `3A2E`，`3A70` 尾跳 `2300`，返回 `3364` | 不能只接受旧包装的 `2F3C` 作为 `2300` 的返回地址 |
| 战斗色盘应用 | `3364` 检查 `F500.bit0`；开启时 `336C` 调用 `233C`，返回 `3372`；关闭时跳 `3390` | 收集器接受 `3372`／`3390` 两个结束边界 |
| 战斗忙分支 | `32D6` 检查 `F50E`；忙时经 `3312 RTS` 返回，不调用 `3A2E` | 一个视频步不保证产生一个逻辑 tick，不按视频标签差补调用 |
| 旧非战斗包装 | `2F36` 调用 `3A2E`，返回 `2F3C`；`2F44` 调用 `233C`，返回 `2F4A`，关闭时跳 `2F6E` | 旧路径仍须保留，不能只支持战斗 IRQ |
| 场景请求 | `3E6C` 读取 `F582`，为零时直接到 `3E78 RTS` | 默认战斗中该分派为 no-op，不能按猜测的场景编号改写世界 |

来源分别是 [RAM IRQ](../../../docs/research/md-yuyuhakusho/static/world-frame-irq.txt)、[战斗包装](../../../docs/research/md-yuyuhakusho/static/world-frame-battle.txt)、[旧包装](../../../docs/research/md-yuyuhakusho/static/first-control.txt)、[帧内尾跳](../../../docs/research/md-yuyuhakusho/static/input-and-tick.txt)和[场景请求](../../../docs/research/md-yuyuhakusho/static/world-frame-scene.txt)。REA 当前不支持此 raw 68000 输入，以上反汇编直接来自 Ghidra，不称为 REA 反编译。

硬件归一化函数 `2C9E` 使用 `F554` 两个字节作为通道扫描游标，见[驱动指令](../../../docs/research/md-yuyuhakusho/static/world-frame-driver.txt)。受信采样 API 的规则比较只排除这两个 scratch bytes，不排除时钟、RNG 或色盘。业务调用可能跨 `retro_run` 视频边界，采集器在第 3,600 步停止接受新调用后再排空挂起调用；不能丢弃最后一次返回。
