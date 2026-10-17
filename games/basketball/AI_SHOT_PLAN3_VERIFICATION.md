# 原版投篮计划 3

## 实现前失败场景

- 固定cold第4649次更新进入mode6计划3；必须执行B471，不替换抽签或跳过模式。
- 先读取对方低篮筐的MountFlags：席位0/1读index1，席位2/3读index0。检查整个字节是否为0，不能只看某一bit，也不能读自己篮筐。
- MountFlag为0时仅把parameter改2，不在本次执行计划2；这条原分支不读取JumpGrade，不能因缺少未使用的跳跃数据报错。
- MountFlag非0且grade>=3时请求DribbleRun；zone&0x70低于60朝对方，高于60反向，等于60时HeadingNone、mode18／nextPlan17；其他字段保留。
- 不把低位Depth区块用于横向门槛。保持原动作进入位：本来已在DribbleRun时不重新进入动作。
- 使用现有JumpVelocity的严格原表换算；MountFlags为当前动作世界的只读引用，不新增永久复制或配置字段。缺失需要的上下文应原子报错。
- 先做原B471隔离执行矩阵与自然采样，再实现；隔离原函数证据与自然调用分开统计。

## 证据与验收

B471 完整原函数见 REA `ev_8bac8418e07eaa8a98c2ac6472d25618bfc7cb159ee632963fe08b31581cafda`（shot-planning/dispatch/batch.json）。原B4B5索引为[1,1,0,0]；B37B方向为[Right,Right,Left,Left]，B37F相反。此表及原条件均保留。

只读18,000帧参考捕获到1次完整自然B471调用，完整WorldInputState精确匹配，全部RAM/PNG与参考相同。另执行原B471隔离矩阵5120组，覆盖4席位、8grade、5种mount字节、8zone、动作及Entered边界，全部与Go匹配；矩阵不计入自然覆盖。

测试先于实现。当前MountFlags经AIShotContext只读引用世界唯一字段，兼容单人物入口可显式传入；没有新增永久config。原生竞态、Wasm、旧计划2/7及283规划调用、1998/57连续世界回归通过，日志位于 `.local/basketball/rea/shot-plan3/`。此修复把裸cold推进5081的mode22边界；后续见AI_COMMAND_RESUME_VERIFICATION与COLD_RUNTIME_VERIFICATION。

```sh
PYTHONDONTWRITEBYTECODE=1 .local/monopoly/venv/bin/python scripts/basketball/capture_ai_shot_plan2.py --plan 3
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_shot_plan3_function.py
source scripts/env.sh
go test ./games/basketball/engine -run 'TestOriginalShotPlanThree|TestShotPlanThree' -count=1
```
