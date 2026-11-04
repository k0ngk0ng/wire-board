# 藏马子对象：实现前边界

恢复新回调 `12EE8` 与 `1300C`，复用已有 `12DD4` 的来源 1／pose 9 上下文。静态证据为[直接 Ghidra 有限指令](../../../docs/research/md-yuyuhakusho/static/fighter1-projectile.txt)；REA 不支持 raw 68000，静态指令不作为自然执行覆盖。所有新写入、私有项目及缓存留在当前 worktree。

`12EE8` 初始化 `12EEE` 强制 pose 10；正常步骤推进动画、X／Y 分别加完整 VX／VY，按落地、边界 `±32` 或接触 bit3／4 在 `13006→3556` 直接删除，不分配冲击 FX；否则 `12FC8→1580` 绘制。`1300C` 初始化 `13012` 强制 pose 11，`Aux48` 高 word 写 `2C0h`、`Body.Gravity` 高 word 写 `18h`；持续段更新水平相位、重建 X、推进 Y、递减剩余 word，在零值或接触时经 `13138→3556` 删除，否则 `130FA→1580` 绘制。两者均没有父体依赖、HitStop／FX／RNG 逻辑，也不按 Completed 结束。

最小独立视图已按读写确认，不使用 `ChuProjectileState`：

| 视图 | 既有 canonical 所有者 |
| --- | --- |
| 两者的 `Visual Fighter8ProjectileState` | 只复用已审计一致的 Header／Animation／Graphics／Cache／Health／Shake／Screen／共享 buffers／sound 字段，不调用 `StepFighter8Projectile` |
| 直线体 VelocityX／VelocityY／Ground／ContactFlags | `Body.VelocityX`／`Body.VerticalVelocity`／`Body.Ground`／`WorldObject.ContactFlags` |
| 摆动体 OriginX／VelocityY | `Body.VelocityX` 的完整 long 是原点；`Body.VerticalVelocity` 为纵速 |
| 摆动体 Phase／Remaining | `Aux48`／`Body.Gravity` 的高 word 视图，低 word 不重复存储 |

接口分别为 `StepFighter1LinearProjectile(state, ChuProjectileEnvironment, AnimationLibrary)` 与 `StepFighter1WaveProjectile(state, ChuAttachmentWorld, AnimationLibrary, []int8)`，均返回自身状态、`ChuProjectileOutcome` 与错误。外层先 `ApplyObjectClearPrefix`，再在已清除对象上合回所有视图及高 word，不能复活低 word。波形直接复用既有 `Arena.WaveX`，其 1,024 项与 ROM `34000 + phase*2` 的 signed byte 逐项相等，不新增波表或 schema。

实现前必须验证的失败场景：

- 自然输入及父体模板来自真实来源 1 初始化，不能把其他角色只改 tag。观察内核逐视频步匹配同输入唯一原内核基线，checkpoint 恢复包含 `refresh_cycles`。
- 初始化清偏移／参数／四 cache、设置公共资源和强制 pose，保留旧 FrameIndex／Flags／Completed；Health 255 与全部初始化先于暂停／慢动作。
- 暂停不清 Shake、不推进时钟／相位／剩余数／坐标、不提交形状或投影。非零旧 HitStop 不引入人物停顿，Completed 任意值均不额外触发删除。
- 直线体积分用完整 long 回绕；落地使用纵速 long 符号及坐标高 word；左右边界加减为 word，严格比较及分支优先级保持。接触／落地直接清除，不套用散射弹冲击 FX。
- 摆动体初始化只写 Phase／Remaining 高 word，保留低 word；每步先加 `40h` 再与 `3FFh` 相与，索引更新后读 signed byte、扩展后算术右移 1，不能用向零除法。
- Facing 任意非零值取反位移。X 先完整复制 OriginX，再只改高 word；旧 X 小数必须被原点的小数替换。Y 按完整 VY 积分，无横速积分、地面或左右边界检查。
- Remaining 先 word 递减，原值零回绕到 `FFFFh`；递减得到零先于接触检查。正常完成／接触均不分配 FX，也不按动画终止位直接删。
- shape 缓冲饱和、无效活跃游标、缺动画／波表及晚期清除错误原子回滚；Size 0 默认 64、向下取整的 16-byte 前缀清除分别核对全部字段。
- `1B/1A8F0` 创建直线体；`1C/1A9BE` 创建摆动体，必须保留 child.X 旧值并把 parent.X 写到 OriginX，循环游标只由原父 Power word 持有。`1A/1AAC2` 创建现有 `12DD4`／pose9，不复制其规则。
- 原父／三子槽、FX／extras、shape 缓冲／游标、声音及 RNG 完整比较；私有调用另查全 RAM 和返回 SP。父子真实动画流只初始化一次，不回灌期望中间状态；自然 IRQ／消费者按实际边界解释。

公共 pose 10 两帧、pose 11 六帧当前缺失，须逐条用完整原 CPU 回调验证，再独立导出八条语义数据供主任务合并。已有 pose 9 与角色 1 自有记录先与现有目录核对，不重复修改全局数据。先保留 CPU 材料、测试和真实缺实现红灯，再写新生产规则；不修改旧 Go、bundle、PROGRESS、其他报告或展示服务资产。

