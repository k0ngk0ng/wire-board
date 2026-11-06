# 正常音高转换与 lookahead 成本验收

新增 `PricePitchWork` 与 `PriceDurationProbe` 已恢复 FM／PSG／DAC 的原字段写回和 lookahead 边界。价格使用临时语义输入，不保存第二套音符或时间状态，也不执行原 PC、opcode、ROM 或栈。音符、频率表、采样资源和已有规则均未修改。

同一原程序在既有两份 core 上完成 1,064 个私有 profile、4,256 次原生执行。只读记录前后完整 native 状态／RAM／寄存器相同，两 core 的 11,388 个保留指令边界及结果相同。FM／PSG 各覆盖全部 128 个有效索引和四种时钟 profile，另覆盖 rest、DAC marker 与三 family 的 take／reuse。已有正式音高资源的数值与原结果另行核对；私有 PSG 高索引没有被加入正式目录。

FM 的除法包含 480 个内部 refresh skip，octave 位移包含 120 个。初版 Go 把位移的 28 CPU cycles 合成一次普通成本，导致 120 个原 profile 出现 14 master-cycle 误差。保留原代码及真实 red 后，修正为内部 22 周期、refresh skip、固定六周期的正确顺序；原 fixture 和冻结测试未改。

测试先于实现。最终原生竞态与实际 Node/Wasm 各通过 38 项顶层测试、2,084 条记录，无跳过，含全部旧声音时序回归；静态检查通过。按游戏内部增量范围没有重复运行其他游戏或全仓库行为矩阵。

冻结来源、命令和输出见 [validation.json](validation.json)，原生两 core 与内部刷新审计见 [native-evidence.json](native-evidence.json)。离线审计使用 `scripts/yuyuhakusho/verify_note_clock.py --audit`；复跑使用 `--mode native|wasm|vet --out <新的仓库内目录>`。验证器的来源清单初版误引用上一批 parser 文件，已保留旧脚本及通过日志，修正清单后重新运行最终完整包验证。

已删除本次 native capture 的重复 fixture 和空 session 目录，保留唯一原 oracle；没有更换或修改 core／ROM，没有上传资源或切换平台试玩构建。

这里的音高阶段止于字段写回，probe 止于 duration caller 或 final commit 之前。caller、return、cleanup、modulation／envelope、乐器寻址、参数 body、PCM 发布和独立世界／IRQ 仍须在各自阶段完成。本批没有完成整场音频调度或平台播放接线。
