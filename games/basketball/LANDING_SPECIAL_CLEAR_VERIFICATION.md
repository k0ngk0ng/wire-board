# 落地首入清除特殊投篮标记

原完整世界对照发现人物 1 的 SpecialShot 清零遗漏。初始失败源与修复前 4,531 世界对照均保留在 `.local/basketball/rea/ball-rest-stall/`，没有以新轨迹覆盖旧差异。

## 原写入来源

只读追踪原 `$0554..$0557` 的每次写入，首次差异发生在原 emulator frame 6547，bank 6、X=1、写入前 128、写入后 0。回调时 PC 已前进至 `$8BDA`；实际写入指令是 `$8BD7 STA $0554,X`，属于共享落地函数 `$8BBD` 的首入段。

分派表让动作 `0x06／0x16／0x26／0x36／0x46` 共享该入口。只有未进入的动作清零，已进入的落地帧保留当前标记。目标是当前行动人物的 SpecialShot，不是球主、LastShooter 或无主球标记。

补采样版本与同一起点 reference 的 15,000 视频帧 RAM 和 5 张截图完全相同，证明读取追踪没有改写运行状态。证据 `.local/basketball/rea/special-shot-clear/provenance.json`。

## 实现和验证

`world_landing.go` 提供一个共享入口写入，在 `advanceWorldPlayer` 解析原请求后、调用动画和场地物理前执行。道具落地 `0x26` 原有的物理后清零改为共用入口，避免在动作完成后重新提交旧命令覆盖后续世界写入。其他球和人物动作不改变。

测试先于实现，覆盖：遗漏首帧、错误重复清零、清错人物、清非落地动作、物理前时序及物理后的新标记被旧命令覆盖。原入口 oracle 执行 160 个完整 `$8BBD` 调用，包含五个分派别名、四席位、进入状态及四种标记；其中已恢复的三类落地共有 96 次完整 World 适配调用逐字段一致。未将另外两个别名的首入验证宣称为其完整携筐／携人动作已恢复。

真实 Go video 3152 前态随后推进 180 视频帧，与原整机连续 60 个完整 World 的 Actions 字段逐项相同，并逐帧验证 JSON 恢复结果一致。Native 通过；race／Wasm 的最终结果记录在本文件收尾段及 `.local/basketball/rea/special-shot-clear/`。

另独立重跑 Go 的 4,531 个完整世界，原剩余 SpecialShot 差异全部消失：除明确排除的零页 contact scratch 和渲染投影外，规则语义全部一致，球、TeamAI、输入和人物字段均为 4,531／4,531。新结果为 `aligned-world-comparison-after-clear.json`。

高台边缘悬空球的原缺陷仍按原程序复现；本修复不清球 airborne、不更改重力、不修改时钟、AI 或胜负。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_landing_special_clear.py
source scripts/env.sh
go test -race ./games/basketball/engine -run '^(TestOriginalLandingSpecial|TestOriginalAuxiliary|TestWorldAuxiliary|TestOriginalWorldActions)' -count=1
GOOS=js GOARCH=wasm go test ./games/basketball/engine -run '^(TestOriginalLandingSpecial|TestOriginalAuxiliary|TestWorldAuxiliary|TestOriginalWorldActions)' -count=1 -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/compare_rest_stall_worlds.py --go go-aligned-worlds-after-clear.jsonl --output aligned-world-comparison-after-clear.json
```

最终定向 race 通过（23.404 秒），Wasm 通过（7.297 秒），包含道具携带与原 World 适配相关回归；另单独执行 `TestWorldActionsEntireCapturedCorpus`、未知动作原子性和保存恢复，race 4.403 秒、Wasm 1.114 秒均通过。最初 race／Wasm 因并行 Court6 接线缺少 `CourtOutcomeFlags` 而未编译，原失败日志保留；API 就绪后使用新的 `final-*`、`world-*` 日志复验，没有覆盖失败记录。
