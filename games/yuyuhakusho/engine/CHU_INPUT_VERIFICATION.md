# 酎完整输入调用：实现前验证范围

目标为 `0x15540` 的完整外层返回：禁用、人类输入、AI、播放与录制均在同一调用边界验证。AI 分支调用已恢复的真实 `StepChuAI`；负 C9 的 AI 录制在 AI 返回之后继续 `0x15594`，不能以 `NeedsAI` 前缀或注入期望电脑按键代替完整调用。

可能失败的场景：提前在 AI 边界结束而宣称完整返回；人类／播放输入错误推进 RNG；禁用后仍调用 AI；负 C9 且停止时错误跳过 AI；AI 后忘记录制或先录制后决策；RawHeld／保留计时被清除；扩展 Held／Pressed／Changed 不符合包络清理；Envelope 与 AI 两份 Controls 不一致；录制终止写到错误位置；播放游标或停止标志混用；四 source 槽身份与规范 A5 编码不一致；下游 AI 或录制失败后泄漏前缀状态／共享 RNG／slice 写入；连续流回灌预期中间状态；函数调用计数冒充完整人物或对局。

先保存完整原 CPU 输入／输出及声明私有输入的边界，再写测试并保留缺实现失败，最后编写组合。完整比较 source 人物、世界对象、输入流、终止标记、共享停止状态与 RNG；自然 3,600 帧 RAM／视频必须与同输入无观察器基线一致。输入帧由包络统一持有，AI 接收包络处理后的 Controls，输出后同步回包络；录制或 AI 失败时两个状态整体原子回滚。

## 完整组合与结果

`ChuInputState` 包含 `Envelope FighterInputEnvelopeState` 与 `AI ChuAIState`。`StepChuInput` 先运行包络并把处理后的 Controls 同步给 AI；只有原路线要求时才执行真实 `StepChuAI`，再把输出传回包络。需要录制的路线最后继续 `RecordFighterInputEnvelope`。人类、播放和禁用路线同样同步唯一控制状态，但不运行 AI 或改变共享 RNG／AI 计时。

自然 4,134 次调用全部执行至完整 `0x15540` 返回：禁用 546、人类 1,794、真实 AI 1,794。自然的 1,794 次 AI 已越过旧包络阶段的 `0x156e8` 前缀边界，没有注入期望电脑输入。全部 3,600 帧 RAM／视频仍与原版无观察器基线一致。

| 隔离路线 | 完整调用数 |
| --- | ---: |
| 禁用 | 2,880 |
| 人类 | 512 |
| AI | 64 |
| 人类后录制 | 1,024 |
| AI 后录制 | 128 |
| 播放 | 494 |
| 播放停止／空游标 | 658 |

合计 5,760 个原 CPU 隔离完整调用，覆盖四个 source 槽、C9 正／零／负、八个人类输入通道及 AI、保留窗口、停止、禁用和输入／流边界。实际 AI 执行 192 次，录制续点 1,152 次；其中负 C9 且停止已置位时仍运行 AI，只跳过录制写入。另有 24 条单初态流共 288 步，覆盖四槽的人类、AI、录制与播放，AI 执行 96 次。

每调用完整比较 source 256 bytes、世界对象区 4,096 bytes、输入流 1,024 bytes、四个终止标记、共享停止／禁用值、输入帧和 RNG。source 属于世界区的一部分，两项字节数不作为额外人物计数。隔离期间逐字节核对全 RAM，除 source、录制流、共享 RNG、停止标志、终止别名及实际调用栈外，没有改写。四个 source 的私有 A5 均使用原调度符号扩展编码。

输入终止的地址别名复用已验证包络规则：通道 0～3 修改尾 RAM 标记，4～7 的 ROM 写处理器经实际内核映射核对为 no-op；AI 通道不读取人类 Start 来终止录制。流容量由调用者存储提供，未发明原版固定裁切。测试验证 Envelope 是 Controls 的唯一来源，AI 错误和真实 AI 已运行后的录制存储错误均整体回滚，不修改调用者 slice、两个状态或共享种子。

原生竞态和 Node/Wasm 的三个聚焦测试均通过。真实缺类型／缺函数失败先于实现；当前来源、日志与夹具见 [chu-input-validation.json](testdata/chu-input-validation.json)。静态输入分支复用[原入口证据](../../../docs/research/md-yuyuhakusho/static/chu-input-flow.txt)，其中 AI 体另外绑定已有[完整 AI 验证](CHU_AI_VERIFICATION.md)，没有重复反汇编或复制一份近似策略。

## 复跑与边界

从本 worktree 根目录运行；原版工具按[研究工具说明](../../../scripts/yuyuhakusho/README.md)准备：

```sh
mkdir -p .local/md-yuyuhakusho/tmp
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_chu_input.py \
  --out .local/md-yuyuhakusho/chu-input-replay
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -run '^TestChuInput' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -run '^TestChuInput' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

本阶段实际聚焦测试使用输入、包络、AI 与专属测试的显式源码范围，父阶段仍需运行整包集成。单初态流只改变声明的外部控制帧和敌人 X，不推进人物运动、碰撞或世界时钟，不能作为完整电脑对局证明。其他角色 AI、托管／恢复和外层人物／世界调用仍在各自阶段完成。
