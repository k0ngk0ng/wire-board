# 原版 mode31 与投篮计划 8／9／10 验证

实现前的失败场景：

- Action3E 进入 mode31 后，错误复用 mode6 的初始化，导致计划、计数器或随机数消耗改变。
- 难度阈值边界选择错误；持久化进入状态重复抽取随机数。
- 水平速度符号与左右半场方向比较错误，plan8 没有正确落入 plan9。
- 垂直速度使用有符号比较，将负速度误判为 0；非零速度提前触发跳跃或投篮。
- plan9 没有写入跳跃阶段 -1，或 plan10 在同一次调用提前执行 plan2。
- 下一次调用已有计划 1／2 被覆盖，导致后续跳跃或出手错误。
- 命令应用时破坏朝向、计数器、目标或随机数等无关状态。

以下矩阵只属于隔离的原版函数执行验证，不能代替自然完整对局或宣布复刻完成。

## 实现及证据

- 原版 `AB87` 首次进入 mode31 时写入 Plan17 和 Parameter8，消费一个随机字节并右移一位，按难度阈值选择 8／9／10；同次调用进入 `B383`。已进入模式保留当前计划，不重置计数器、目的地和追踪对象。
- `B51D` 比较横向速度高字节的符号和进攻方向；相同方向且垂直速度高字节等于 0 时进入空中瞄准。方向不同时直接执行 `B540`，不先改写计划号。
- `B540` 在垂直速度高字节等于 0 时启动持球跳跃、写跳跃阶段 -1、下一计划 1。`B557` 在相同条件下只选择下一计划 2，不在本次提前执行。
- 瞬时上下文 `HorizontalSpeed` 来自当前人物横向速度，接入独立人物调用和完整世界输入；不新增存档字段。
- REA 的 mode31 证据位于 `.local/basketball/rea/ai-batch/batch.json`，ID `ev_eef21fb30302a3ec59b8b3dc097be9a7fa2a559339d598963a8489a9d646759d`；计划分派证据位于 `.local/basketball/rea/shot-planning/dispatch/batch.json`，ID `ev_8bac8418e07eaa8a98c2ac6472d25618bfc7cb159ee632963fe08b31581cafda`。静态分析用于定位，结论另以原指令实际执行核验。

## 对照矩阵

`verify_ai_mode31.py` 不修改 ROM 指令，执行 `AB87` 及其真实子调用至返回，共 16,896 条隔离用例：

- 3,456 条首次进入，覆盖四个席位、水平正负方向、垂直 -1／0／1、八个难度组与阈值两侧的随机值。
- 13,440 条持久化调用，覆盖计划 1／2／8／9／10、动作 62／20／26、水平速度高字节 -128／-1／0／127、垂直速度高字节 -128／-1／0／1／2／3／127、跳跃阶段、离地状态与真人队伍。
- 比较动作及进入位、模式及进入位、计划、计数器、目标、目的地、朝向、跳跃阶段、投篮周期、下一动作和完整 RNG。
- 原表只有七个意图明确的阈值对；难度 28–31 会读到紧邻的指令字节 BD／72。实现保留原读取结果，并由矩阵验证，不外推新的阈值。
- 红测先得到 `original AI strategy mode 31 not reconstructed`；实现后全部矩阵通过。红绿日志位于 `.local/basketball/rea/ai-mode31/`，可重建夹具和来源摘要位于 `engine/testdata/ai-mode31-function*`。

复现：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_ai_mode31.py
source scripts/env.sh
go test ./games/basketball/engine -run '^TestAIMode31IsolatedOriginalMatrix$' -count=1 -v
```

原 ROM SHA-256：`0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`。夹具 SHA-256：`c399b2ab5452b44be50caf9a066b16c6b6528f29f588c4e5bb97fa7cea8a3a53`。

上述受控输入仅用于隔离原函数，不修改自然捕获 RAM，也不改变完整比赛的固定种子 `0x12345678`、恢复间隔或 48,000 视频帧上限。mode34 等未恢复分支仍显式报错；该矩阵不代表全部原版 AI、完整赛程或产品复刻已完成。

定向 `-race` 与 Node Wasm 验证均通过，覆盖 mode31、Action3E、强制投篮、已有投篮计划、人物接口与完整世界输入／循环（包括 1,998 个原始连续逻辑帧）。结果分别见 `.local/basketball/rea/ai-mode31/race.log` 和 `wasm.log`。

```sh
go test -race ./games/basketball/engine -run 'Test(AIMode31|OriginalAction3E|AIForcedShot|OriginalAIShotPlan|ShotPlan(Two|Seven)|ComputerActor|World(Inputs|Loop))' -count=1 -v
GOOS=js GOARCH=wasm go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run 'Test(AIMode31|OriginalAction3E|AIForcedShot|OriginalAIShotPlan|ShotPlan(Two|Seven)|ComputerActor|World(Inputs|Loop))' -count=1 -v
```
