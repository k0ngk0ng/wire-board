# 飞影龙头、身体段与强化验证

新增独立规则 `StepFighter4DragonHead`（13BB6）、`StepFighter4DragonTrail`（13E9A）和 `StepFighter4Boost`（13FC8）。运行时只消费 typed state 与只读语义数据，不携带 ROM、CPU core、指令解释器或 RAM 镜像。`Fighter4DragonSample/History` 是六个实际使用历史 plane；head FE、world A038/history 的 canonical 接入由 Root 负责。

失败清单在 `FIGHTER4_DRAGON_FAILURES.md`。先采原 CPU，再冻结完整测试与夹具，并真实保存三函数缺失的编译 red；其后才写函数行为。先行测试 SHA-256 为 `aaa6c8ee815b7e10f5799814874106ca9c02fd68a3283a2ec03619d7a5c90615`，未在实现后修改。绑定见 `testdata/fighter4-projectile-test-first.json`；red 为 `testdata/fighter4-projectile-red.jsonl`。

原 CPU 测试数据：

- 2607 个完整脏初态回调：head923、trail1098、boost586。
- 16 条单初态流程，共8596次调用：真实原 clock16、creator16、head648、独立身体段2276、boost5640。只注入明确记录的 pause/contact 外部输入，不逐步回填期望、时钟或动画记录。身体段按实际 FX 地址顺序独立执行，新分配高地址段可在同一轮被执行；保留旧 bank 的未覆盖样本。
- 三 callback 比较完整父三子、16个FX槽、2048字节历史、shape、声音与RNG；每次1580保存当时完整 `WorldDraw` 和序号。每回调保存全64KiB最终delta，私有栈明确划定；规则外自然IRQ delta按byte串联，队列消费者单列。未用历史空洞、Status低byte及所有未写字段保持。
- 真实 signed-word 查表地址覆盖索引 `-32..1311`，因此只读 Curve 固定 `FirstIndex=-32`、1344词。保留相邻前缀/后缀实际内容，不按角度wrap，不改用 signed-byte Arena 表。9项公共AN语义记录完整：十进制pose14..18各一帧，pose19四帧。

原版身份为 ROM SHA-256 `c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06`，hook core SHA-256 `e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306`。采集脚本为 `scripts/yuyuhakusho/capture_fighter4_projectile.py`，使用已有 `ReferenceSession`，恢复同时设置 `restore_refresh_cycles=True`。普通合法输入唯一原core3602步control完全复用 selector 提供的 `fighter4-attacks-reference-frames.json`，SHA-256 `891f1e493f3e3d8bd34e6bf4789739364c6a3f3dc3d318d846d3b797f581d658`；hook重放每步RAM/video相等。普通输入自然调用本scope为0，不能写成龙自然生命周期覆盖。

后续只追加特殊合法物理输入的自然oracle，未改生产Go、测试、边界、连续流程、9AN/Curve或原capture脚本。输入 `source4-special-discovery-inputs.json` SHA-256 `5ba6b13d2bb0b285c4307981a5e903d75a7936de3d2632a9f1bf449189650424`，复用唯一原core3602步 `fighter4-attacks-special-reference-frames.json` SHA-256 `666c2a9fff6c47b3b0540275beae5a7a1c0f6ada1e1d690d6a67d1596617b22e`。新自然观察器 `capture_fighter4_dragon_natural.py` 捕获第3481/3482帧共2个13BB6，全部3602步RAM/video一致，无未解释修改；该次没有观察到13E9A或13FC8。旧普通空oracle的原始SHA `e8e29c6e9cae99c14890f1df17a2e4c8ce850deb26f33296907aa39072bbb1b5`、gzip SHA与旧manifest完整快照保存于 `testdata/fighter4-projectile-natural-append.json` 的递归版本链。

双真人练习场输入 `source4-special-discovery-duo-inputs.json` SHA-256 `b303ca8622f90747052902f81b2b7f2bdde39f59e47feb6f36a6bf8287a6ac34` 再追加1590个真实完整回调。它只重放已冻结的唯一5502步control `fighter4-attacks-duo-reference-frames.json`（SHA-256 `9f282f53a016d184d45052b30eaf8c482d4b568132f7979f7eebff4a7faa5917`），5500入口步和2步drain全部RAM/video一致。飞影家族9000、child9100、channel1，身体段由实际Owner高word9100归属家族9000：

| 回调 | 帧范围 | 次数 | Draw / clear |
| --- | --- | --- | --- |
| 龙头9100 | 3479..3589 | 111 | 110 / 1 |
| 七个独立身体段9700..9A00 | 3482..3616 | 777 | 770 / 7 |
| 强化9100 | 3598..4299 | 702 | 701 / 1 |

自然强化能量高word从223耗至−1，第4299帧完整父构造将MovementSource从15946改回12578并删除child。每个parent/family、FX、history、声音、shape及Draw仍由同一先行断言精确比较；自然曲线333次读取，实际索引−32..1241。该输入内没有回调重叠IRQ、队列消费者、RNG或未解释区外修改。包含之前两次head后，最终自然oracle共1592条：head113、trail777、boost702。旧两条oracle SHA及旧append ledger完整快照与最初空oracle一起保留，未混用两个特殊输入control。

自然三API不接受MAIN的JumpParameters，因此新增自然snapshot将这项输入上下文留空；真实当前表的source/data保留在 `movement_parameters_observed`。父+90及其他canonical bytes仍完整比较，不能以只读表重选造成的验证适配器差异替代或掩盖实际父状态变化。

已通过同一冻结断言的 native `-race`、Node-Wasm `go_js_wasm_exec` 和 `go vet`。双真人自然数据追加后再次通过 native（53.635秒）与Wasm（13.925秒）；未改原有断言。可重复命令和产物SHA见 `testdata/fighter4-projectile-validation.json`；完整原生/Wasm JSONL保留。重采需限定输出在当前worktree内，例如：

```sh
PYTHONDONTWRITEBYTECODE=1 TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
  python3 scripts/yuyuhakusho/capture_fighter4_projectile.py \
  --out .local/md-yuyuhakusho/fighter4-projectile-capture
```

范围止于每回调的1580请求或3556清除完成；硬件sprite消费者和现有通用149AC特效后续不是这三个函数的执行范围。未修改共享world/schema/bundle/PROGRESS，无commit。

已清理本worker私有capture目录的重复fixture和首次实现中间日志，验收fixture、原始red、最终native/Wasm日志和capture计数日志均保留在testdata；未删除共享Go缓存、其他角色或活动工作目录。精确数量与字节见 `testdata/fighter4-projectile-cleanup.json`。

自然追加的私有目录在保留oracle、旧manifest快照、最终日志后也已清理，范围见 `testdata/fighter4-projectile-natural-cleanup.json`。

双真人追加的私有目录也已在保留相同验收产物后清理，范围见 `testdata/fighter4-projectile-duo-cleanup.json`。
