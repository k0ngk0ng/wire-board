# Source5 投射物资产审计

本审计验证仙水子对象回调 `1414e`、`143e4`、`146ba` 及已保存的停顿入口 `142b4`、`1458a` 所需的素材覆盖。范围包括原 CPU 逐次保存的实际 draw 请求、`fighter5-projectile-data.json` 的全部 typed AN 帧，以及 `1479c` 的九项 follower 小表。ROM 仅作为只读开发来源；本审计不导出 tile 或 ROM，不修改共享 catalog、规则 bundle、运行时代码或 UI。

在编写审计脚本前，固定以下失败场景：

1. 选定 ROM、catalog、原 CPU 夹具或静态证据不符合其 SHA-256 绑定，gzip 损坏，或者输入 schema 不兼容。
2. typed AN 键重复、缺失或增加了未解释的键；十六进制 pose `0x14..0x19` 被误作十进制 `14..19`；动画指针、8 字节 record、shape directory、shape 数据、sound 映射或 typed 字段与 ROM 不一致。
3. typed AN 的主体／子对象资源映射与保存的真实对象字段不符；33 个实读键中有键缺少捕获来源。
4. follower 表的地址、九项 type／delay、参数乘 2 的选择或终止项与 ROM 不符；有效表项所需的 graphic key 缺失。
5. `draw=true` 没有完整请求，非 draw 调用携带请求，或者请求与 callback 退出时对象的真实字段不同。
6. 有效生成的 GraphicsSet／TileBank／Parameter 没有精确 catalog 项，依赖 `PreloadedOnly`，或者缺少 transfer／piece。暂停的 stage-two 脏初态请求必须逐项单列并证明完整保留，不计入素材覆盖。未暂停 follower 在计时器尚未归零时仍会保留脏参数并执行投影，须另列为「仅资源键保留」，不得混作有效生成键或声称整个请求不变。
7. graphics directory、transfer descriptor、sprite piece 或保存的原始 oracle payload 与 ROM 不符；目录或终止边界越界。
8. transfer 长度、tile ID 或 tile 原始字节不符；piece 引用了本帧未分配的 slot；非透明源图被透明数据替换。真实透明初始化帧按来源验证，不使用 fallback。
9. 有效子对象资源键没有原 CPU draw 消费者，主体 typed AN 资源键没有已保存的 AN 提交消费者，或者 catalog 在审计期间发生变化。

审计通过，`missing_assets` 为空。现有 `world-sprites` catalog 可直接复用，共覆盖 24 个 GraphicsSet／TileBank／Parameter 精确键和 513 个去重 indexed tile。每个键均有自身的 transfer 和 piece，未使用 `PreloadedOnly`。共 27 项 transfer、158 个 piece 的目录、原始 payload、slot 分配及 tile 字节全部与选定 ROM 一致；JSON 逐键保留来源地址、摘要、tile ID 和 piece 所需 slot。

| 消费者 | GraphicsSet | TileBank | Parameter |
| --- | --- | --- | --- |
| 三种子对象及停顿入口 | `0xd8f1a` | `0x240000` | `0, 2, 4, 6, 8, 10, 12, 14, 16` |
| source5 主体的 typed AN 提交 | `0xc97aa` | `0x180000` | `2, 72, 106, 200, 212, 216, 218, 220, 222, 224, 226, 228, 252, 254, 256` |

33 个 typed AN 帧全部核对了 ROM 的 pose 指针、8 字节 animation record、shape directory、shape 原始数据、sound 映射及全部 typed 字段。共享 AN／shape `0xd92ca`／`0xd9a3a` 的 16 帧使用十六进制 pose `0x14..0x19`，对应十进制 `20..25`。主体 AN／shape `0xca6da`／`0xcb38a` 的其余 17 帧使用 pose `0x00`、`0x07`、`0xa0`、`0xa3`、`0xac`。每个键均有保存的 AN 提交消费者；本审计没有把主体 AN 提交当作完整主体 draw 观察。

