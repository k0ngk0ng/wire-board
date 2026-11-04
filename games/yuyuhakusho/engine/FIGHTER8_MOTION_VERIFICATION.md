# 户愚吕兄 motion：实现前验证范围

恢复来源 8 的 `3e/3f/40`（`1c09a/1c196/1c27c→15afc`）与共享 `0f/11` 新来源上下文。接口拟为 `StepFighter8Motion(ChuSpecialMotionState, JumpParameters)`，没有新增 canonical 字段；完整 `+0x48` long 由 `AlternateVelocity` 唯一持有。MD 大端 `+0x48` 的 word 为该 long 的高 word，`40` 计数只修改高 word，保留低 word。

实现前失败场景：只改 tag 而沿用其他来源资源；`3e` 小于／等于／大于帧7的阶段顺序错误、上优先下及±0x40000边界的一次超调错误、旧纵速／位置 long与地面 signed高word处理错误、残影高度／ticks条件、X按任意非零facing及撞击bit2/3跳帧顺序错误、完成误按ground恢复站立而非无条件1860c的air02/07和参数重置；`3f` 非frame0/ticks0或busy仍建立、team字节加0x10／TargetBit递增／Attrs只加0x60误套mask/OR、ground只写高word、child位置与旧速度/Power错误、父frame1先完整VX偏移再XOR Facing.bit0顺序错误；`40` pose分派优先级和Completed处理错误、高word计数与signed128门槛／word回绕错误、提前退出漏计数、C与Right/Left优先级错误、±0x30000一次超调和任意facing下VX直接加X错误、ae先半速后位移及负数算术移位错误；正常busy／FX不足错误回滚、诊断错误漏回滚、资源小数／声音／RNG／完整父三子FX丢失、单初态实际时钟流回灌期望状态或冒充完整actor/child/world验收。

`147ae` 初始化需要的字段均已由 `ChuSpecialChild` 覆盖，没有必要新建父引用、速度或计数副本。只建立子槽，不推进子生命周期。先原CPU自然／有界dirty矩阵与实际动画时钟流，再冻结四项测试并留实际缺实现红灯，最后新增独立Go；旧Go/bundle/PROGRESS保持本任务未修改。静态入口来自已有直接Ghidra清单，REA不支持raw68000；引用观察与静态推断分别说明。所有缓存／临时文件留在worktree，恢复明确启用 refresh-cycle sidecar。

## 当前验证结果

| 证据 | 调用／步骤 | 覆盖 |
| --- | ---: | --- |
| 真实来源 8 普通／特殊自然流 | 458 | 共享 `11` 66、`3e` 80、`3f` 73、`40` 239；`0f` 没有自然记录 |
| 私有完整原 CPU 调用 | 3,184 | `3e` 1,436、`3f` 488、`40` 1,088、共享 `0f` 108／`11` 64 |
| 单初态实际动画时钟流 | 20 条、2,036 步 | 六条真实 air02 恢复，十四条站立恢复 |
| 独立 Go 验证 | 4 项顶层测试 | 最终 native `-race` 10.607 秒，Node/Wasm 2.889 秒，全部通过 |

普通与特殊输入分别从冷启动运行观察 core，与 selector 任务各自独立未观察原 core 的 3,602 视频步基线逐帧比较完整 RAM／画面，全部一致。特殊输入为 `fighter8-special-inputs.json`，SHA-256 `b26515ad1e9f558fc54699fd0fe69c7c1cf7e3e081682f136bc8da1121121cce`；三新槽真实自然运行到各自恢复。参数／父动画／形状指针由真实来源 8 初始化取得，不使用其他来源改 tag。私有原 CPU 输出与全部四测试先于实现；测试初始／最终 SHA-256 为 `e74304524ab73190283ab31bfeb06a4902031188c29560f61b878b8dd7153cb6`。特殊自然流在生产冻结后独立追加并刷新同四项验证，生产与断言保持不变。真实红灯使用 `-gcflags=-e` 完整列出 `StepFighter8Motion` 未定义，同时保留并行 selector 的独立先行缺符号错误，没有加临时桩掩盖。来源／夹具／日志见 `testdata/fighter8-motion-validation.json`。

