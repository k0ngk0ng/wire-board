# 实际逻辑世界前置与联合对象准备边界

从世界中断入口，经图形 DMA、跨帧与垂直中断，再到输入归一化、暂停、随机数和反馈，现可由一次事务和同一预算推进。返回边界为 3A5A 的对象准备入口。角色、碰撞、对象扫描及世界中断尾部仍待计时，完整比赛音频继续关闭。

## 已接通的调用

`session.BeginBattleWorldLogic` 从真实逻辑世界入口开始；`session.PrepareBattleWorldObjects` 将它与前置世界中断／包装合并。底层分别调用 `AdvanceBattleWorldLogicBegin` 和 `AdvanceBattleWorldIRQToObjects`。正常分支返回 `AwaitingObjects`；busy 包装不再推进第二次逻辑世界。全部新音频帧只返回一次，晚期预算失败会撤销此前已计算的 DMA、音频、控制器和世界字段。

Clock、Controllers、RNG 等直接借用实际分叉 World。`BattleMainContext.NormalizedDevices` 只保存上一轮归一化的设备类型；normalized frame 仍以 World.Controllers 为唯一 owner。输入端口表示上次实际采样结果，不保存原始 80 字节包或寄存器文件。

归一化的读取先锁存、work hook 后计到操作完成。主协调器先在 LivePorts getter 排空当前批次和挂起 IRQ，再返回最新采样；后续 work 前置为空转，只计一次当前读取。原读到的 device type 在 IRQ 后继续有效，后续 nibble 可来自最新采样。可信 reserved carry 保持 16-bit 存储语义，物理按键保持 12 bit；相关完整证明见[归一化验收](../controller-normalization-work/README.md)。

## 真实原版范围

新取证保留 16 次实际 3A2E→3A5A 调用，包含 5,935 个 CPU 工作边界、460 次 I/O、167 个后台批次目标，以及单独观测的 4,900 个主机争用周期。10 次调用在归一化中途跨帧并执行真实垂直中断，再返回原调用继续。逐项比较全部时刻、refresh、VDP、pin、mask、IRQ 阶段、标志、控制器、RNG、Clock、Palette 和图形锁存。

原生采集前两帧尾部控制从未开启观察器的固定原核心执行获得；原有 2,400 帧视频／RAM／PCM 控制逐字段相同，额外两帧只用于排空已开始的调用。一帧可能完成前一帧留下的调用，又在晚些时候开始新调用，因此采集保存全部实际调用，不在视频帧末重置观察器。

后台 ROM 读取的主机争用独立观测后明确作为 CPU oracle 的外部输入。当前整段前置时刻仍是给定入口条件；另外的真实混音器 E2E 验证生产音频、跨帧、JSON 恢复与晚期回滚。它们不等同于从新建房间开始已经独立推进整局。

另有 4,568 个世界 clock／RNG／反馈完整原版调用、71,555 个边界，以及 689 个归一化完整调用、147,216 个边界。原版预期与旧纯值 API 结果继续一致。见[前置工作说明](../world-frame-work/README.md)。

## 视频与场景字段修正

真实状态接入发现先前 wrapper 投影把 F512/F514 视频模式误当场景阶段。现在明确区分：

| 原字段 | 实际 owner | 本次自然值 |
| --- | --- | ---: |
| F512 | World.VideoMode | 2 |
| F514 | BattleGraphics.PreviousVideoMode | 2 |
| F580 | World.SceneMode | 52 |
| F582 | World.SceneRequest | 0 |

生产入口与视频回调改用正确字段。旧 wrapper 的 typed 输入投影和新前置的错误标签投影重新导出，原投影及源文件完整归档。全部原生 fixture、CPU／I/O／后台／总线输入和时序端点保持不变；没有放宽任何时刻断言，见 [context-correction.json](context-correction.json)。旧验证器保留冻结路径的归档核对，并使用纠正后的输入复跑。

实现前冻结新前置、归一化、共享 owner 和整段合并测试。原缺 API、编译接线、旧字段校验失败及最终运行记录均保留。唯一整段合并测试的准备修正是在实现前把 natural2393 的前置行索引从 10 改为实际的 7，全部断言未变，见 [objects-index-adapter.json](objects-index-adapter.json)。暂停探针的准备修正在前置工作目录单独记录。

## 复跑与检查范围

在仓库根目录执行，输出目录须位于仓库内且尚不存在。验证器保存命令、环境、输出与摘要，并将 Go 缓存、模块、遥测和临时文件限制在工作区。

```sh
python3 -B scripts/yuyuhakusho/verify_world_logic_begin.py --mode native --out .local/md-yuyuhakusho/logic-recheck-native
python3 -B scripts/yuyuhakusho/verify_world_logic_begin.py --mode native-race --out .local/md-yuyuhakusho/logic-recheck-race
python3 -B scripts/yuyuhakusho/verify_world_logic_begin.py --mode wasm --out .local/md-yuyuhakusho/logic-recheck-wasm
python3 -B scripts/yuyuhakusho/verify_world_logic_begin.py --mode vet --out .local/md-yuyuhakusho/logic-recheck-vet
python3 -B scripts/yuyuhakusho/verify_world_logic_begin.py --audit
```

各命令应返回退出码 0。native 与实际 Node／Wasm 包含原世界完整调用和单一初态连续运行；native-race 包含受影响工作、owner、失败／恢复与音频关联范围，完整世界大矩阵由前两项验证。旧 PCM、完整混音器、VDP、声音 IRQ、采样返回及跨帧包装继续关联回归。其他游戏及未修改的浏览器网络矩阵未重复运行。最终计数、耗时、名称集合和日志由 validation.json 绑定。

| 验证范围 | 顶层测试 | 含子项记录 | 结果 |
| --- | ---: | ---: | --- |
| 原生完整关联范围 | 85 | 572 | 通过，27.63 秒 |
| 实际 Node／Wasm 完整关联范围 | 85 | 572 | 通过，172.99 秒 |
| 原生竞态重点范围 | 83 | 566 | 通过，188.21 秒 |

原生与 Wasm 完整范围的通过名称集合一致，竞态范围为其明确子集，均未跳过测试。全部本游戏包的编译及测试链接也单独检查；这项不冒充全部行为测试。

原生与离线导出使用现有固定 ROM／核心及新工作区目录：

```sh
python3 -B scripts/yuyuhakusho/capture_world_frame_work.py --out .local/md-yuyuhakusho/frame-work-recapture
python3 -B scripts/yuyuhakusho/capture_world_frame_prefix.py --out .local/md-yuyuhakusho/logic-begin-recapture
python3 -B scripts/yuyuhakusho/extract_world_logic_begin.py --capture .local/md-yuyuhakusho/logic-begin-recapture/native-frame-prefix.json.gz --out .local/md-yuyuhakusho/logic-begin-reexport
```

## 剩余集成

继续计价 125A 对象准备、3B8C 场景阶段、3592 对象扫描、各角色／碰撞与其他场景工作，并接入实际图形传输生产者及外层 IRQ 尾部。给组件新增任意 work yield 前，须消除旧 SoundQueue／Palette 等视图回写，并在真实写入位置发布 IRQ 可读的滚动值。

还须把新主时钟内的 level4 采样／声音调用接到同一 service，而不能沿用旧跨帧相位或重新获得预算。大于当前未结算参考帧的 DMA deadline、其他视频模式初始化仍明确未完成。完成联合主状态后，再接平台 PCM 流、加载和无损资源发布，最后验收完整浏览器比赛。试玩服务继续保留当前构建；此前未复现的 Go/Wasm heap 异常未宣称修复。
