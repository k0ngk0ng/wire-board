# 原扣篮动作 59／5B

实现前失败场景：

- 59／5B 与 58 原分派都指向 `$8E9D`，不得按默认报错归为抱篮筐或改成普通扣篮。
- 59 的两帧分别为镜像姿势 44、持续 3／2 tick；释放球时不下移人物 16 像素。5B 首帧为姿势 51，其余节奏沿用 58。
- 初入定位、主人、篮筐层级及高位标记、坐标小数、耐久、动画边界、目标动作与辅助释放均须按同一原函数执行。
- 入口允许原附着，清除发生在运动前；持续帧不得再扣耐久或重设球位置。
- 球队 7 video 21080 首失败前态须以原配置与种子继续；不能跳过原 59 请求或宣布整场通过。

## 原版证据

复用既有 REA `ev_bea0f79ba87f478cf701e72e25cfa17b2a82dca749f06781243f52a48313e99b` 的 `$8E9D`，补核对 bank 6 `$85FB` 动作分派表与 `$AA82` 动画表。58／59／5B 都指向 `$8E9D`；原 `$8F63` 明确只有 59 跳过下移 16 像素。59 动画为 `$AC3D`，5B 为 `$AC61`，不是重画或换色替代。

`verify_dunk_variants.py` 执行未改 ROM 的 E165→E168，全状态比较 228 次配置化调用，包括四角色、三种动作、六个挂点、高位标记、动画游标边界和 24 种附着入口；另有 12 条连续原扣篮链，共 220 次调用。自然语料原 190 次人物扣篮和 260 次释放篮球继续回归，不把配置矩阵称作自然比赛。

## 结果

实现前原函数和真实 team 7 前态均因未恢复 59 失败，保留 `before.log`。实现后附着入口矩阵进一步发现旧 58 路径在运动后才清支撑，导致沿旧支撑移动；保留 `attachment-before.log`，修正为原 `$8ED6` 在 EEFF 前清除。最终正常包 Native／race／Wasm 验证包含全部新样本、原自然扣篮语料与世界扣篮阶段。

日志位于 `.local/basketball/dunk-variants-20261018/`。真实 team 7 首失败前态（video 21080）连续 120 帧及逐帧 JSON 恢复通过。

复现：

```sh
source scripts/env.sh
.local/board-art-env/bin/python scripts/basketball/verify_dunk_variants.py
go test ./games/basketball/engine -run '^Test(DunkVariants|OriginalDunkAndReleasedBallTrace|DunkPreserves|WorldActionsOriginalDunkStage)' -count=1 -v
go test -race ./games/basketball/engine -run '^Test(DunkVariants|OriginalDunkAndReleasedBallTrace|DunkPreserves|WorldActionsOriginalDunkStage)' -count=1 -v
GOOS=js GOARCH=wasm go test ./games/basketball/engine -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run '^Test(DunkVariants|OriginalDunkAndReleasedBallTrace|DunkPreserves|WorldActionsOriginalDunkStage)' -count=1 -v
```

同一场地 1、球队 `[0,7]`、角色 `[0,1,0,1]`、全原版 AI 与种子 `0x12345678`，两遍完整赛均通过：22,631 视频帧、7,515 逻辑更新、每遍 85 次 JSON 恢复，轨迹 SHA-256 均为 `f8d5876ade681c07765d272d524b59e70ea49fbc1ce3f74cb7b9ad209fb6548f`。源代码前后哈希一致，记录在 `.local/basketball/dunk-variants-team7-20261018/native/`。这是该明确配置的整场证据，不代表全部球队 7 球员组合或整款游戏已经完成。
