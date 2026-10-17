# 特殊球动作 19

实现前列出的隔离失败场景：首次进入与已进入状态混淆；未先提交共享 latch／持球者／发射元数据；左右发射及无人持球特技位错误；动画末帧循环和翻转错误；物理后读到旧 latch；非负 latch 的反复重定位／停竖速与计数下溢错误；竖速高字节 -8、-7 边界或低字节被错误清除；落地覆盖动作 10 的顺序错误；保存恢复丢失动画、计数及共享状态。源 oracle 和测试先于实现。

范围：原 ROM bank 6 `$BBB9` 动作 19；受控原指令调用和持续运行链。不是自然完整比赛覆盖，不更改原悬空球缺陷，不自行补电脑策略。世界接线由主任务整合。

## 原代码与恢复行为

REA / Ghidra 函数证据为 `ev_71dc8a7b4784ac1e1e5f16eabd4d8c4c3768f2e7024e6e16f84a940c366884f7`，位于 `.local/basketball/rea/ball19/analysis/`。函数边界为 `$BBB9..$BBFC`，辅助函数为 `$BAAB` 和 `$BD14`。静态分析用于确定分支与调用次序；行为对照来自未修改的 ROM 指令执行。

首次进入时使用原投篮发射计算，发出音效 30，写共享接触标记 128 和计数 6，释放持球者。动画按原 `$BEF4` 表循环显示帧 17、18、19，每帧持续 2 次更新。物理完成后重新读取共享接触标记；标记非负时调用原特殊球下落重定位，再停止竖直运动并递减计数，计数下溢后进入动作 10。负竖速限幅只写高字节，保留原小数部分。最后的落地分支优先于计数分支，进入动作 6。

组件采用准备／完成两阶段。准备产生的持球者、发射数据、接触标记及音效须在物理前提交；完成阶段通过回调读取物理后的接触标记，不能重复写回准备阶段的数据。

## 验证结果

`verify_ball19.py` 通过原 `$E149..$E14C` 球分派执行 816 次受控调用，覆盖左右方向、首入／已进入、标记正负、计数边界、竖速边界、动画末帧与特技发射。另有 8 条持续链，共 258 次更新，覆盖正常落地和计数转动作 10 后落地。链内未重新注入状态。Go 每步 JSON 保存恢复后继续运行，并对比原世界字段与音效序列。

Native、race、Wasm 定向检查均通过，结果保存在 `.local/basketball/rea/ball19/{native,race,wasm}.log`。实现前失败记录为 `before.log`；其他任务同时处于测试先行阶段时的编译错误仍保留。ROM、基础 RAM 和压缩夹具 SHA-256 见 `engine/testdata/ball19-source.json`。

```sh
source scripts/env.sh
python3 scripts/basketball/verify_ball19.py
go test ./games/basketball/engine -run TestOriginalBall19 -count=1
go test -race ./games/basketball/engine -run TestOriginalBall19 -count=1
GOOS=js GOARCH=wasm go test ./games/basketball/engine -run TestOriginalBall19 -count=1 -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
```

这些结果证明组件在声明的原调用范围内一致，不代表全部场地设备、角色组合、网络对局或完整比赛已通过。当前原指令夹具使用纽约球场背景状态；其他场地的世界接线与自然可达路径须另行验收。