`1479c` 的九项表与 ROM 一致：前八项为 type `1..8`、delay `4`，经原指令乘 2 后选择 Parameter `2..16`；末项为 `0, 0`，先清除对象，不提交 draw。九项均有原 CPU 选择证据。Parameter `0` 另作为初始化后暂停时的真实透明 ROM 帧验证；透明性来自精确源 tile。

| 原 CPU 夹具 | 调用 | draw 请求 | 覆盖请求 | 暂停／慢速脏请求 | 未暂停 follower 脏请求 |
| --- | ---: | ---: | ---: | ---: | ---: |
| oracle | 191 | 179 | 179 | 0 | 0 |
| boundaries | 6,363 | 4,966 | 3,433 | 1,506 | 27 |
| sequences | 3,596（16 流） | 936 | 936 | 0 | 0 |
| 合计 | 10,150 | 6,081 | 4,548 | 1,506 | 27 |

1,506 次暂停／慢速 stage-two 脏请求的完整 draw 与调用前一致，按条目保存在 `retained_dirty_paused_requests`。另 27 次未暂停 follower 的计时器尚未触发换帧，只保留资源键并执行跟随／投影；`0→255`、`2→1`、`255→254` 各 9 次，按条目保存在 `retained_dirty_unpaused_follower_requests`。这 1,533 次请求全部标记 `asset_coverage: false`。自然 oracle 的全部 179 次 draw 均有精确资源，其中实际出现了八个非初始化子对象 Parameter `2..16`。

从 worktree 根目录复跑：

```sh
python3 -B scripts/yuyuhakusho/audit_fighter5_projectile_assets.py \
  --out games/yuyuhakusho/verification/fighter5-projectile-assets.json
python3 -B scripts/yuyuhakusho/audit_fighter5_projectile_assets.py \
  --out .local/md-yuyuhakusho/fighter5-assets-repeat.json
cmp games/yuyuhakusho/verification/fighter5-projectile-assets.json \
  .local/md-yuyuhakusho/fighter5-assets-repeat.json
shasum -a 256 games/yuyuhakusho/verification/fighter5-projectile-assets.json
```

最终脚本由两个独立 Python 进程连续执行，报告逐字节相同，大小均为 875,733 字节。报告绑定脚本和 26 个输入文件的 SHA-256，并在审计结束前复核输入未变。

- 报告 SHA-256：`48027e046c5de3b7fa2f56be941b82cee734469891637059631d7fc0a4eb26b2`
- 脚本 SHA-256：`6272d8955b1c74886a5f897787804e83b03a367479666f6c41d14ec2bcbf91e5`
- catalog gzip SHA-256：`200e28e775a809de89c19df0dfef314d5697224fcb6a145853beb7344b5833ef`，审计前后不变。
- 选定 ROM SHA-256：`c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06`。

静态说明来自已保存的直接 Ghidra 12.1.4 有界反汇编及其 manifest，原 CPU 行为来自三组冻结夹具。REA 不支持本任务的 big-endian raw68000；本脚本读取证据和源字节，不运行原 CPU，也不声称 REA 反编译。

本结果证明保存范围内的资源覆盖。规则 bundle 接入、完整主体 draw、后续共享特效 draw、presentation 的 cache／skip／全局 slot 持久化、调色板时序、自然整帧合成及 scanline 行为仍由对应世界／呈现验收负责；未观察输入之外的可达性不在本结果中声称完成。

本 owner 仅新增审计脚本、JSON 报告和本说明，未修改 production、共享 catalog、其他 data、规则 bundle、UI、`PROGRESS` 或 Go 测试，未提交 commit。已清理本次 `.local/md-yuyuhakusho/` 下的两份报告副本，共 1,751,298 字节；`-B` 执行没有生成 Python 缓存。ROM、共享缓存、工具和其他 owner 的文件均保留。
