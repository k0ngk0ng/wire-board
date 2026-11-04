# 全角色 AI 验证

目标是原入口 `0x1cc8a` 的完整返回，覆盖 `0x1cd3e` 的 11 个来源角色决策分支，共用目标筛选、反应和 17 种行为只保留一份独立实现。原 `StepChuAI` API 及其自然、边界、连续夹具须保持通过。该范围不包括攻击动作执行、世界调度、平台托管或完整对局。

## 实现前列出的失败场景

来源角色编号与分派槽不一致；只恢复角色决策入口而未运行到外层返回；错误共享随机门槛或消耗次数；能量、生命、坐标差和比较的 word 回绕／有符号条件不一致；朝向 bit0 与任意非零混淆；低 byte 输入赋值误清扩展字节；把 XOR 变化与按下沿混用；请求号或按钮优先级错误；来源 5 的共用反应特殊分支遗漏；追踪目标寄存器被队伍 byte 覆盖的原行为被理想化；私有 A5 正 24-bit 总线别名被当作原调度的符号扩展指针；错误原子回滚或改写只读世界；用角色 9 自然记录宣称全角色自然覆盖；逐调用回灌 AI／RNG 结果冒充连续策略；把原 CPU 隔离调用等同于可玩角色或完整对局。

先取得原 CPU 完整自然调用与明确私有输入的全角色边界／单初态流，编写新测试并实际观察缺实现失败，再修改生产代码。每步比较来源人物完整 256 bytes、四人物及子槽的 4,096 bytes 和共享 RNG；只允许声明的目标位置／外部输入变化，不注入期望决策或期望中间状态。静态证据来自直接 Ghidra；当前 REA 不支持 raw 68000，不能把它称作 REA 已反编译该 ROM。

## API 与共享实现

`StepFighterAI` 接受来源角色 ID 0–10。`FighterAIActor`、`FighterAIFighter` 和 `FighterAIState` 是原 Chu 数据类型的别名；目标筛选、反应、17 种共用行为与 RNG 驱动只保留一份实现。各来源角色只提供原版决策尾部，没有新增启发式策略，也没有指令虚拟机。

`StepChuAI` 继续严格限定来源角色 9，并使用原 Chu 决策尾部。全角色夹具中每个来源 9 返回均与 `StepChuAI` 再比较；原有三个 Chu 测试也在本次原生竞态和 Node/Wasm 中重跑通过。旧 [chu-ai-validation.json](testdata/chu-ai-validation.json) 记录的是先前源码版本，当前共享源码与回归结果由 [fighter-ai-validation.json](testdata/fighter-ai-validation.json) 绑定。

`ChildAttackValue` 是目标人物 `+0x100` 子槽的 `+0x65` byte。来源 10 在其两次角色随机门槛之前读取它：子槽 X 距离不超过 80、该值至少 48、子槽 Y 非零且子槽地面与自身地面一致时，直接选择 A／请求 1 并转向。来源 9 不读取这个字段。

## 原版证据与结果

[capture_fighter_ai.py](../../../scripts/yuyuhakusho/capture_fighter_ai.py) 使用固定 Genesis Plus GX 内核与 ROM。被动观察的 3,600 帧 RAM／视频摘要逐帧匹配基线。采样等待 `0x1cc8a` 完整返回，并核对返回栈位置；私有调用使用原调度的符号扩展 A5 和独立栈。私有调用同时检查来源人物、RNG 和声明栈范围之外的 RAM 未改写。

| 证据 | 记录数 | 覆盖范围 |
| --- | ---: | --- |
| 自然完整调用 | 1,794 | 来源角色 9 |
| 明确私有输入的完整调用 | 37,056 | 全部 11 个来源角色、17 种共用行为、全部角色入口与共用决策叶节点 |
| 单初态流 | 33 条／792 步 | 每个来源角色 3 条；每步只改变声明目标的 X |

没有未完成调用、采样错误或越界 RAM 改写。隔离调用每次随机消耗为 0–7 次。测试逐步比较完整人物、四人物及子槽、RNG，并检查只读可见世界未被修改。压缩文件和解压后的完整 JSON 均按 SHA-256 绑定。

实际缺实现失败记录在 [fighter-ai-red.jsonl](testdata/fighter-ai-red.jsonl)：当时缺少 `FighterAIState`、`StepFighterAI` 和 `ChildAttackValue`。随后实现通过六项顶层测试，包括原有三个 Chu 回归：原生竞态用时 85.765 秒，Node/Wasm 用时 20.685 秒。测试日志、夹具、来源摘要和限制见 [fighter-ai-validation.json](testdata/fighter-ai-validation.json)。

## 保留的原版细节

- 来源 1 的 `0x1d55e`／`0x1d562` 确实是 `BTST` 后的 `BLT`。完整入口上下文与 112 次原 CPU CCR 观察均表明 D1 高 word 为零且 N／V 为零，因此不跳转；条件成立时仍消耗额外随机数。
- 来源 2 的 `0x1d75c`／`0x1d776` 只设置基础输入和请求 1，直接返回，不执行朝向修正。
- 来源 5 的共用反应在普通反应门槛通过且目标帧标志 bit4 有效后，针对攻击标志 bit2／bit3 额外取一次随机数；命中时分别写中立／下方向输入及请求 1。较早的飞行物防御分支保持普通行为。
- 来源 8 的原版连续比较两次常量 2；对应随机值 3 不选择臆造的第四种动作。
- 向前攻击以朝向任意非零判断，朝向蹲 A 叶节点只检查 bit0。末尾转向只改 `Held`；基础 byte 写入保留扩展 byte。

角色决策入口与 1,394 条可达指令见 [fighter-ai-decisions.txt](../../../docs/research/md-yuyuhakusho/static/fighter-ai-decisions.txt)。这些静态记录来自直接 Ghidra，不代表原 CPU 的自然执行覆盖。

## 复跑

从本 worktree 根目录执行。重新采样需要按[研究工具说明](../../../scripts/yuyuhakusho/README.md)准备固定 ROM、内核与观察器；使用已保存夹具运行 Go 测试不需要 ROM 或模拟器。

```sh
mkdir -p .local/md-yuyuhakusho/tmp
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter_ai.py \
  --out .local/md-yuyuhakusho/fighter-ai-replay
```

聚焦测试使用本次已验证的七个源码文件。缓存、临时文件与重跑日志均写入 worktree。

```sh
task_root="$PWD"
(
  cd games/yuyuhakusho/engine
  GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
  GOTMPDIR="$task_root/.local/md-yuyuhakusho/tmp" \
  go test -race -count=1 -run '^(TestFighterAI|TestChuAI)' -json \
    input.go fighter_input.go random.go chu_ai.go fighter_ai.go \
    chu_ai_test.go fighter_ai_test.go \
    > "$task_root/.local/md-yuyuhakusho/fighter-ai-native-replay.jsonl"
)
(
  cd games/yuyuhakusho/engine
  GOOS=js GOARCH=wasm GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
  GOTMPDIR="$task_root/.local/md-yuyuhakusho/tmp" \
  go test -count=1 -run '^(TestFighterAI|TestChuAI)' -json \
    -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
    input.go fighter_input.go random.go chu_ai.go fighter_ai.go \
    chu_ai_test.go fighter_ai_test.go \
    > "$task_root/.local/md-yuyuhakusho/fighter-ai-wasm-replay.jsonl"
)
```

本阶段只验证 AI 函数返回。攻击执行、人物运动、碰撞、世界调度、平台托管与完整对局仍须由集成阶段验证；来源 0–8 和 10 没有自然完整对局证据。父阶段仍需运行完整包集成门禁。
