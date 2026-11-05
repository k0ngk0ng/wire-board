# 来源 7 子对象验证

`fighter7_children.go` 独立实现原版 `136e6`、`1391e` 和 `15094`。11,323 个脏状态回调、1,378 个自然回调和 8 条单初态连续流已通过完整 native race／Wasm 比较，`go vet` 通过。世界调度与平台集成仍由 Root 验收，本记录只覆盖这些子回调及实际派生 FX。

## 证据与先行测试

[失败场景](../../../docs/research/md-yuyuhakusho/FIGHTER7_CHILDREN_FAILURES.md)先于采集器、测试和生产文件建立。直接 Ghidra 68000 正文位于 [入口](../../../docs/research/md-yuyuhakusho/static/fighter7-children-focused.txt)、[跳表和派生链](../../../docs/research/md-yuyuhakusho/static/fighter7-children-chain.txt)及 [回血 FX helper](../../../docs/research/md-yuyuhakusho/static/fighter7-children-helper-full.txt)。REA 不支持 raw 68000；静态解码使用已安装的 Ghidra 12.1.4 与 `68000:BE:32:default`，没有将静态结果称为已执行行为。

唯一未观察普通控制组由 Root 提供，固定输入为 `source7-next-inputs.json`，原 core 的 3602 帧输出为 `fighter7-attacks-reference-frames.json`。采集器使用 hook core 回放，所有帧 RAM／画面摘要与控制组一致。第 2400 帧实际校验 P1 `8400` 的来源 tag 为 7、callback 为 `12668`、Size 为 `100`，随后才派生私有状态；每次恢复都使用 `restore_refresh_cycles=True`。

后续自然采样复用 Root 的 `source7-special-discovery-inputs.json` 和 `fighter7-attacks-special-reference-frames.json`，完整 5502 帧 RAM／画面摘要一致，只接受视频标签 5500 及以前的入口，另留 2 帧收完调用。普通控制组没有子回调，特殊控制组实际覆盖三个 callback 和可归属的派生 FX。采集器只将固定结束标签从 3600 改为「控制组长度减 2」以支持这一控制组，冻结断言与生产实现没有改动；私有证据的原始采集器全文另存为 `capture_fighter7_children_private_frozen.py`，其 SHA 与先行记录完全一致。

原 CPU 采集完成后冻结完整测试，SHA-256 为 `9e2832aeb24902e3cf742347c228aab60f3083ab02b0a33744a2df8b867cd352`。`fighter7-children-test-first-freeze.json` 与 `fighter7-children-own-symbol-red.log` 保留生产文件不存在时的真实编译失败，明确缺少 `StepFighter7FirstChild`、`StepFighter7SecondChild`、`StepFighter7Pan`。生产实现没有改变冻结测试。

| 证据 | 记录／调用 | 已验证内容 |
| --- | ---: | --- |
| 私有脏状态 | 11,323／11,323 | first 4,321、second 6,497、pan 505；初始化、暂停／慢速、动画 byte 时钟、动作分支、命中停顿、signed word／long 溢出、回血、分配失败与清除前缀 |
| 单初态连续流 | 8／7,948 | 两种父动作、双向、暂停；clock 1,150、creator 1,150、record 1,150、first 1,032、second 1,032、pan 1,026、派生 FX 1,408，最终子槽及 FX 全清除 |
| 普通及特殊自然控制组 | 1,378／1,378 | 普通 3602 帧、特殊 5502 帧全部匹配；first 209、second 209、pan 256、派生 FX 704 |

采集器保留完整 64 KiB RAM 差分、四个父体的原始槽、IRQ 的 24／32 位实际入口与返回、`37c56/37cac` 声音清槽以及每次 `1580` 当时的 `WorldDraw`。私有回调没有未知外部写、随机数调用或未完成调用。P1 family codec 只拥有本家四槽；其他三家只合并明确恢复的 Health／HitStopTicks 字段，原始槽其余字节逐字节比较。

自然 pan 从视频标签 4053 连续执行至 4308，共 256 次，末次清除 256 bytes，`F570` 为零。自然回血分支和 8 FX helper 在标签 1935、2112 实际执行，但这两次四父原始槽均未变化，因此只证明回血动作可达，不声称自然生命值增加。私有 1,809 次回血分支覆盖实际生命变化、距离、空槽、负生命、上限和 signed word 溢出。自然回调没有 IRQ、外部 RAM 变更或随机数调用，23 次清除和 860 次即时 draw 均逐项比较。

## 原版行为与字段

