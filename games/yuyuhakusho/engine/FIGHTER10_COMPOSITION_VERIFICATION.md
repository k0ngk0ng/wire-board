# 来源 10 完整动作、输入与人物组合：实现前验证范围

取证边界为完整 `15a18→15b82` 动作、完整 `15540` 输入返回，以及 `12682→126ba` 人物规则阶段。真实菜单输入使用 `fighter10-inputs.json`，真人来源 10 对来源 9 电脑；私有调用另覆盖来源 10 电脑。只新增采集器、夹具与先行测试，不在此任务中编写生产共享化实现。

失败场景：仅替换角色标签却沿用来源 9 的参数／动画／形状表；硬编码来源 9 命令、选择器或 AI；新增五槽与公共槽组合次序错误；命中停顿期间跳过命令／输入准备或重复时钟；动作尾段阶段位在前一选择后变化却仍用旧值；能量只保留高 word 或 `+48` 高 byte 变化破坏低 24 bits；世界 RNG 按人物重置、漏掉一次动作随机尾段或多消耗 AI 随机数；A5 的正 24-bit 别名与原 canonical 指针混淆；完整输入漏掉 AI 后录制、播放结束／停止、控制器按下沿和共享控制状态；AI 可见目标子槽 `+65` 漏解码而错过来源 10 特有威胁反应；暂停／慢动作仍运行输入动作或漏掉继续执行的登记、边界与 HUD；晚期动画库、录制空间、HUD 或缓冲错误泄露前置修改；自然中断声音清理无证据地排除整队列或回灌期望值；单初态流每步注入期望状态；把本范围误称完整人物初始化、精灵／子对象／碰撞调度或世界完整对局。

先从真实来源 10 初始化保存参数、父动画和形状指针，再取得原 CPU 自然、明确私有输入及单初态流。自然过程只观测，未观察原内核与观察内核逐帧核对完整 RAM／视频摘要。私有栈放在 `FFFF00` 下，避免 `FFE0` 统计别名；每次调用核对完整返回 SP 与全部 64 KiB RAM 的允许写入范围。复用旧 snapshot 与 Go 原字节适配器，在本文件的独立 wrapper 中补齐 `ChildAttackValue`，不改旧来源 9 适配器。

先行 API 约定：`StepFighterAction(ChuActionState, uint8, ChuActionParameters, CommandLibrary, AnimationLibrary)`、`StepFighterInput(ChuInputState)`、`StepFighterActor(ChuActorState, uint8, ChuActorView, CommandLibrary, AnimationLibrary)`。三个新 API 的真实缺实现构建失败后通知生产共享化任务；本任务不得提供临时桩或解释原 CPU 的运行时。

## 完整原 CPU 边界

同一次原人物规则回调同时保存外层人物、内层完整输入和内层完整动作，不把三个计数相加当作三倍独立玩法覆盖。输入在 `1269a` 的真实返回处检查 SP 等于入口 SP＋4；动作在 `15b82` 的 RTS 前检查原入口 SP 已恢复；人物在 `126ba`、硬件精灵提交之前检查 SP 与原入口相同。另有直接完整动作和直接完整输入私有调用；直接输入使用声明的返回哨兵 `3d8`。

| 证据 | 完整动作 | 完整输入 | 人物规则 |
| --- | ---: | ---: | ---: |
| 真人来源 10 自然流 | 2,065 | 2,065 | 2,065 |
| 明确私有上下文 | 175 | 52 | 64 |
| 单初态连续流 | 6 条、480 步 | 3 条、48 步 | 4 条、96 步 |

自然流复用已经保存的未观察原内核完整 3,600 帧基线，核对其 ROM／内核身份、输入 SHA-256、原始帧摘要文件及 manifest，然后从冷启动重新运行当前观察内核。全部 RAM／视频摘要一致。来源 10 的自然输入为真人和开局禁用路径，没有来源 10 的自然 AI；私有输入实际运行来源 10 AI，不能把它写成自然覆盖。

参数绑定来自真实来源 10 初始化：动画集合 `+50`、形状集合 `+8c` 和参数指针 `+90` 同时从实际初始化人物复制到四个声明私有身份，三个资源指针及来源人物摘要保存在每份 manifest 的 `binding`。参数直接从该 ROM 资源读取；没有使用来源 9 的表或人工参数块冒充来源 10 初始化。四个私有 A5 分别使用 `FFFF8400/FFFF8800/FFFF8C00/FFFF9000`，全部按原符号扩展身份运行。

## 先行测试与独立采集

