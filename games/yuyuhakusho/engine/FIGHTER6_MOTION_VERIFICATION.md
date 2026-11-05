# 阵 MAIN 动作验证

`StepFighter6Motion` 的五个 MAIN 动作已通过原版 CPU 完整字节对照和 Go 原生竞态／Node-WebAssembly 验证。本轮范围截至原版公共尾处理入口 `15afc`，尚未据此宣称角色、世界或完整对局验收完成。

## 实现前验证范围

MAIN负责 `33/34/35/36/38`（`1bab2/1bad6/1bafe/1bbbe/1bc92→15afc`），统一入口对 `0e/0f/11/13/37` 委派独立support任务。最小类型为 `Fighter6MotionState{ChuSpecialMotionState; StunTicks uint16}`，`+0x84` word已有StunTicks所有者，不新增涡旋计数；`33`的`+0x48`高word沿用完整AlternateVelocity，低word保持。`enterFighter6Exit(*FighterBodyState)`由support文件提供，对应原18682，本MAIN不重复实现。

实现前失败场景：错误来源资源；33先计数或错误读取完成、忽略先Y加VY、计数高word／精确等16和word回绕错误、误清低word、1860c误条件站立；34增加重力／先夹Y、地面signed高word／小数错误、BeginLanding与ExtraJump清零顺序错误、FX错误泄露前置Y；35/36起始帧及常量40000/20000/10000分段错位、ground高word−32的word回绕／残影ticks门槛、任意非零facing、先位移再contact.bit0跳帧、完成ground高word相等才stand否则air错误、存储VX被常量覆盖；38先pose分派与Completed优先级错误、StunTicks word计数／signed256和128门槛、A/B/C取消优先级、Right优先Left及30000边界一次超调、ae类先半速后位移和负余数错误、18682误当air或漏Aux7E←Aux80；support误内联／重复canonical；资源小数／完整父三子FX／声音/RNG遗漏，正常满池当错误，诊断错误漏回滚，实际动画流回灌期望状态，或把phase通过写成完整actor/world。

先原CPU完整自然／有界dirty／实际只读动画时钟流，再冻结四项MAIN测试并保存实际缺实现red，最后只新增MAIN Go。普通／special输入和唯一未观察3602控制由Root／selector共享，本任务独立观察core逐视频核对RAM／画面。参数／动画／形状指针来自source6真实初始化；私有canonical A5、FFFF00下栈、refresh sidecar恢复、全RAM审计与完整SP恢复都保留。静态入口来自已有直接Ghidra清单；REA不支持raw68000，不声明REA反编译。

## 原版结果

| 动作 | 原版入口 | 自然调用 | 私有边界调用 | 动画时钟流中的调用 |
| --- | --- | ---: | ---: | ---: |
| `33` | `1bab2` | 16 | 336 | 32 |
| `34` | `1bad6` | 0 | 540 | 6 |
| `35` | `1bafe` | 44 | 540 | 180 |
| `36` | `1bbbe` | 71 | 540 | 284 |
| `38` | `1bc92` | 214 | 820 | 2,284 |
| 合计 | | 345 | 2,776 | 2,786 |

普通输入 `source6-next-inputs.json` 的 MAIN 自然调用为 0。追加合法 `fighter6-special-inputs.json` 后，独立采集的普通和特殊技流程各 3,602 个视频步均与未挂观察器的原版参考逐步保持完整 RAM／画面一致，新增 345 次完整 pre-tail 调用。追加期间 Go、四项测试和采集脚本均未改变。

自然调用中，`33` 在视频帧 2,160 至 2,175 返回空中 `02/07`，`36` 在 2,000 至 2,070 返回站立。`35` 的调用分为 1,920 至 1,934 和 1,951 至 1,979 两段，合计 44 次并返回站立；中间 16 个视频帧没有 MAIN 入口，不能直接用保存动作状态的视频帧数替代阶段调用数。`38` 在 3,389 至 3,602 共 214 次调用，最后仍是 `38/AA`；本组件只证明这段自然活动片段，尚未证明自然涡旋完整退出。`34` 在本次真人输入中没有自然样本，仍保留私有边界及真实动画时钟证据。

