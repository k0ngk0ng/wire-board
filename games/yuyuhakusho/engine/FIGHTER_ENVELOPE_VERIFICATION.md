# 人物调用包络：实现前验证范围

恢复动作外层的输入选择 `0x15540`、屏幕投影 `0x156f0`、生命／能量显示更新 `0x15772` 与人物列表提交 `0x14922`。已有 `LatchFighterInput`、`ProjectFighter` 继续复用，不另写近似版本。AI 的实际入口为 `0x1cc8a`，由专属规则处理；包络必须显式暴露该调用及录制恢复位置，不使用人造 AI 输入冒充完整函数结果。

可能失败的场景：禁用时错误清原始输入或保留窗口；混淆普通、人类、播放、录制与 AI 分支；把有符号 C9 当枚举正值；遗漏播放停止标志、Start 终止和指针推进；输入扩展字节没有按原分支清除；录制模式漏写原始输入，或把终止位写到逻辑缓冲而非原地址；将 AI 的期望输出回灌到包络测试；重算屏幕投影而丢失 word 回绕；显示生命／能量与实际资源混用；增减插值、闪烁阈值、VDP 请求、旧 HUD 字或图块源错误；误删未写缓冲内容；人物列表计数单位或存储边界错误；原版观察改变 3,600 帧基线；把输入边界和显示函数通过当成完整世界步。

先补齐已确认入口的静态指令与实际运行边界，再保存原 CPU 完整字段／缓冲结果，编写测试并保留真实缺实现失败，最后添加独立语义。AI 相关前缀与恢复点单独命名；独立原版 AI 尚未接入时不得宣称该路径已完整返回。正常模式／播放／录制、HUD 与人物提交的完整输出须保留可复跑夹具，涉及 RAM 私有存储的边界由调用者提供，不能凭临时取证窗口发明原版容量裁切。

## 已验证结果

自然记录共 19,919 个完整函数或明确前缀：输入 4,134、HUD 4,134、人物登记 4,134、屏幕投影 7,517。输入中 2,340 次完整返回，1,794 次在 `0x156e8` 调用 AI 之前结束；没有把 AI 输出放进 Go 作为期望中间态。自然模式均为 C9=0，播放与录制由隔离矩阵覆盖。观察的全部 3,600 帧 RAM／视频与固定基线一致。

隔离记录共 3,601 次：输入 2,160、录制恢复 81、HUD 1,344、登记 16，包含 48 个 AI 后录制前缀和 24 个普通 AI 前缀。函数调用期间没有许可范围外 RAM 改写。6 条单初态流共 72 步，其中输入 24、HUD 48；输入流仅更新真实外部控制帧，所有流都不恢复或注入期望中间状态。它们没有人物动作、碰撞或完整世界时钟，不是完整对局。

原生竞态与 Node/Wasm 的 3 项聚焦测试均通过。测试使用显式的 `input.go`、`fighter_input.go`、`motion.go`、`fighter_envelope.go` 和专属测试文件，避免与其他代理正在建立的缺实现红阶段互相阻塞；父阶段仍需执行整个游戏包集成回归。当前夹具在补充 ROM 写映射核对后重新采集，全部压缩夹具与已经通过测试的字节完全相同。来源、日志和摘要见[验证记录](testdata/fighter-envelope-validation.json)。

## 输入与录制边界

`PrepareFighterInputEnvelope` 先检查全局 `A032.bit0`。禁用仅清 Held／Pressed／Changed 的基础及扩展字节，保留 RawHeld 与输入保留计时。C9 的有符号正值播放四字节记录，清扩展输入，源 Held.bit7 设置全局停止标志；C9=0 选择人类或 AI；负值在取输入之后继续录制。人类取样复用已有 `LatchFighterInput`。

AI 路径返回 `FighterInputNeedsAI` 或 `FighterInputNeedsAIThenRecord`。调用者执行真实 AI 规则；后者还须将 AI 后的控制状态交给 `RecordFighterInputEnvelope`，对应 `0x15594` 恢复点。原始 Mode／Channel、停止标志、保留窗口和流位置均保持明确，包络不代造电脑按键。

录制终止存在已验证的地址别名：`0x155c0` 将 A2 改成控制器通道，随后四条 BSET 写 A2−4～A2−1，未使用录制流末尾。通道 0～3 分别改变 RAM `FFFC..FFFF` 的后 4～1 字节，通道 4～7 落入 ROM 区。固定内核的实际 ROM 槽零写处理器已核对为 `m68k_unused_8_w`；这些写入不改变对应尾 RAM 标记。八个通道的原 CPU 例子见[终止别名记录](testdata/fighter-envelope-recording-aliases.json)。该行为没有按常识改写成流结束符。

Disabled、Stopped、EndMarkers 是世界共享数据，必须按原人物顺序传递。流位置是语义存储的身份／游标，不是独立引擎执行的地址；缺失或越界的调用者存储原子失败。本轮不覆盖损坏指针的所有 M68K 总线别名。

## HUD、登记与呈现

`StepFighterHUD` 对显示生命 `+7a` 与显示能量 `+bc` 分别向实际资源靠近一单位；它们不与真实资源合并。生命小于 112、能量大于等于 200 的闪烁使用 `F585.bit0`。两个条更新保留旧 tile 单元，输出最多两个 `FighterBarTransfer`，各含 0～31 的图样 ID 与目标 tile ID；生产规则没有 ROM 或整幅 RAM 依赖。

测试适配器把新传输描述符编码为原 `0x77c/0x740` 的 16-byte 队列命令，与完整 3,072 bytes 队列、1,024 bytes HUD 单元、游标和完整 256-byte 人物比较。私有 HUD 显示值覆盖自然非负范围及 224／255 边界；不宣称损坏显示值与任意越界 HUD 存储的硬件地址别名均已恢复。HUD 单元和传输游标同样是世界共享状态。

`RegisterFighter` 覆盖 `0x14922`：按 byte 偏移将 owner word 提交到人物列表，再加二。原版无固定容量裁切，独立 API 由调用者存储提供边界；MOVEA.W 的负偏移不能因提供大 slice 就被误作正索引。该边界先建立真实失败测试，再修正符号扩展处理。列表和计数是共享状态，供原版影子对象读取；影子对象后续更新不在本阶段中。`ProjectFighter` 直接复用既有 `0x156f0` 实现，没有另建投影规则。

## 复跑

从 worktree 根目录运行原版采样：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter_envelope.py \
  --out .local/md-yuyuhakusho/fighter-envelope-replay
```

完整包集成执行 `go test -race -count=1 -run '^TestFighterEnvelope' ./games/yuyuhakusho/engine`。聚焦显式文件方式从 `games/yuyuhakusho/engine` 执行 `go test -race -count=1 input.go fighter_input.go motion.go fighter_envelope.go fighter_envelope_test.go`；Wasm 使用同样文件与研究工具说明中的 Node 执行器。缓存及临时目录始终设为 worktree 内 `.local/` 路径。
