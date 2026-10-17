# 统一视频／逻辑运行驱动

实现前失败场景：视频帧等于逻辑更新；视频调用与调用方手动StepLogic导致双推进；开球时跳过原33次更新；重复消费主循环RNG；暂停进入当次world漏跑或恢复当次world多跑；暂停期间时钟增加；短按在平台视频边界丢失或被重复消费；只保存World而漏掉输入边沿/时钟/等待/开局阶段；多个活动phase或全nil的恢复状态被接受；中场比分丢失；结束阶段没恢复却静默前进。

原CA54..CAF9：场内Scene1等待计数阈值CB07为2，再等下一中断沿，因此常态最少3个视频中断一次逻辑更新。FF80是控制器读取入口，不是视频计时器。原硬件只在逻辑入口采样；平台StepVideo会额外锁存按下边沿，避免手机/键盘的短按丢失。StepLogic允许显式重放原采样，不自动产生视频tick。

显示DMA/VRAM的CPU执行耗时不通过解释器仿真。场内逻辑和显式等待分离；已知忙等待转换为原CPU时间，显示层负责淡入/背景提交。不能把这个视频调度器声称为周期精确NES执行。

## 接口与状态

- `NewRuntime(RuntimeConfig)`：从配置和种子进入真实场地／裁判开球流程。
- `StepVideo(state, held [4]Buttons)`：浏览器每个视频 tick 调用一次；到期会自动执行逻辑。调用方不得再额外调用 `StepLogic`。
- `StepLogic(state, input [4]ControllerFrame)`：独立验证／重放入口，不推进视频；显式显示等待尚未结束时拒绝直接调用。
- `ValidateRuntimeState(state)`：只读校验，不在恢复时修补或重置状态。
- `ClearRuntimeButtons(state, controller, mask)`：明确取消某设备／手势独占的逻辑位；只清对应 `Held` 和未消费的 `PendingPressed`。`ClearRuntimeInput` 是 mask 255 的兼容包装。

输入数组对应原控制器 0～3，驱动再按配置映射至人物席位。控制器 0 的 Start 控制暂停／恢复；原 FF80 采样边界仍可由 `StepLogic` 精确提供。平台的 `StepVideo` 缓存短按，并在消费时补成一次 `Held|Pressed` 合法采样；物理 Held 另行保留，避免第二次短按被吞。失焦、页面隐藏、设备断开及 pointercancel 必须显式取消对应输入，不能用普通松开替代取消；保存／恢复本身不隐式清按键。

`RuntimeState` 只允许 Opening、Playing、Transition 三个阶段指针中的一个非 nil。时间、逻辑计数、等待、待消费输入、显示缓存全部序列化。新半场需要保留原 reset 没有清掉的部分输入／AI／状态字段，因此跨节使用冻结的 `Carry`；交接至新 playing 后即丢弃，不并行推进两份世界。

配置校验覆盖队伍、人物、场地、装备、控制器、战术、活动阶段一致性、能力配置、逻辑计数、十进制时钟、投影及显示列历史。计数限制在 JSON 安全整数范围。最终结果使用 `RuntimeState.Outcome`，记录原规则选出的赢家与实际比分。`Confirmed` 仅在结果画面完成最后四名角色更新后设置；后续尚未恢复的奖励／菜单场景明确返回错误。

## 调度与跨节

正常场内模式按 3 个视频 tick 推进一步，暂停仍经过相同逻辑采样及主循环 RNG，但只更新投影；暂停进入当步仍完成原 world，恢复当步也不多跑 world。中场的原 Scene 2 阈值为 0，对应每个视频 tick 可处理一次选择输入；等待 Start 确认既有配置，不自动开始下一节。

开节在同一次逻辑更新中执行 `InitializeOpeningPeriod` 后直接进入第一轮人物输入／动作；不会额外消费 RNG 或先插入前置碰撞。后续 playing 由 `StepWorldLoop` 负责其原主循环边界。HUD 使用 world 提供的 D326 快照，而不是最终动作后的分数；结束检查仍早于本轮 HUD 更新。

阶段 5 的原忙等待按 717,316 CPU 周期、NTSC 每视频 29,780.5 CPU 周期转换为明确等待，不解释 CPU。原背景 VRAM 装载时间由已准备的 Canvas 素材替代；淡入／淡出保留 8 个视频 tick 的等待。NES 渲染／中断执行开销不逐周期模拟，因此本运行器不声称每次初始化／忙等待的视频帧数与硬件完全相同；场内规则与动作更新数量保持原接口。

到阶段 4 后按原规则保留比分、递增节数。第一节结束后等待 Start，并通过真实场地／开球流程进入第二节。第二节最终结果 phase 6 已恢复四名角色的庆祝／退场动画、移动与投影，以及缓存比分的显示数据。初次进入结果画面不表示完成；控制器 0 的 A／Start 或原 144 次计时结束后，完成本轮角色更新，再设置 `Outcome.Confirmed` 和原 `NextScene`／`NextPhase`。对战选择、战役结束菜单及 phase 7 奖励后续流程仍未恢复，下一次逻辑调用明确报错，不自动重开。

