# 暂停／恢复固定调用时序验收

新增 `scoreclock.StartPause`、`ResumePause` 已连接实际声音 bus、canonical YM／PSG leaf 和必要的 pending 音量。暂停按源顺序捕获、静音 FM 与 PSG；继续只恢复 FM，PSG 由后续音序驱动。调用方仍须提供真实 gate 后时钟和 detached owners，并负责世界／IRQ 的后续调度。

本批原生竞态与实际 Node/Wasm 各通过 32 项顶层测试、866 条记录，均无跳过：其中新增暂停组为 4 项／6 条，其余 28 项／860 条为旧声音时序回归。静态检查及全仓库编译／测试链接通过；未运行全仓库行为测试。

- 已有自然对局的暂停 255 次、恢复 244 次 Main 访问，在给定原观察 bus／mailbox／status 反馈下，时刻、访问顺序和有效 TL 端口值全部一致。硬件忽略的 TL bit7 原值继续保存在来源中，未伪称 raw-byte 全等。
- 实际 mixed Runtime 验证了真实 bus hold 后捕获 24 个有效 TL、FM／PSG 静音、JSON 保存恢复、FM 继续及参考帧 PCM 输出。没有把 helper 的临时 bus 释放改成整帧长期 hold。
- caller 与嵌套 YM／PSG 服务共用一个操作预算；预算耗尽、无效输入、capture 错误与 detached 分支的失败传播通过。
- `service.go` 与 `psg.go` 仅把已有 canonical leaf 提取成共享时钟／预算的内部方法，公开独立接口不变。旧生产文件归档、18 个旧测试／夹具逐字节一致检查及完整旧回归，见 [shared-leaf-compatibility.json](shared-leaf-compatibility.json)。旧冻结清单保留历史源 SHA，不改写成当前证明。

测试先于实现，缺 API 的真实 red 已保留。派生夹具 v1 误选同帧中较早的世界暂停请求；首次对照实际失败。按未修改原始记录中的 driver ACK／clear 点修正后，499 个 Main IO 与 levels 不变，生产代码及冻结 Go 测试不变。v1、提取脚本旧版、red、修正依据和 v2 均保留。离线再次提取逐字节相同，重复目录已删除；本批没有重新运行原生 core。

自然 IO 记录未直接观察 PSG 尾部和最终 RTS；对应尾部依赖已有 PSG 原生 leaf 证明与固定静态成本。实际 mixed 验证也未使用新增完整原版 PCM oracle。因而本批没有完成全 driver、独立外部反馈、完整世界／IRQ 或整场逐采样音频验收。

命令、环境、来源与产物绑定见 [validation.json](validation.json)。离线审计为 `scripts/yuyuhakusho/verify_pause_clock.py --audit`；复跑使用 `--mode native|wasm|vet --out <新的仓库内目录>`，保留已有结果。