两个攻击子对象的 stage 为零时初始化图形 `d8f1a`、tile bank `240000`、AN `d92ca`、shape `d9a3a`，先设置 Health 为 255，再判断暂停／慢速。`136e6` 的初始 pose 为 `3e`，`1391e` 为 `3c`。两者的主入口都直接检查 `+69`，因此 `137f4`／`13a08` 是已证明可达的命中停顿路径。

`136e6` 正常路径先执行 `19cc4` 的残余位移，再推进动画时钟。共同尾段在 ground clamp 之前，将 Facing 写入下一 sibling 的 `Aux48` 最高 byte，并将完整 X／Y 写入 sibling 的 `+10/+14`。写入不以 sibling Size 为条件。

`1391e` 的动作 2 使用完整 long 跟随目标，X 增量为 `(targetX-X)>>2`，Y 增量为 `(targetY-Y)>>3`；减法和加法保留 32 位溢出。按住 B 时，`20666..208d6` 独立分配 8 个 `149ac`／type `24` FX，再遍历四个固定父体回血。回血不区分队伍；只排除空槽、负 Health、超出 X 24 或 Y 32 距离的对象。距离、绝对值、加 16 和 clamp 223 均保留原版 signed word 语义，包括 `-32768` 取负和加法溢出。

| 临时 view | canonical 字段／偏移 | 合并范围 |
| --- | --- | --- |
| 子体输入、动画、图形、清除 | `WorldObject` 既有字段 | 当前子对象，无 RAM 镜像 |
| Parent | `ObjectID`、父体 Pose `+5f` | 只读 |
| First Link | sibling VX `+10`、VY `+14`、Aux48 `+48` | 只写这三个完整 long；Aux48 保留低 24 位 |
| HealingTarget | 四父 ID、Size `+04`、Health `+78`、X `+40`、Y `+44` | 只合并 Health |
| EffectExtra | FX VY `+14`、Delay `+5a` | 与 `EffectPool` 字段互不重叠 |
| PanAmplitude | `World.Clock.PanAmplitude`／`f570.w` | 全局 word |

`15094` 没有 draw。初始化把 canonical VY 的高 word 写为 `1000`，保留低 word；非暂停／非慢速调用将高 word 减 `10` 并写入 `f570.w`，减至零后执行对象清除。三个 callback 都没有证明新的 `+fe` 语义；实现不引入第二个 FE owner。

## 运行方式与集成边界

采集命令保留在 [采集器](../../../scripts/yuyuhakusho/capture_fighter7_children.py)。以下命令仅使用已经存在的两个控制组，重新生成自然证据并保留私有 fixture，输出限制在本 worktree：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter7_children.py \
  --out games/yuyuhakusho/engine/testdata \
  --baseline docs/research/md-yuyuhakusho/source7-next-inputs.json \
  games/yuyuhakusho/engine/testdata/fighter7-attacks-reference-frames.json \
  games/yuyuhakusho/engine/testdata/fighter7-attacks-reference-manifest.json \
  --baseline docs/research/md-yuyuhakusho/source7-special-discovery-inputs.json \
  games/yuyuhakusho/engine/testdata/fighter7-attacks-special-reference-frames.json \
  games/yuyuhakusho/engine/testdata/fighter7-attacks-special-reference-manifest.json \
  --natural-only
```

私有证据使用 [冻结采集器](../../../scripts/yuyuhakusho/capture_fighter7_children_private_frozen.py)和普通控制组重放到新的 worktree 内输出目录，再与保留的 `boundaries`／`sequences` SHA 比较；该脚本会同时生成零自然样本记录，因此复现私有证据时不直接覆盖最终自然 fixture。

Go 验证使用 worktree 内的 `GOCACHE`、`GOMODCACHE`、`GOTMPDIR` 和 `TMPDIR`。完整 native 门禁为 `go test -race ./games/yuyuhakusho/engine -run '^TestFighter7Children' -count=1 -v`，71.953 秒通过；完整 Wasm 门禁使用相同测试前缀，20.183 秒通过。`fighter7-children-validation.json` 保存生产文件、冻结测试、fixture 和日志的 SHA。自然样本补齐前的中间验证只选择私有范围，最终结果包含完整自然门槛。

[独立语义数据](../data/fighter7-children-rules.json)完整覆盖子对象 pose `3c..45` 的 94 条 AN／shape 记录，23,744 bytes，SHA-256 为 `72d54890df9bbb66b7a70e0f026e343dc749d971b9450979d103324e65d16aec`；现有 catalog 没有重复记录。数据不包含 RAM、指令或原 CPU 输入。[导出脚本](../../../scripts/yuyuhakusho/export_fighter7_children_data.py)与来源 manifest 可重复生成并校验这些记录。bundle 合并、世界调度、四父健康原子合并、平台及最终验收由 Root 完成，本 scope 未修改 shared helper、world、bundle 或 UI。
