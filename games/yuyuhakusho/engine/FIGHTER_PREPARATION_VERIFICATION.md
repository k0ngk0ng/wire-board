# 共用输入准备与攻击值随机尾段：实现前验证范围

目标为完整 `0x15c36..0x15cae` 共用输入准备，以及完整 `0x15bfe..0x15c0e` 攻击值尾段。前者位于角色命令准备之后，后者位于动画形状提交之后；它们不能互换执行位置或替代完整人物更新。

实现前失败场景：把 `+b8` 高 word 的有符号比较改成长整型或无符号比较；能量小数误影响门槛；把充能计数饱和到 64、65 以外或强行修正旧大值；held／pressed／changed 混用；朝向任意非零与 bit0 混用；扩展 C1 的 bit0／2 优先级颠倒；扩展与基础请求竞争顺序改变；无请求时错误清除旧请求；把 `+72` 当成 `+73`；随机乘法使用高 word、有符号数或错误返回寄存器部分；先截 byte 再做乘法；忘记 byte 加法回绕；攻击值／倍率为零时跳过 RNG；把演员随机源隔离而破坏共享顺序；遗漏未知人物字节或 RNG 写入；观察器改变自然基线；每步注入期望状态；把函数调用计数当成对局。

先核对有界原指令并保存完整自然／原 CPU 隔离输入输出，后写 Go 测试并保留缺实现失败，再恢复独立规则。比较完整人物 256 bytes 与共享随机状态；连续流只修改明确声明的外部输入，不恢复期望计时、请求、随机状态或攻击值。

## 已确认语义

`PrepareFighterInput` 读取保留的 `+71` held、`+72` pressed 和独立扩展 pressed byte `+c1`，不读取原始 held `+74` 或 XOR changed `+73`。能量 `+b8` 的高 word 按有符号数比较；小于 223 且 A／C 任一 held 时，计数 `+c7` 在 0～64 递增到 65，旧值大于 64 则保留。条件不成立时清零。小数部分不影响门槛。

朝向仅检查 bit0。朝向偶数时，扩展 bit0 优先给请求 7，bit2 给请求 8；奇数时两位交换。扩展未选中时，held bit5 与基础 pressed bit4 同时成立也给请求 8。没有触发时保留旧请求，不能清零或用按键变化替代 pressed。

`RandomizeAttackValue` 无条件调用既有 `NextRandom`，即使倍率或当前值为零也推进共享随机状态。使用返回值低 word 与 `+c4` 无符号 word 相乘，取积的高 word 低 byte，加到 `+65` 并保留 byte 回绕。完整有界指令见[静态证据](../../../docs/research/md-yuyuhakusho/static/fighter-preparation-core.txt)。

## 覆盖结果

- 自然完整调用共 8,268 次，输入准备和随机尾段各 4,134 次；全部 3,600 帧 RAM／画面与原版基线一致，无未完成调用或观察错误。
- 9,614 个原 CPU 隔离完整调用，其中输入准备 9,074 个、随机尾段 540 个。覆盖能量高 word 的符号／223 门槛及小数、计数 64／65／旧大值、全部 held／扩展输入 byte、朝向 bit0 优先级、基础 pressed 与 held 组合，以及零／极端 RNG 种子、倍率和 byte 加法回绕。
- 7 条单初态连续流共 896 步，其中输入 3 条、随机 4 条，各 128 步。输入流只改变明确声明的能量、held／pressed／扩展 pressed 和朝向；随机流只改变外部倍率，随机源和攻击值在 Go 内连续推进。

每次比较完整人物 256 bytes 和共享随机状态，不屏蔽未知字节。隔离调用同时检查完整 RAM，只有人物已确认输出、RNG 状态和原调用栈允许变化。真实红阶段因缺少状态类型及函数编译失败，先于实现；日志、来源及夹具摘要见[验证记录](testdata/fighter-preparation-validation.json)。

Go 原生竞态与 Node/Wasm 两项聚焦测试均通过，两次均以完整 engine 包编译；本记录不将聚焦测试结果描述为整个游戏的集成回归。

这证明两个完整小函数及各自连续状态；尚未证明它们与角色命令、动画、共享尾段、碰撞和世界调度组合后的完整人物更新，也不代表完整对局或原版电脑恢复。

## 复跑

从 worktree 根目录运行：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter_preparation.py \
  --out .local/md-yuyuhakusho/fighter-preparation-replay
mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -run '^TestFighterPreparation' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -run '^TestFighterPreparation' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

原版复跑输出的同名夹具及 manifest 可与已保留记录比较。模拟器与 ROM 只用于开发对照，生产运行时使用独立 Go 规则。
