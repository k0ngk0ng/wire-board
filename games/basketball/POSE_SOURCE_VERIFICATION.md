# 共享姿态源验证

恢复 `$056A` 前排除以下失败场景：

- 用当前持球人推导旧姿态源，导致自由飞行期间遗失上次写入。
- 运球／持球只在动作首帧写入；传球反而每帧覆盖旧值。
- 持球举球姿态的高位标记未在 `$EC7B` 清除。
- 篮筐携带、投掷、击飞／抛起混淆写入时机或读取释放后的所有者。
- 无关人物、道具、接触步骤把共享值重置为零。
- 更新原版夹具时修改已有动作、位置、随机数或时钟，掩盖轨迹偏差。

原版函数证据保留在 `.local/basketball/rea/pose-source/analysis/`。夹具增加字段必须来自原始 RAM 采样的 `$056A`，不能用 Go 输出补全预期。

## 当前恢复范围

`WorldActionState.PoseSource` 保存原共享状态，不从当前持球人推导。运球、持球每次更新写入持球人；传球只在发射入口写入。持球举球姿态的临时高位经 `$EC7B` 清除，阶段结束后的状态保留清除结果。篮筐携带每次写入所有者，得分动作仅在所有者小于 4 时写入，投掷只在释放前写入一次。其他已恢复动作及接触阶段保持旧值。

REA 批次证据为 `ev_83eb8e40e7f1fc57f720bf11e1cf022f07b914951cdf9616b092fa8c2fdb6a89`，包含 `$B53E`、`$B585`、`$B5F4`、`$DEE7`、`$DF06`、`$DF39`、`$EC7B` 的反编译结果。直接指令核验补充 `$DE28` 的持有人条件，区分 `$DF39` 的抛起动作 7 与 `$DF76` 的击飞动作 8；后者没有该字段写入。

原 `$9E5B` 的 `$3E` 抛起分支已恢复，见 `LAUNCHED_PLAYER_VERIFICATION.md`；同入口的 `$3F` 分支尚未恢复。辅助物动作 `$B074/$B182/$B1B3/$B446/$B471` 和篮筐抛起动作 `$DF44` 仍由现有处理器明确拒绝。本次没有仅补写共享值来伪装这些动作已经实现。

## 验证结果与复跑

先新增共享源测试并记录编译失败，再增加字段并确认 6 个行为用例失败，最后恢复写入。针对性测试覆盖运球／持球首入和连续更新、举球标记清除、传球首入及后续自由飞行保留旧值。

原版夹具的 1,998 次动作阶段、1,998 次输入阶段、1,997 次准备阶段均通过；两份只初始化一次的连续夹具覆盖 1,998 次正常更新和 57 次主动输入更新。7 份阶段／连续夹具从原始记录增加 `poseSource`，两份连续夹具同时包含 AI 调查新增的索引 88 初始配置；移除这些新增字段后，所有原有字段与修改前完全相同。其他历史函数级 AI 夹具尚未增加该字段，不能据此声称其验证了共享源状态。

```sh
source scripts/env.sh
go test -v ./games/basketball/engine -run 'Test(WorldBallPoseSourceWrites|WorldActions|WorldInputs|WorldPreparation|WorldLoop)'
go test -race ./games/basketball/engine -run 'Test(WorldBallPoseSourceWrites|WorldActions|WorldInputs|WorldPreparation)'
```

证据位于 `.local/basketball/rea/pose-source/`：`tests-behavior-before.log`、`corpus-before.log`、`world-phases-after.log`、`fixture-migration-audit.json`、`write-audit.json` 和 `target-race.json`。原始轨迹的预期值来自采样 RAM，没有使用 Go 输出补写。
