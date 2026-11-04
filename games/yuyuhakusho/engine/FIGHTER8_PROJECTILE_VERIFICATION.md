# 户愚吕兄子对象：实现前边界

来源 8 对应原 HUD 的「戸愚呂(兄)」，来源 10 为弟。前期将两者兄弟标注写反，本文件已更正当前姓名；来源 ID、代码与原 CPU 证据保持不变。先行记录和初始红灯中的旧标注保留为历史材料。

本阶段只恢复来源 8 的 `147AE`，初始化从 `147B4` 到持续段 `1480C`；规则结束于 `1483E→1580` 绘制请求或 `1487C→3556` 完成前缀清除。静态资料来自[直接 Ghidra 的 31 条指令](../../../docs/research/md-yuyuhakusho/static/fighter8-projectile.txt)，不是执行覆盖率。REA 不支持 raw 68000，未声称 REA 反编译。

该回调不读父体、速度、Power、HitStopTicks，不积分位置，不分配 FX，也不调用 RNG。窄视图只持有回调实际读取／写入的字段，直接映射现有 canonical `WorldObject`：

| 视图 | 现有所有者 |
| --- | --- |
| `Header MatchObjectHeader` | `WorldObject.Header`，含 Behavior／Size／Stage |
| `Animation AnimationActorState` | `AsAnimation()` 已有字段；ObjectID 只由此视图持有 |
| GraphicsSet／TilesBase／GraphicCache | 同名 `WorldObject` 字段 |
| Health／Shake | `Body.Health`／`WorldObject.Shake` |
| ScreenX／ScreenY／ScreenOffsetX／ScreenOffsetY | 对应既有屏幕坐标与临时偏移 |
| Buffers／PendingSound／SoundQueue | 当前世界共享缓冲与声音视图 |

接口采用 `StepFighter8Projectile(Fighter8ProjectileState, ChuAttachmentWorld, AnimationLibrary)`，返回窄状态和 `ChuProjectileOutcome`。不嵌入 `ChuProjectileState`，不新增 canonical 字段。删除时外层先对原对象执行 `ApplyObjectClearPrefix`，再合回窄视图，确保 Depth／Layer／Team／旧速度等未进入此视图的字段也按实际前缀清除。

实现前必须取得原 CPU 字节并验证以下失败场景：

- 从真实来源 8 初始化及输入取得父／子上下文，不能把其他角色的 tag 改成 8。自然观察逐视频步匹配共享未观察基线，存档恢复保留 `refresh_cycles`。
- 初始化资源严格为 Graphics `D104A`、Tiles `1E0000`、Animation `D1E5A`、Shape `D2C6A`；清偏移／参数／四个 cache，但保留旧 FrameIndex／Flags／Completed／位置和其他未写字段。
- 初始化、Duration 8／Ticks 255 与 Health 255 写入先于暂停／慢动作；暂停只请求绘制，不推进计时、清 Shake、提交形状或投影。
- 正常步骤先清 Shake、再推进原动画时钟；Completed 恰为 1 才删除。其他非零 byte 不能被当作完成，byte 时钟回绕和终止 Flags 保持原顺序。
- 无运动或父体依赖。父体位置／阶段／速度、子体旧速度／HitStopTicks、无关 FX 游标变化都不能引入追踪、停顿或额外拒绝。
- 动画提交及投影使用当前世界缓冲、待处理声音、色盘和相机；坐标／偏移 word 回绕、朝向非规范 byte、缓冲饱和及旧字段保留均须比较。
- 大小 0 的默认 64-byte 清除、向下取整的 16-byte 前缀与部分清除分别验证。动画缺失、活跃缓冲游标非法及晚期清除错误原子回滚，不留下初始化或前置写入。
- `3F/1C196` 原创建器的 Team `+10h`、Attributes 只 `+60h`、Ground 高 word、X／Y 完整 long、Facing／Depth／Layer−4／TargetMask／Pose A6 与旧速度／Power保留须逐字节核对，再从同一独立状态运行子回调到删除。
- 完整父／三个子槽、FX 池、shape 缓冲／游标、声音和 RNG 保留原字节；私有调用另检查全 RAM 范围与返回 SP，自然 IRQ／声音消费者按实际边界留证，不掩屏规则字段。

所需 `A6` 四条记录已存在于全角色目录，索引 `0..3`、Duration `80/8/8/8`，末 Flags `80h`。后续以实际回调逐条验证并与现有目录比较；若一致，只保留独立 CPU 语义证据，不修改全局 bundle 或 schema。

先保存自然、边界及真实创建器到删除的材料，再写测试和实际缺实现红灯，最后加入独立生产规则。完整人物、世界、全新对局和平台 E2E 留给后续组合；本阶段不改既有 Go、bundle、PROGRESS 或服务资产。

## 已验证结果