连续流程共 20 条，每条只提供一次初始状态，之后执行原版 `19c96` 动画时钟与当前动作。只从当前动画集、姿势和帧索引选择不可变原版记录，刷新 `Duration/Flags`；没有回灌期望中间状态。空池和满池各覆盖上升、下落、地面／空中短冲与长冲，以及涡旋超时、早取消、晚取消和方向输入。终点为 6 条空中 `02/07`、8 条站立或落地、6 条公共恢复 `0c/2b`。这些流程未执行公共尾处理、形状提交、子对象或 FX 生命周期。

每次比对父对象 256 字节、三个子对象 768 字节、FX 池 2,048 字节及游标，并核对逻辑字段、声音请求、队列和 RNG。私有调用固定使用 `A5=FFFF8400`、`SP=FFFFFF00`；除父／子、FX、声音／游标及 `FEC0..FF04` 栈区域外，审计全部 64 KiB RAM，未发现未解释写入。调用返回时 SP 一致，检查点恢复包含 refresh cycles。MAIN RNG 消耗为 0。

## 已确认的动作语义

| 动作 | 确认行为 |
| --- | --- |
| `33` | 先 `Y += VY`，再对完整 `AlternateVelocity` 的高 word 加一并保留低 word；只有新高 word 恰为 16 时进入无条件空中恢复。 |
| `34` | 仅按旧 VY 移动，按有符号高 word 判断着地；先夹到完整 Ground，再执行落地并清 ExtraJump。 |
| `35/36` | 帧 4／8 开始常量位移；快段为 `40000`，末段递减为 `20000/10000`，不覆盖存储 VX。起始帧先移动，再按 contact.bit0 跳帧；仅完成值 1 触发恢复。 |
| `38` | `AA` 姿势优先于完成标志处理；StunTicks 按 word 回绕，方向加速保留一次阈值超调，Right 优先 Left。`A9` 先算术半速再移动；完成后按有符号 StunTicks 的 128 门槛选择站立或公共恢复。 |

`StunTicks` 是已有 `+84.word` 的唯一所有者；未新增涡旋计数。公共 `18682` 由 support 文件唯一实现，只恢复动作 `0c`、姿势 `2b`、阶段及 `Auxiliary7E←Auxiliary80`，保留重力、速度、ExtraJump 和 StunTicks。`0e/0f/11/13/37` 通过 support API 委派，其独立验证记录见 `FIGHTER6_SUPPORT_MOTION_VERIFICATION.md`。

## Go 验证与复跑

测试和失败场景先于实现，实际缺符号 red 已保留；最初失败场景全文及 SHA 见 `testdata/fighter6-motion-test-first.json`。冻结测试 SHA 为 `92f5bec89c9223d9ee18f01bc64d1816527ebe9d675c6ba770a23a2dd60024a2`。MAIN 实现 SHA 为 `334b3d5cdc4e6d61dd1a62c8b664ad106fd3bca26ab3aa4625105eca5dd07ca1`。

四项测试覆盖原版调用、真实时钟连续流程、word 与恢复语义、诊断错误原子回滚。首次可执行检查全部通过，未修改规则或冻结断言：原生竞态检查 10.362 秒，Node-WebAssembly 2.781 秒，原始绿色日志保存在 `fighter6-motion-ordinary-native.jsonl` 和 `fighter6-motion-ordinary-wasm.jsonl`。追加自然样本后，同四项冻结测试再次全部通过：原生竞态检查 11.600 秒，Node-WebAssembly 2.998 秒。此前一次因 support 定义尚未到位产生的编译失败单独保留，未作为绿色结果。

在工作树根目录使用以下命令，缓存和临时文件均留在工作树内：

```sh
export GOCACHE="$PWD/.local/go-cache"
export GOMODCACHE="$PWD/.local/go-mod"
export GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp"
export TMPDIR="$PWD/.local/md-yuyuhakusho/tmp"
go test -race -json -count=1 -run '^TestFighter6MotionMain' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm go test -json -count=1 -run '^TestFighter6MotionMain' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter6_motion.py \
  --out .local/md-yuyuhakusho/fighter6-motion-recapture
```

原版复跑使用现有本地 ROM、Genesis Plus GX 对照核心和 hook；SHA 及完整输入／参考绑定见三个夹具 manifest。最终来源、夹具和测试日志校验值集中在 `testdata/fighter6-motion-validation.json`。