全部六项顶层组合测试在生产共享化之前写完，真实构建失败仅缺 `StepFighterAction`、`StepFighterInput` 和 `StepFighterActor`。初始测试文件 SHA-256 为 `a2cab7998fabdc560562ad006f3df139718a1eddb8996bb4dc9578637094abcf`，最终保持相同内容。按并行任务约定，CPU 夹具随后由独立原内核采集，采集器不运行 Go 规则。源码、缺实现失败和最终日志分别绑定，不能把后采夹具描述成先于测试取得。

测试复用旧来源 9 的完整原字节编码／解码器，在独立 wrapper 中从各人物的 `+100+65` 填入 `ChildAttackValue`。原私有输入实际执行 `1e3fe` 威胁响应分支 24 次，覆盖全部四个人物身份；对应 AI 输入多消耗 2 次 RNG，人物组合再加动作的恰好 1 次 RNG。其他 AI 路径有其实际随机计数，暂停／慢动作人物为 0 次。所有语义状态、完整人物／家族、原 FX／形状／投技缓冲、资源小数、`+48` long、共享 RNG、登记、边界样本和 HUD 提交均逐项比较。

完整动作从自身状态推进动画时钟，再由原动画／形状提交选择下一记录；连续流执行完整 `15a18`，使用来源 10 的实际动画时长和终止标志。长的 `2d` 与 `49` 流持续到退出原动作，其他新槽同样检查已退出初始动作。每条流只初始化一次，后续仅接收预先声明的外部按键／控制器和暂停／慢动作输入，不注入期望人物、动画、FX 或 RNG。这里只运行所声明的人物／世界子集，其他人物、子对象、碰撞与 FX 生命周期没有并行运行。

12 个有界代表调用专门连接完整动作尾段与来源 10 选择器：地面／空中投技各真实提交一条 32-byte 记录；请求 1／2／3／6、成本临界、生命 111／112、能量小数和 `+48` 高 byte 清零逐项比较。空中特殊先改阶段掩码后继续读取的顺序同样由整段原输出验证，没有把独立选择器的结果回灌到动作状态。

最终六项顶层测试均通过：native `-race` 25.097 秒，Node/Wasm 6.173 秒。三层自然／私有完整输出、单初态连续组合和三 API 的早期／晚期失败均在同一原始先行测试文件中验证，完整来源绑定见 `testdata/fighter10-composition-validation.json`。

## 输入视图诊断

首次完整验证发现独立 `ai-record` 输入流的外部元数据使用了无关的硬件 0 号控制器帧。该人物通道为 255，原 CPU 入口和出口的输入采样均为零；AI 控制、结果和 RNG 已一致。采集器改为保存该人物真实入口的外部采样后，重新从冷启动完整采集。9 行外部输入元数据改变，全部原 CPU 前后状态、其他字段和其余八组夹具内容保持一致，记录见 `testdata/fighter10-composition-input-view-correction.json`。生产规则与测试断言没有为此改动，测试文件恢复为缺实现阶段的完整原始内容。

## 声音与验收范围

自然动作中有 35 行、自然人物中有 48 行受到独立声音消费者清槽。只有实际观察到 `37c56/37cac` 且地址位于 `F80A..F80D` 才排除该行声音队列相等；全部其他规则字段与未受影响的队列仍完整比较。Go 不接收中断后的期望队列。私有连续流禁用中断，没有声音队列排除或回灌。

本范围止于人物精灵提交之前，不代替初始化、完整精灵绘制、其他对象生命周期、碰撞／世界轮次、资源加载或平台网络验收。原 CPU 私有栈和全 RAM 改写审计只用于取证，生产规则没有原 CPU 状态或指令解释器。静态入口来自既有直接 Ghidra 证据；REA 当前不支持 raw 68000。

## 复跑

从当前 worktree 根目录运行，采集需要已有固定原 ROM／观察内核和来源 10 的未观察基线；Go 验证只读取规范夹具与语义数据：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter10_composition.py \
  --out .local/md-yuyuhakusho/fighter10-composition-replay
mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -run '^TestFighter10Composition' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -run '^TestFighter10Composition' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

最终只清理本任务重复／中间采样和诊断输出，规范的九组夹具／manifest、真实三 API 缺实现红灯、最终 native／Wasm 日志和输入视图修正记录均保留。删除范围与摘要在 `testdata/fighter10-composition-cleanup.json`；共享 Go 缓存、固定 ROM／工具、来源 10 未观察基线和其他任务文件未删除。
