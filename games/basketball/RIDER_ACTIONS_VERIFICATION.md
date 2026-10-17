# 骑乘动作恢复

实现前验证失败场景：依附目标编号与本角色混用；未先执行支撑对象就移动骑乘者；原高度偏移丢失小数；支撑者动作变化后仍保持依附；新进入时多推进一次动画；空中状态或速度被错误清空；跳离时依附位未清；持球与无球骑乘共用错误动作组；未知道具支撑被默认为普通人物；发生错误后留下部分对象改动。

先沿用自然输入 `rider-normal-reference`（向下移动、A+B+向右），捕获全部骑乘动作调用。`scripts/basketball/capture_rider_actions.py` 检查插桩后的逐帧 RAM 和截图与原参考一致，不注入骑乘状态。随后以原调用输入／输出验证语义组件，再接入世界调度。该场景覆盖范围以采样实际动作编号为准。

当前恢复 `StepRiderPlayer` 的 `$3D` 动作及世界中的人物支撑适配。11 次自然原调用全部匹配，包含 7 次持球和 1 次脱离；逐项比较骑乘者、父对象回链、球动作、跳跃阶段和完整世界字段。原 `$EEFF` 依附分支只复制整数坐标，保留骑乘者自身小数，不推进物理。高度由父对象类型与姿态决定，不能复用接触阶段的高度阈值。脱离后直接执行原待机处理，再进入下落，不能提前运行一次下落起始冲量。

REA 主动作证据保存在 `.local/basketball/rea/rider-actions/analysis/`；辅助函数 Evidence 为 `ev_4f8d284dfff18d1c755aeaf955362aa9ced948ac9544ee3c04ae5e552e41c261`。针对性 race 与 Wasm 通过，结果为同目录的 `native-tests.json`、`wasm-tests.log`；首次 Wasm 使用错误工作目录的失败保留在 `wasm-wrong-cwd.log`，修正工作目录后通过。自然语料未覆盖 `$40..46` 和道具支撑；世界适配器对未恢复支撑明确返回错误。

新增完整世界主动输入参考 `world-loop-rider.json.gz`，覆盖采样中的全部 57 个完整更新；末尾下一轮已完成输入但动作未结束，其完整原记录及未结束状态仍保留。连续测试先暴露 `$3D` 的接触参数缺口，已从自然接触采样恢复，并由 `export_contact_profiles.py` 生成唯一的接触表，旧 2,392 次接触搜索仍通过。后续两条原 AI 索引别名分支和附着接球也已恢复，完整 57 次更新、746 次对象调用在原生与 Wasm 下均与原版一致。轨迹 SHA-256 为 `9388f2c89b3f1dd0e8f2ab91b1fefb087493241dc6f41e9ad64daf56deb02800`，独立 CLI 结果在 `.local/basketball/engine/continuous-runtime/verification.json`。

另外核对 `$E18B/$E208` 的请求接受表：原允许切换到的未恢复骑乘动作必须明确失败，不能继续站立而吞掉按键。拒绝的请求保留原值。失败和通过证据为 `request-scoped-before.log`、`request-scoped-after.log`；这次针对性编译覆盖全部生产源和请求测试，当时另一工作项正处于循环测试先行阶段。

附着接球 `$1E` 已继续恢复：原 `$91E3` 在进入时将下一动画动作设为骑乘 `$3D`，通过 `$EEFF` 的依附分支跟随父对象，不推进地面物理。5 次自然原调用的角色、球命令及完整世界字段全部匹配，证据在 `receive-ball-after.log`，REA Evidence 为 `ev_f65d12eeb02542f8bf66e5dd4eac9ac049a73b09793acf753629dbedf0c3633a`。骑乘时篮球沿用原偏移组 2。静态高度、动画、请求接受列表和持球偏移由 `extract_rider_tables.py` 保存在 `testdata/rider-tables-source.json`，包含原 ROM 摘要。
