# 飞影双真人吞龙 MAIN 自然证据

本轮使用已冻结的双真人输入及 selector 任务唯一未观察原版 control，采集真实 `9000` 父对象的 `2a/2b` 完整 pre-tail 返回。固定 5,502 个视频步，入口仅接受前 5,500 步，最后 2 步只用于返回排空。不生成第二份未观察 control，不把 `9000` 改为 `8400`。

采集前失败场景：来源／输入／control 摘要错配；把 source3 的 `8400` 或其 child 当作飞影；漏掉 source4 `9000`、输入通道 1、child `9100`；记录超出 5,500 的入口或少跑两步排空；只保存后帧状态而无真实入口／出口；完成时 SP 不一致；吞龙前后 MovementSource、Energy 小数、child contact／输入或 FX Owner 字段遗漏；child 和 FX 交界范围错误；声音／RNG 或历史缓冲变更未解释；把 `2a/2b` 当次返回扩写为完整强化尾段的 Go 验收。

已有四项 MAIN 测试固定要求 `A5=FFFF8400`，因此不将真实 `FFFF9000` 行混入旧 oracle，也不改冻结测试或生产实现。新增夹具用于原版自然取证；调用冻结 MAIN 的双真人完整组合和 whole-world E2E 由对应任务验收。Go 生产、四项冻结测试、原 oracle 397 次、3,818 次私有边界、40 条时钟流及参数资源均保持不变。

## 原版观察结果

独立观察的 5,502 个视频步均与 `fighter4-attacks-duo-reference-frames.json` 的完整 RAM／画面逐步一致。原输入 SHA 为 `b303ca8622f90747052902f81b2b7f2bdde39f59e47feb6f36a6bf8287a6ac34`，唯一未观察 control SHA 为 `9f282f53a016d184d45052b30eaf8c482d4b568132f7979f7eebff4a7faa5917`。source3 真人确为 `8400`／通道 0；飞影确为 `9000`／通道 1，第一保留子对象为 `9100`。

| 阶段 | 原版视频帧 | 完整调用 | 最后状态 |
| --- | --- | ---: | --- |
| `2a` 跟随并吞龙 | 3,488 至 3,589 | 102 | `2b/AB`，参数源 `15946` |
| `2b` 强化动作 | 3,590 至 3,646 | 57 | `00/00`，保留参数源 `15946` |

3,589 帧的真实 `2a` 调用把参数源从 `12578` 改为 `15946`，Energy 高 word 恢复为 223。3,598 帧的 `2b` 调用实际创建 `13fc8`；该子对象的自然回调首尾为 3,598 至 4,299，共 702 次。视频后帧参数源在 4,299 恢复 `12578`；在 4,379 首次同时满足默认参数源、Energy 高 word 为 −1、站立动作和空子槽，至 5,502 仍满足这些条件。这些后续标量是完整原版场景的观察结果。

159 次调用均保存完整父对象 256 字节、三子对象 768 字节、FX 池 2,048 字节及游标，保留真实 `A5=FFFF9000`。每次 SP 均恢复；完整 RAM 变化审计未发现未解释写入，历史缓冲在 MAIN 阶段保持原样，RNG／声音请求变更为 0。没有 RAM 写入、寄存器重定位、私有调用或状态注入。

原始记录与完整视频状态窗口保存在 `testdata/fighter4-motion-duo-natural.json.gz`；输入／control、入口截止及排空、对象绑定和原始数据 SHA 见同名前缀 manifest。`fighter4-motion-duo-windows.json` 提供阶段窗口及恢复时点，`fighter4-motion-duo-unchanged-baseline.json` 绑定未改变的旧数据和冻结代码。

## Go 集成验收

[双真人三层组合 E2E](FIGHTER4_DUO_COMPOSITION_VERIFICATION.md) 通过实际 `9000` 场景调用冻结 MAIN；Action／Input／Actor 各有 3,316 次完整原版返回对照，包含吞龙改源、强化及后续恢复。三项 E2E 首轮全部通过，未修正生产代码或断言：原生竞态检查 39.893 秒，Node-WebAssembly 11.718 秒。测试文件为 `fighter4_duo_composition_test.go`，冻结 SHA 为 `cf4990b5493177c9b5c936a85fb61377baf252672c0dc4d870676b44b7c2b4b4`；日志为 `fighter4-duo-composition-native.jsonl`／`fighter4-duo-composition-wasm.jsonl`。

主任务的 `fighter4_duo_world_test.go` 也已通过原生竞态验收：逐轮对照 38.900 秒，单初态推进 35.740 秒，并执行 7 次真实保存／恢复。Go 边界是原版练习场景已经就绪后的 2,500 至 5,500，共 3,001 轮、34,236 个回调和 18,901 次绘制；前面的原版菜单不列入 Go 边界。单初态世界循环覆盖实际 `9000` 对象的吞龙、强化和恢复，保留原版练习模式回调及阶段，没有把它改为普通比赛。对应日志为 `fighter4-duo-world-native.jsonl`，原版夹具绑定为 `chu-world-fighter4-duo-manifest.json`。

原四项 MAIN 测试仍只消费原 `8400` 数据，没有据此声称它们验证了 `9000`。单独 MAIN 夹具只承担原 CPU 取证；世界调度、子对象生命周期及单初态完整强化流程由上述世界 E2E 直接验收。本文的世界检查结论限于已完成的原生竞态结果；三层组合另有 Node-WebAssembly 绿色结果。

## 复跑原版取证

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter4_motion_duo.py \
  --out .local/md-yuyuhakusho/fighter4-main-duo-recapture
```

该命令只复用现有输入及唯一 control，输出留在工作树内；不会生成第二份 control，也不会覆盖旧四测试的夹具。独立验证摘要见 `testdata/fighter4-motion-duo-validation.json`。
