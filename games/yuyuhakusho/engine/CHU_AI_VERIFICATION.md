# 酎原版电脑：实现前验证范围

目标是完整 `0x1cc8a` 返回，包括难度／计时器、候选目标轮转、共用反应判断及来源角色 9 的决策分支。电脑输出交给与真人共用的人物规则和共享世界 RNG；不使用自创追击／随机出招逻辑替代原版。

可能失败的场景：把入口或角色分派定位当作完整策略；漏掉共用反应的早返回；目标槽号、目标位号与平台席位混用；未排除自己、空槽、负生命或同队；改变四槽轮转顺序或最多检查次数；byte／word／long 计时及减法回绕错误；难度表索引和比较有符号性错误；RNG 消耗次数、返回低 word、高 word 残留或乘法位宽错误；朝向／双线／距离条件错误；把保留、按下和变化字段混写；遗漏命令槽、其他对象或全局写入；以预期输出回灌中间状态；观察器改变原版完整帧；隔离决策计数冒充电脑完整对局；未经证据恢复其他角色策略。

先保存静态可达路径与原 CPU 完整输入／输出，再写测试、保留缺实现失败，最后编写独立 Go 规则。自然采样必须与同输入的无观察器原内核 3,600 帧 RAM／画面一致；隔离输入与世界变动明确声明。测试比较人物完整 256 bytes、被引用世界对象及全部共享 RNG／全局变化，不屏蔽未知写入。独立策略不读取 ROM 或模拟器。

## 已恢复路径与接口

`StepChuAI(ChuAIState)` 恢复 `0x1cc8a` 的完整外层返回，包含共用反应 `0x1cd70`、17 种共用行为、来源角色 9 的 `0x1e1e4` 选择及其实际到达的共享尾段。静态记录分为 [入口与反应](../../../docs/research/md-yuyuhakusho/static/chu-ai-core.txt) 和[共用行为／酎选择](../../../docs/research/md-yuyuhakusho/static/chu-ai-decisions.txt)，共 594 条可达指令；未把其他角色的策略扩成已完成范围。当前 REA 不支持 raw 68000，静态记录来自直接 Ghidra，运行语义由固定 MC68000 原 CPU 对照验证。

状态中的 `Fighters[4]` 按原对象 `8400/8800/8c00/9000` 排列，包含占用、队伍、生命、坐标高 word、攻击标志及各自 `+100` 子对象的坐标／标志；这些字段只读。`Actor` 保留当前人物的控制通道、请求、行为、轮转、计时和跟随目标。原难度字直接使用 word 表偏移 `0/2/4/6`。生产代码只使用语义字段和已验证的四张只读表，不读取 ROM、不执行地址或原指令。

电脑与真人共用后续人物规则。输入包络负责进入 AI 之前清除基础与扩展 Held／Pressed／Changed，AI 只按原分支写低 byte，保持 `RawHeld`、保留计数和未写通道；不能把每次 AI 调用当成所有按钮均重新按下。返回的 `RandomSeed` 必须回写共享世界后再交给下一消费者，不能为每个人物单独保存随机序列。

## 完整原 CPU 对照

| 范围 | 数量 | 覆盖边界 |
| --- | ---: | --- |
| 自然完整返回 | 1,794 | 固定 3,600 帧输入；实际 source 为 `9000`，角色 ID 为 9 |
| 原 CPU 隔离完整返回 | 15,468 | 四个 source 对象槽、四种难度、全部 17 行为及 15 个酎共享选择入口 |
| 单初态连续流 | 17 条／408 步 | 仅按声明改变目标 X，不恢复或回灌 AI／RNG 中间状态 |

所有自然帧的完整 RAM／画面基线一致。每调用比较 source 对象全部 256 bytes、`8400..93ff` 世界对象区全部 4,096 bytes、共享 RNG、难度和场地边界；source 位于世界区内，两者不是独立对象计数。隔离调用还逐字节检查完整 RAM 的范围外改写，除 source、RNG 与实际调用栈外均不允许改变，结果没有异常。

矩阵包括空槽／自身／负生命／同队过滤、轮转、子弹威胁、朝向 0／1／3、距离及坐标回绕、跨线跟随、生命／能量有符号边界、计时 `8000` 减一的溢出标志、周期行为 byte 回绕和稀有选择。原 CPU 每调用实际记录到的 RNG 次数为 0、1、2、3、4、5、6、7、9，未把未观察到的 8 次作为已验收路径；独立输出共享种子与原版完整一致。模式 16 在失败目标的队伍读取中会改变下一次选择临时值，已保留该原行为，没有改为理想化顺序搜索。

原生竞态与 Node/Wasm 的三个聚焦测试全部通过。测试与真实缺类型／缺函数失败先于实现；当前来源、完整夹具和测试日志摘要在 [chu-ai-validation.json](testdata/chu-ai-validation.json)。单初态流没有推进人物运动、碰撞或世界调度，不能作为电脑完整对局的验收。

## 私有寄存器编码诊断

初次隔离模板把 A5 写为 `00ff9000`。原调度自然入口实测为 `ffff9000`；两者访问相同的 24-bit 总线地址，但 AI 的 `CMPA.W` 将目标 word 符号扩展后，与完整 32-bit A5 比较。相同 RAM／seed 的两次原 CPU 执行分别得到跟随目标 `8800/8400`，证实最初差异来自私有输入的寄存器编码，非策略函数或 ROM 状态。

最终全部隔离使用原调度的符号扩展 source 编码。Go 策略没有为该诊断差异修改；旧私有输入、两次完整原 CPU 输出和自然 A5 观察保留在 [chu-ai-pointer-diagnostics.json](testdata/chu-ai-pointer-diagnostics.json)。该诊断不是正常游戏状态；独立规则以规范人物对象 ID 表达身份，不接收任意 CPU 总线别名。

## 复跑

从本 worktree 根目录运行；固定 ROM／内核与被动观察器须按[研究工具说明](../../../scripts/yuyuhakusho/README.md)准备：

```sh
mkdir -p .local/md-yuyuhakusho/tmp
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_chu_ai.py \
  --out .local/md-yuyuhakusho/chu-ai-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_chu_ai.py \
  --out .local/md-yuyuhakusho/chu-ai-pointer-replay --pointer-diagnostic
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -run '^TestChuAI' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -run '^TestChuAI' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

本阶段聚焦验证使用 `input.go`、`fighter_input.go`、`random.go`、`chu_ai.go` 与 `chu_ai_test.go` 的显式源码范围，以隔离其他并行阶段的临时缺实现；父阶段仍需运行完整包集成门禁。完整人物／世界调用、电脑与真人混合、托管／恢复及完整对局，连同其他角色策略，尚待后续验收。