## 私有与创建器材料

普通真实来源 1 的 3,602 视频步观察结果逐步匹配共享原内核参考。该流没有触发三种子回调，oracle 保留零记录与 `pass:false`，不作为自然覆盖。

| 证据 | 数量 | 范围 |
| --- | ---: | --- |
| 完整原 CPU 边界 | 4,230 次 | 直线体 1,535、摆动体 2,695；包含 1,024 个波形索引逐项实际访问 |
| 单初态父子流 | 16 条、4,588 阶段 | `1A/1B/1C` 父动作、真实时钟与全部三个保留子槽，直到父体站立且已建子对象全部清除 |
| 只读时序刷新 | 984 次 | 按双方各自 pose／index 的同一不可变记录，仅刷新 Duration／Flags |
| 实际原 CPU 调用 | 3,604 次 | 父时钟 984、父动作 984、旧 pose9 子体 268、直线体 504、摆动体 864 |

私有初态来自普通真实来源 1 的第 2,400 步 checkpoint，资源／形状／参数指针保留原初始化来源。每次恢复显式包含 `refresh_cycles`；每条父子流只初始化一次。后续没有回灌预期父／子／FX／RNG、时钟或位置，所有 CPU 前后都保留完整父／三子、FX／extras、shape 缓冲及游标、声音与 RNG。私有调用另验证完整 64 KiB RAM 写入及返回 SP，未解释变化为 0。

[独立语义文件](../data/fighter1-projectile-rules.json)只含八条新 AN／形状记录，pose10 两条、pose11 六条，大小 2,813 bytes，SHA-256 `09c7c5eeff2d896983edb8200ec47ff2fc20675394658d2f40207e1b1554914b`。它没有原 CPU 输入、RAM、指令或像素载荷，不重复波形表。[来源报告](../data/fighter1-projectile-rules-manifest.json)绑定当前采集器、固定 ROM 及既有 Arena.WaveX。已有父体记录与 pose9 则逐项核对现有目录。

所有测试在生产文件前写入，[真实红灯](testdata/fighter1-projectile-red.jsonl)明确缺少本阶段自己的两种状态和函数；[先行记录](testdata/fighter1-projectile-test-first.json)绑定当时材料。四项私有／创建器／数据／原子测试首次正常 package native `-race` 20.821 秒、Node/Wasm 5.339 秒通过，规则没有中途修正。自然回调使用同一预先写入的断言，在合法特殊流就绪后独立追加；不能以私有通过替代自然或完整世界验收。

## 自然与最终验证

[合法特殊输入](testdata/fighter1-special-inputs.json)的 3,602 个视频步逐步匹配[唯一原内核基线](testdata/fighter1-attacks-special-reference-manifest.json)。被动采集获得 161 次完整回调：摆动体 72、直线体 22、已有 pose9 子体 67，包含五次完成的实际清除。没有为自然运行改写 RAM、得分或动作状态。原发现按仍活跃对象计数为 69／21／66，完整回调另包含三个摆动体及另两种各一次删除，不能把清除入口遗漏。

最终五项正常 package 测试全部通过：native `-race` 22.069 秒，Node/Wasm 5.624 秒，完整 engine `go vet` 通过。自然、私有及连续材料的 RNG／独立队列消费者／IRQ 范围外改写均为 0；规则输出未作字节屏蔽。生产规则没有中途修正。当前代码、冻结父动作、两份原版来源、夹具、日志和清理摘要见[验证报告](testdata/fighter1-projectile-validation.json)。

摆动体使用既有 `Arena.WaveX`，全 1,024 个相位都由原 CPU 访问并对照。Phase 和 Remaining 只在视图中持有高 word；canonical 合并先清实际前缀，再回写高 word，低 word 保留或清零均按原范围处理。直线体及摆动体只共享已审计一致的窄图形字段和前缀清理，不调用 `StepFighter8Projectile`；已有 pose9 子体继续直接调用 `StepFighter2Projectile`。

## 复跑

从 worktree 根目录运行，复用已保存的原内核参考，不重复控制组：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter1_projectile.py \
  --out .local/md-yuyuhakusho/fighter1-projectile-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter1_projectile.py \
  --out .local/md-yuyuhakusho/fighter1-projectile-special-replay --natural-only \
  --prefix fighter1-projectile-special \
  --inputs games/yuyuhakusho/engine/testdata/fighter1-special-inputs.json \
  --reference games/yuyuhakusho/engine/testdata/fighter1-attacks-special-reference-frames.json \
  --reference-manifest games/yuyuhakusho/engine/testdata/fighter1-attacks-special-reference-manifest.json

mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json ./games/yuyuhakusho/engine -run '^TestFighter1Projectile'

GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
GOOS=js GOARCH=wasm \
go test -count=1 -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
  ./games/yuyuhakusho/engine -run '^TestFighter1Projectile'
```

上述父子阶段只包含原时钟／动作／子回调和只读时序刷新，父体离开所测动作后保持恢复状态。碰撞、活跃 FX 生命周期、共享尾段、完整人物／世界和浏览器 E2E 不由这些流替代。本阶段没有提交、发布、上传或修改本地服务资产。