## 显示与音频

逻辑前提交上一轮画面；视频边界发布提交、真实场地列命令和 HUD 缓存。world 和开球物理都提供实际 `MotionSurfaceWrites`，显示层读取 motion 前的地形缓存，不猜新旧位置差。场地重载调用 `ResetDisplayCourt` 和 `InitializeOriginalDisplayHUD`；第二节的名字、标签和已有比分按原 HUD 程序处理。

冷启动没有可重放的旧 NES 菜单画面，首个逻辑更新前不提交清零的对象／drawOrder。初始化场地后才出现真实空绘制列表和 `(128,297)` 镜头，避免重复对象 0 或幽灵人物；客户端不需要过滤错误状态。

普通 Start 暂停／恢复**没有**原 253／255 声音请求。D35A 与 D3C4/D3EB 的静态指令没有发声，全局 FBCC 的独立 300 帧暂停观测也没有这两个命令；FBE3 音频驱动更新了 300 次，其中暂停的 30 帧也更新 30 次，原 BGM 持续更新。253／255 仅按背景装载等实际 Go 事件交给声音适配器；客户端不能按 phase 自造原声音事件。源记录见 `.local/basketball/engine/runtime-pause-audio.json`，复现脚本 `scripts/basketball/capture_runtime_pause_audio.py`。

## 验证

REA 节奏／控制器证据 `ev_bafdbdc0d57607ca23239718fb492b5d98c00431b1a6fed6e83cfe77b983f63f` 位于 `.local/basketball/rea/runtime/`。Ghidra 对 CAE9 主循环展开有不完整控制流，节奏结论同时核对原 CB06 表 `[0,2,0,0,0,0,0,0]`、CAF1 阈值和 CAF7 等待下一沿的指令。

运行器定向测试覆盖正常视频节奏、短 B／方向输入、暂停／恢复、完整 JSON 恢复、输入手势取消、多控制器隔离、跨节比分保留与再开球、冷显示边界。跨节测试使用明确的规则边界状态触发节结束，不冒称已经由真人完整打完一局。原子失败不改旧快照。原开球、动作、HUD 和结束等组件的自然调用证据仍见各自验证文档。

测试命令：

```sh
source scripts/env.sh
go test -v ./games/basketball/engine -run TestRuntime
go test -race ./games/basketball/engine -run TestRuntime
```

当前七项运行器测试已通过；竞态／Wasm 日志保存在 `.local/basketball/engine/runtime-{race,wasm}.log`，实施前记录包括 `runtime-before.log`、`runtime-cold-display-before.log`、`runtime-short-direction-before.log`、`runtime-cancel-before.log`、`runtime-cancel-buttons-before.log`。并行工作项若尚在测试先行、导致其他测试引用未实现类型，定向构建仅包含生产文件与 `runtime_test.go`，不修改或删掉其他测试。浏览器、音频与 WASM 协议 E2E 由主线程另行验证，不能用这些 Go 定向结果替代。

结果阶段新增完整原版比对、分支确认、每帧 JSON 恢复验收。145 次自然完整结果调用（580 次人物更新）逐字段与声音一致；竞态及 Wasm 日志为 `.local/basketball/engine/result-{race,wasm}.log`，详情见 [结果验证](RESULT_VERIFICATION.md)。结果 `Confirmed` 只表示本场结果呈现结束，不表示奖励或完整战役完成。

## 中场显示边界

浏览器完整对局曾在第一次中场后的 video 10985 报 `Invalid display camera Y: 0`。保存状态显示 `Submitted`／`Visible` 已被 Scene 2 菜单的 `(0,0)` 相机覆盖；Start 后的 Scene 1／phase 0 尚未初始化，也不能作为球场显示源。原 `match-transition` 自然记录 frame 14351 明确显示：节间 reset 从 Scene 1 切到 Scene 2，同时将下一相机从 `(8,297)` 清为 `(0,0)`；这不是球场镜头移动。

`runtimeHasCourtDisplaySource` 根据实际场景和初始化状态控制提交：Scene 2 菜单及未进入的 Scene 1／phase 0 保留最后有效球场提交。D1B5 完成后恢复正常提交，使用已初始化的 `(128,297)` 镜头。显示层不接收菜单坐标，渲染器的合法相机范围保持不变。该规则同时覆盖冷启动和第二节开球。

先行测试 `TestRuntimeIntermissionRetainsCourtUntilInitialized` 复现了零相机提交；修复后验证 60 个视频 tick 的中场等待、逐帧 JSON 恢复、Start 输入边沿、下一节初始化及裁判开球。Native race 和 Wasm 定向测试通过，日志保存在 `.local/basketball/engine/intermission-display-{before,after,race,wasm}.log`。测试通过缩短规则边界触发中场，完整自然浏览器对局由主线程单独复跑，不以此定向结果代替。
