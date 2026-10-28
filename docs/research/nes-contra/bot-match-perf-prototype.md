# 纯像素匹配的行位平面原型

该原型在 Apple M4 上将真实双电脑 50 ms 计时批次的中位耗时从 **65.35 ms 降至 29.76 ms**，约为原来的 45.5%。每批仍执行 3 个权威帧，没有降低规则、呈现或观察频率，也没有缩小模板与候选覆盖。正式代码未修改；接线仅通过工作区内的 Go 编译 overlay 完成。

## 方法与语义

每张已公开的 256×240 图像，按 RGB 颜色建立 `240 × 4` 个 `uint64` 的行位平面。利用现有 `locations`，每个像素只设置一个 bit。静态模板按颜色与行压缩为 mask，在构造 `BotVision` 时编译一次。

一个候选的匹配使用移位、按位与和 `bits.OnesCount64`。亮色命中分别累积到 X／Y 位并集，由首尾置位位置恢复原跨度。返回分数仍严格为 `details × 20 + matched`；完整匹配、12 个细节点、40% 覆盖、两种亮色及 X／Y 的 7／11 像素跨度条件全部保留。透明模板像素没有 mask；黑色与暗色命中仍计入分数。

只有允许部分匹配且静态几何条件成立的候选使用位平面。纯 Exact 候选仍用原来的逐点早退路径，避免把通常只检查一两个像素的失败候选变慢。原来的稀有色／角点 anchors、两票门槛、8px 色数上界、候选去重、排序和身份判定均保持原样。

匹配器输入只有 packed RGB、不可变模板与候选坐标。World 只出现在基准夹具的合法 Start 和战斗前提断言中，不传入匹配器。模板的点坐标唯一、最大宽高为 128，由现有 `NewBotVision` 构造约束保证。

## 等价性证据

先编写边界／差分测试并运行失败，再添加原型 helper。新增文件均以 `contra_match_perf_prototype` 标签隔离，不进入普通构建：

- `games/contra/engine/bot_match_perf_prototype_test.go`
- `games/contra/engine/bot_match_perf_prototype_helper_test.go`

验证覆盖原有 **51 个边界**、固定种子的 **18,760 个随机候选**，包括 63／64／65／127／128 宽度、所有 64 位对齐偏移、屏幕右下边缘、透明和亮度边界。实际战斗图像另逐项核对 **91,541 个候选**，其中 31,710 个允许部分匹配、132 个被接受；每项匹配结果、分数及由分数推导的 Exact 标记一致。

战斗夹具使用标准 `NewSession(42, 2, ...)`，第 2 帧按 Start，第 600 帧断言 `LevelRoutine == 4`。静态知识包含 2,332 个模板，公开图像有 14 种 RGB。测试不会把菜单或过场当战斗基准。

边界／随机差分通过 race 检查；隔离接线后，现有像素／HUD、别名、重叠、Exact-only、单物理 anchor 和网格平移等观察器回归也通过 race 检查。

## 测量结果

环境：Go 1.26.2，darwin/arm64，Apple M4。模板与候选基准各运行 3 组；完整计时各运行 5 组，每组 10 次。下表使用中位数。

| 范围 | 原实现 | 原型 | 说明 |
| --- | ---: | ---: | --- |
| 12×16 部分遮挡模板 | 194.2 ns | 58.92 ns | 都返回分数 3,024 |
| 完整战斗候选匹配 | 15.94 ms | 3.98 ms | 91,541 个完全相同的候选 |
| 候选匹配＋每帧位平面 | 15.94 ms | 4.18 ms | 加入新索引构建成本 |
| 双电脑完整 50 ms `OnTimer` | 65.35 ms | 29.76 ms | 包含呈现、观察、策略、规则和状态处理 |

完整计时的原实现范围为 65.03–66.82 ms，原型为 29.55–30.11 ms。每帧新增索引约 115 KB／17 次分配；完整批次中位分配量从约 9.94 MB 增至 10.29 MB。没有把匹配内核的约 3.8 倍提升当作整个计时的提升；完整计时实测约为 **2.20 倍**。

这些数据对应一个经合法流程到达的第 1 关战斗状态。它们不是全部关卡最坏负载结论，也不能代替生产服务器架构上的实测。

## 整合方式

最小接线只有三处：在不可变 `botVisionPattern` 中保留编译 mask；`NewBotVision` 构造阶段编译；`ObserveBotFrame` 从既有 `locations` 建立本帧位平面，并仅替换最终候选匹配调用。原 `botMatchVisualPattern` 保留为 Exact-only 路径及独立差分参照。无需读取碰撞、对象槽位、敌人类型或其他实时规则状态。

私有 `.local/contra-match-perf-prototype/prepare_overlay.py` 复现以上三处接线，生成 `overlay.json` 和源 SHA 绑定。它不覆盖仓库共享文件。正式整合时建议先保留本原型作为差分测试，再在目标服务器重跑完整计时；分配优化可在后续 profile 确认需要后进行，不必同时改变匹配语义。

[验证与源码绑定](evidence/bot-match-perf-prototype-verification.json)记录全部命令和样本；[压缩记录](evidence/bot-match-perf-prototype-records.json.gz)保留先红后绿、候选数量、race 与原始基准输出。临时红日志已压缩归档后删除；没有生成或保留测试二进制，没有修改用户素材、其他游戏或正式观察器文件。

```sh
. scripts/env.sh
go test ./games/contra/engine -tags contra_match_perf_prototype -run '^TestBotMatch(PerfPrototype|VisualPatternBoundaries)' -race -count=1
go test ./games/contra/engine -tags contra_match_perf_prototype -run '^$' -bench '^BenchmarkBotMatchPerfPrototype' -benchtime=500ms -count=3
python3 -B .local/contra-match-perf-prototype/prepare_overlay.py
go test -overlay .local/contra-match-perf-prototype/overlay.json ./games/contra/server -run '^$' -bench '^BenchmarkContraSharedVisualTimer/two-bots$' -benchtime=10x -count=5
```
