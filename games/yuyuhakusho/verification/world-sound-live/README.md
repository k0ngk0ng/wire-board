# 世界声音请求与联合事务

本次将实际世界调用链中的六类 enqueue 来源接到 World 的同一份队列：动画提交、碰撞管理、特效初始化、开场、比赛流程、提示／结算。运行时不增加 ROM、CPU 解释器、原 PC 时间表或持久事件历史。

`StepChuWorldObjectsObserved` 在现有对象事务中建立临时端口，经人物动作、子对象和形状提交路径显式传递。所有请求复用 `EnqueueSoundObserved` 的实际 TST、CMP、写入及返回操作。中断改写的是 World 队列和 AudioPause；组件返回的临时队列视图立即更新，防止回调合并覆盖消费者。现有公开纯规则 API 保持签名，并使用同一实现。

`StepChuSessionFrameObserved` 把按键、世界对象和帧末调色板纳入原有撤销边界。`audio/session.Runtime.WorldFrame` 再分叉实际音轨、音频主控、PCM、芯片及声音调度器；整个世界帧成功后一起提交。hook 失败、调用契约错误、后续对象失败或调色板失败均丢弃分支。端口、owner 指针、音频分支及 apply 只能在同步调用中使用，不写入存档。

## 验证范围

先行场景见 [FAILURES.md](FAILURES.md)，测试冻结记录见 [test-first.json](test-first.json)。真实缺 API 编译失败保存在 `missing-api-red.jsonl`。

- 11 个角色的既有原生整轮对象夹具：被动观察与纯接口逐轮比较全部状态和事件，并继续检查原版世界状态与回调次序。原版 queue 有独立消费行为，旧原版世界比较仍保留其原有 queue 排除；不能把这一项称为原版混合音频验证。
- 介入测试在负槽 TST／CMP 之间修改队列，验证实际重新读取；入队返回时清空或替换队列及修改 AudioPause，验证旧组件不会写回过期状态。
- 真实 `audio/session.Interrupt` 从 World producer 中消费请求，与同初态的独立直接调用比较完整音频状态；后续世界失败时验证整个音频与世界回滚，并验证 JSON 恢复。
- 旧动画、碰撞、特效、开场、比赛流程、提示及声音调度测试保留原样回归。

合成事务测试最初误传 `MaxOperations=1000000`，超过已有 `tickService` 上限 65536。在实际直接 IRQ 调用失败后，仅把测试调用预算修正为 65536；生产预算检查、原生输入与预期结果均未变。原测试、准确 diff 和理由见 [test-adapter-correction.json](test-adapter-correction.json)。另一次覆盖断言发现：11 份 World 原版夹具均从开场资源设置完成后开始，不能覆盖 start 来源。所有 24,454 轮原版对照先已通过，随后总覆盖断言正确失败。仅在测试准备中增加实际 NewChuSession → CompleteChuSessionResource → 首个逻辑帧，保留六来源断言与全部原版夹具不变。原测试、diff 和理由同样归档。两次接线阶段编译失败、该次 Wasm 失败及被替代的原生运行日志均保留。

复跑命令（输出目录必须是工作区内尚不存在的目录）：

```sh
python3 -B scripts/yuyuhakusho/verify_world_sound_live.py --mode native --out .local/md-yuyuhakusho/world-sound-live/recheck-native
python3 -B scripts/yuyuhakusho/verify_world_sound_live.py --mode native-race --out .local/md-yuyuhakusho/world-sound-live/recheck-race
python3 -B scripts/yuyuhakusho/verify_world_sound_live.py --mode wasm --out .local/md-yuyuhakusho/world-sound-live/recheck-wasm
python3 -B scripts/yuyuhakusho/verify_world_sound_live.py --mode vet --out .local/md-yuyuhakusho/world-sound-live/recheck-vet
python3 -B scripts/yuyuhakusho/verify_world_sound_live.py --audit
```

原生完整回归和 Wasm 执行同一测试范围；额外的 `native-race` 聚焦这次新增的队列介入、调用租约、门槛和真实音频事务回滚。完整原生竞态回放的两次中间记录保留，不能把它们列为通过。Wasm 沿用前次异常调查的 `gccheckmark=1,clobberfree=1` 检查。通过本次测试不代表先前不可复现的 Go heap 异常已修复。

## 最终结果

| 运行范围 | 顶层测试 | 含子项记录 | 结果 |
| --- | ---: | ---: | --- |
| 原生完整回归 | 29 | 66 | 通过，56.19 秒 |
| Node／Wasm 完整回归 | 29 | 66 | 通过，232.75 秒 |
| 新增介入与事务路径的原生竞态检查 | 4 | 18 | 通过，8.17 秒 |

两个完整回归的测试名称集合一致，覆盖 24,454 轮原版世界对照和实际开场流程；共观察 4,336 次 enqueue 操作（animation 3,540、collision 672、effect 76、lifecycle 20、overlay 24、start 4）。相关 vet、27 个 Go 文件格式和文档链接检查通过。2,129 个原有受保护测试／夹具／资源路径未改动。正式记录和压缩日志的 SHA-256 见 [validation.json](validation.json)。

## 尚未完成

本接口只开放 enqueue 内部的同步介入点，不为纯世界计算、pending 写入、暂停切换或整个对象回调计价，也不提供无声音请求区间的中断接受点。事务测试中的 IRQ 入口时钟是合成输入，不能用于正式世界调度。

仍须按实际世界工作量和 VDP／CPU 中断规则计算进入时机，允许 IRQ 落在 arena、collision、fighter 等实际路径内部；之后才能完成带时钟的联合帧存档、平台 PCM 输出、资源加载及完整对局听音验收。固定帧首／帧尾方案仍被既有反例排除。当前试玩服务继续保持原构建，完整比赛音频未开启。
