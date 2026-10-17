# 特殊球 17 恢复与验收

本轮先记录失败场景，再生成原指令夹具、编写测试与实现。目标是原动作 17（bank 6 `$BBFD`），不是动作 19（`$BBB9`）。两者分派由 `$B4FC` 表核定。

## 先验失败场景

- 首入遗漏音效 30、选筐、原竖直冲量叠加或左右方向 6 像素水平速度；错误地清空已有纵深速度，或错误调用普通投篮发射／清除特殊球标记。
- 12 次动画重复、四帧镜像或切换至动作 12 的时机错误；刷新恢复时重新初始化。
- 落地后未清水平／纵深速度，或误清竖直速度与重力；空中错误停止移动。
- 场地运动发生在入场写回之前，或动作尾段覆盖场地已写入的共享选筐、球权等状态。
- 非 17 动作或非法动画游标被静默接受；错误过程中提交部分状态。

验证范围以原指令独立调用、连续链、实际世界接线及 Native／Wasm 结果分别记录；独立夹具不代表原菜单自然整局。

## 结果与边界

动作 17 的 60 次原调用、6 条连续动作链共 588 次更新均通过。完整世界状态、音效和逐步 JSON 恢复一致；动画先于场地运动切换动作 12，落地只停止水平和纵深运动。REA 证据为 `ev_e47ea2aa36f04bb3886c8cbafecef0d596b4e1aa383caf0e93d30c19a9c82777`。

新增动作 16／17／19／22 的世界适配合计通过 1,586 次原调用对照，并通过 16 个原版完整动作调度阶段的状态、调用顺序和音效对照。Native、race、Wasm 均通过；源夹具重复生成哈希一致。所有这些都是受控原状态，不能据此声称原菜单自然整局或全部组合已经通过。

可复查入口为 `.local/basketball/rea/ball17/result.json`。其中保留 tests-first 编译失败、首次冲量误差、调度采样点修正前后日志；调度夹具保留原角色动作／游标配对，避免把组件探测的投篮动作硬套到完整世界。

```sh
source scripts/env.sh
python3 -B scripts/basketball/verify_ball17.py
python3 -B scripts/basketball/verify_recovered_special_world.py
go test ./games/basketball/engine -run '^(TestRecoveredSpecialBall|TestOriginalBall17)' -count=1
go test -race ./games/basketball/engine -run '^(TestRecoveredSpecialBall|TestOriginalBall17)' -count=1
GOOS=js GOARCH=wasm go test -exec="$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run '^(TestRecoveredSpecialBall|TestOriginalBall17)' -count=1
```

球队轮换的 16 组完整比赛首轮为 11 组通过、5 组失败。通过配置各重复完成两场，正常时钟、固定种子、恢复及摘要检查不变；失败输入和日志位于 `.local/basketball/roster-rotation-special-balls-20261018/`。其中 3 组呈现已知悬空球形态，另 2 组为持球归属和附着人物动作首差；未宣称全量通过。

后续修复受击支撑解绑后，case 11 在 Native 和 Wasm 各完成两场，该批现在有 12 个不同配置通过。case 4 恢复原自引用运球后仍卡局；原整机从同一真实前态连续 864 个世界与 Go 一致，不将该配置计为通过。源状态验证报告分别见 `HURT_RIDER_VERIFICATION.md` 和 `SELF_DRIBBLE_VERIFICATION.md`。

最终接线另跑旧 1,998 个原动作阶段，Native／race／Wasm 均通过。新阵容浏览器测试通过 42 项，但该整场只观察到球动作 0–11，因此仅证明该阵容的整场、原输入与呈现不回归，未计为新特殊球 16／17／19／22 的画面验收。浏览器记录见 `.local/basketball/all-computer-full-match-browser-teams-0-1-characters-2-3-0-1/`。
