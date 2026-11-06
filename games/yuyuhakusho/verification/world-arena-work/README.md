# 默认场景内部耗时与世界音频介入

默认场景的摄像机采样、波形／滚动行生成和反馈复制现在在实际世界调用链中报告工作边界。声音可以在没有新请求的场景计算中执行，后续对象或调色板失败时，真实音频和世界仍共同撤销。

`StepDefaultArenaObserved`／`StepDefaultArenaCameraObserved` 与原有纯接口共享规则实现。成本由当前分支、摄像机样本及实际循环决定；运行时不读取原 PC、opcode、原 CPU、原始 RAM 或记录的时间表。`ArenaWorkOperation.Advance` 借用调用方的唯一主时钟，处理工作边界刷新和摄像机除法的内部刷新跳过。工作上下文和回调不进入存档。

摄像机两个求和均先按 word 溢出，样本数为 1..16。原除法的中间余数始终小于除数乘 65536，因此没有带符号 carry 分支；15 轮中成功减法次数等于 `floor(sum/(2*count))` 的置位数。其内部耗时为 `136 - 2*popcount(...)`，并在结束前按原顺序推进 refresh deadline。

场景计算使用私有值视图；目前允许介入的音频只借用 World 的 queue／AudioPause，不读取中间摄像机或滚动行值。因此这些纯计算与音频操作可以按工作边界交错，最终在回调返回时合并。未来中断若需要读取其他世界字段，须另证这些边界。

## 证据与测试

[失败场景](FAILURES.md)、两份新测试、原版采集脚本、静态成本和原生结果均在生产修改前冻结，见 [test-first.json](test-first.json)。真实缺 API 红灯保留在 `missing-api-red.jsonl`；新测试和原生预期未作后续修订。

- 两个固定参考核心各验证冷启动 2,400 个视频步，再执行 360 个 camera 和 432 个 arena 案例，共 1,584 次调用。每个核心的 187,824 个步骤、时钟、刷新时限及原始结果完全相同。
- 新测试逐步比较 792 个完整调用，覆盖初始化、暂停、slow、样本数量、word 溢出、摄像机限幅、Pan 正／负反馈和内部刷新。
- 实际 `WorldFrameWithHooks` 在 arena 内部运行真实 `session.Interrupt`，与同初态直接调用比较完整音频结果，并验证后续对象／调色板失败时共同撤销。
- 旧场景规则、11 角色共 24,454 轮世界对照及上一轮声音请求／回滚检查继续执行；旧测试、夹具和资源未修改。

| 运行范围 | 顶层测试 | 含子项记录 | 结果 |
| --- | ---: | ---: | --- |
| 原生完整相关回归 | 12 | 834 | 通过，48.75 秒 |
| 实际 Node／Wasm 相关回归 | 12 | 834 | 通过，209.03 秒 |
| 新工作边界及声音事务的原生竞态检查 | 8 | 817 | 通过，17.93 秒 |

相关 vet、Go 格式、文档链接和差异空白检查通过。正式命令、环境、日志与输入／源码摘要由 validation.json 绑定。

采集器第一次在启动任一原生核心前，把既有案例数误写为 1,224；实际为 360 + 432 = 792。仅修正该准备断言，原始夹具不变。记录见 [capture-count-setup.json](capture-count-setup.json)，此错误不属于模型语义失败。

复跑使用工作区内尚不存在的输出目录：

```sh
python3 -B scripts/yuyuhakusho/verify_world_arena_work.py --mode native --out .local/md-yuyuhakusho/arena-recheck-native
python3 -B scripts/yuyuhakusho/verify_world_arena_work.py --mode native-race --out .local/md-yuyuhakusho/arena-recheck-race
python3 -B scripts/yuyuhakusho/verify_world_arena_work.py --mode wasm --out .local/md-yuyuhakusho/arena-recheck-wasm
python3 -B scripts/yuyuhakusho/verify_world_arena_work.py --mode vet --out .local/md-yuyuhakusho/arena-recheck-vet
python3 -B scripts/yuyuhakusho/verify_world_arena_work.py --audit
```

重新采集原版和导出静态成本：

```sh
python3 -B scripts/yuyuhakusho/capture_world_arena_work.py --out .local/md-yuyuhakusho/arena-recapture
python3 -B scripts/yuyuhakusho/export_world_arena_costs.py --out .local/md-yuyuhakusho/arena-costs-reexport.json
```

Wasm 沿用 `gccheckmark=1,clobberfree=1` 检查；此前不可复现的 Go heap 异常仍须在正式浏览器发布验收中关注，不能据本次通过声称已修复。

## 尚未完成

本次成本和工作边界覆盖默认场景及其摄像机，入口主时钟由外部提供。其他场景、人物、碰撞、世界入口及尾部工作仍须计价；未报告工作不代表零耗时。真实 VDP 批次、CPU 屏蔽／接受规则、RAM IRQ 前缀和完整带时钟的联合帧存档仍未接通。

音频介入测试采用声明的合成入口和阈值，证明事务组合，不能当作原版 IRQ 进入时刻。正式平台完整比赛音频仍未开启，试玩服务保持原构建。