每次比较完整父人物 256 bytes、三保留子槽 768 bytes、FX 池 2,048 bytes、申请游标及全部拥有字段；未知旧 Duration、速度和 Power 由原字节适配器保留并比较，没有父／子／池字节屏蔽。私有调用审计完整 64 KiB RAM 改写范围及返回 SP，原 RNG／声音请求不属于这些 motion 的改写。独立音频消费者只按实际 PC／地址标记解释队列变化。

## 分派与部分字段

`3e` 完成优先于其他分支，无条件调用 `1860c`：清 ExtraJump、重置动作时钟、载入本来源参数 Gravity、清完整 VX／VY，进入 `02/07` 和阶段 `0x14`，即使 Y 恰在 ground 也如此。未完成时，帧小于 7 只上移 `0x10000`；帧大于 7 只下移并按地面高 word 夹取。帧 7 才处理高度／ticks 残影、Up 优先 Down 的纵速 `0x4000` 调节、Y夹取、按任意非零 facing 的横移，最后按接触 bit2／bit3 跳帧。速度恰等于 `±0x40000` 时允许一次调节超到 `±0x44000`。

`3f` 帧 0／ticks0 主槽空闲时建立 `147ae`：team byte 加 `0x10`、属性 word 仅加 `0x60`，不额外 mask／OR，ground 只改高 word，其他位置／Facing／深度按原写入。旧子 VX／VY／Power 均保留。帧 1／ticks0 是父人物先按完整 VX 与当前 facing 偏移，再 XOR Facing.bit0；不向子槽填这个父偏移。

`40` 先按 pose 分派，所以活动 `ad` 即使 Completed 为 1 仍先计数；计数是完整 `AlternateVelocity` 的高 word，按 word 回绕后做 signed `>128` 判断，低 word 始终保留。超时或 C 新按下先进入 `ae`／重置，当前步不做加速或位移。活动 `ad` 的 Right 优先 Left，`±0x30000` 边界允许一次 `0x3000` 超调，随后直接把 VX 加 X，不按 facing 翻转。`ae` 未完成时先算术减半 VX 再位移；完成恢复站立。其他 pose 只在完成时转 `ad`／重置，计数 long 不另清。

## 连续流和边界

每条流只初始化一次，原 `19c96` 与独立 `StepAnimationClock` 推进当前记录，随后按各自当前 pose／index 选同一只读原动画记录，仅刷新 Duration／Flags。没有注入期望人物、子槽、FX 或 RNG，也没有执行共享尾段／形状／声音提交和子／FX生命周期。

飞行 neutral／上下控制各 80 步，接触跳帧 48 步，均真实进入 air02；克隆 free／busy 各 73 步；完整 `ac→ad→ae` 超时及左右控制各 239 步，预定 C 取消 132 步。空／满 FX 池分别连续验证，半速负余数、word计数回绕和资源小数均按原值推进。实际特殊自然输入另外覆盖三新槽；完整 actor／world、新建对局和画面由后续组合继续验证，本报告不宣称这些已完成。

## 复跑

从 worktree 根目录运行，采集需要固定原 ROM／观察 core 及已验证的原参考帧；Go 测试只读取规范夹具：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter8_motion.py \
  --out .local/md-yuyuhakusho/fighter8-motion-replay
mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -run '^TestFighter8Motion' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -run '^TestFighter8Motion' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

最终只清理本任务重复采集／stdout，规范原 CPU 夹具／manifest、真实红灯、最终 native／Wasm 和参考帧保留。删除范围见 `testdata/fighter8-motion-cleanup.json`；共享缓存、其他任务及 8799 服务资产均未修改或删除。