| 范围 | 数量 | 实际覆盖 |
| --- | ---: | --- |
| 普通真实菜单流 | 3,602 视频步 | RAM／画面逐步匹配共享原内核；`147AE` 为 0，oracle 明确保留 `pass:false` |
| 合法特殊流 | 105 次完整回调 | 104 次绘制请求，加第 1,921 步一次实际 256-byte 删除 |
| 原 CPU 私有边界 | 3,279 次 | 子回调 3,135、`3F` 创建器 144，包含脏字段、时钟／完成分支、投影和部分 clear |
| 单初态父子流 | 24 条、7,788 个阶段 | 父体按真实时钟完成移动／翻转并恢复站立；全部已建子对象到删除 |
| 独立规则验证 | 5 项顶层测试 | native `-race` 25.083 秒，Node/Wasm 6.245 秒，完整 engine `go vet` 通过 |

私有材料使用真实普通来源 8 运行到第 2,400 步的 checkpoint，保留其初始化资源、形状与参数来源；没有使用其他角色改 tag 的替代初态。每次恢复使用 `restore_refresh_cycles=True`。24 条流包含 1,752 次只读记录刷新，其余 6,036 次为原 CPU 时钟／父动作／子回调。刷新只根据双方各自当前 pose／index 读取相同记录并写 Duration／Flags，不回灌期望位置、对象、时钟或 RNG。父体完成所测 `3F` 后保持站立，其余子回调继续到清除；不是完整世界模拟。

每条流的父体在动画索引 `1`／ticks `0` 按三种完整速度移动，并翻转 Facing bit0；子体保持创建位置与朝向。测试对这些变化逐次断言，证明 `147AE` 没有跟随依赖。完整父／三个子槽 1,024 bytes、FX 池 2,048 bytes、shape 缓冲 1,024 bytes 及游标、声音、RNG 和环境都参与比较。私有调用另检查完整 64 KiB RAM 写入范围和返回 SP，没有规则字节屏蔽。

[特殊输入](testdata/fighter8-special-inputs.json)的所有 3,602 个视频步均匹配[共享原版基线](testdata/fighter8-attacks-special-reference-manifest.json)，未为自然采样改写 RAM。创建器的入口视频标签为 1,816，子回调首次入口标签为 1,817；业务调用可以跨视频步，标签差不表示补一次逻辑 tick。105 次子回调从 1,817 到 1,921 完整采集，最后一次完成清除，不把已清除对象遗漏为只有 104 次调用。

第 1,825／1,826／1,828 步的三个回调合计跨六个 RAM IRQ。各次进入 `FFFFF000`，在 `FFF09A`／`FFF0A8` 返回，分别解释 26／25／25 个规则输出区外的最终 byte 变化。原 IRQ 前后差异和逐 byte 写入链保存在[自然夹具](testdata/fighter8-projectile-special-oracle.json.gz)，未解释变化为 0。队列消费者和 RNG 调用均为 0；父／子／FX／缓冲和声音等规则结果仍完整比较。

四条 `A6` 记录已经分别通过左右朝向的完整原回调，并在自然完整生命周期中观察。夹具中其余六条父动作／恢复时序记录也与现有全角色目录相等。[独立动画证据](testdata/fighter8-projectile-animation-validation.json)及[自然动画证据](testdata/fighter8-projectile-special-animation-validation.json)只绑定已存在的记录和 CPU 输出；没有新增 runtime data 文件或修改 bundle／schema。

## 测试先行与复跑

失败清单、真实普通基线、私有原 CPU 字节／创建器连续流和所有测试先于生产文件。[真实红灯](testdata/fighter8-projectile-red.jsonl)明确缺少本阶段自己的窄状态／函数，[先行记录](testdata/fighter8-projectile-test-first.json)绑定当时材料。特殊输入就绪后，以同一先行自然断言验证完整实际生命周期；生产规则未作中途修正。当前源码、原版来源、夹具和日志摘要在[验证报告](testdata/fighter8-projectile-validation.json)。

从 worktree 根目录执行，复用 selector 已保存的未观察参考帧，不重复控制组：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter8_projectile.py \
  --out .local/md-yuyuhakusho/fighter8-projectile-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter8_projectile.py \
  --out .local/md-yuyuhakusho/fighter8-projectile-special-replay --natural-only \
  --prefix fighter8-projectile-special \
  --inputs games/yuyuhakusho/engine/testdata/fighter8-special-inputs.json \
  --reference games/yuyuhakusho/engine/testdata/fighter8-attacks-special-reference-frames.json \
  --reference-manifest games/yuyuhakusho/engine/testdata/fighter8-attacks-special-reference-manifest.json

mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json ./games/yuyuhakusho/engine -run '^TestFighter8Projectile'

GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
GOOS=js GOARCH=wasm \
go test -count=1 -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
  ./games/yuyuhakusho/engine -run '^TestFighter8Projectile'
```

上述命令只采集／验证所列闭合边界，绘制请求不执行硬件 sprite 队列。原型环境、工具缓存和输出均留在 worktree；本阶段未提交、发布、上传或修改本地展示服务资产。
