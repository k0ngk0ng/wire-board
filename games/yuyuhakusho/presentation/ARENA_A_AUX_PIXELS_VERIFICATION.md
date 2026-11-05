# 场地 a 六条附加精灵的最终像素验收

20 个自然原样本的 1,368,854 个未排除像素全部匹配原视频。场地 a 的附加精灵实际贡献 29,063 个可见像素，其中 28,224 个与背景颜色不同；每帧均有至少 481 个不同于背景的附加精灵像素。Native race 与 Node／Wasm 的最终报告逐 byte 相同，摘要为 `fcc64e4405bf5864828288739e12e22c001a8da6982f8376432bb78d6469aba2`。

机器索引为 [arena-a-aux-pixel-validation.json](testdata/arena-a-aux-pixel-validation.json)。本次只增加独立验收脚本、命令与夹具，没有修改共享生产实现，也没有再次运行模拟内核或生成第二份原版控制组。

## 对照路径与 mask 边界

离线 producer 从已保存的 20 个原样本读取原视频和实际显示输入，并从完整 natural callback 记录取得逐次 CPU raw。每帧六条 raw 的实际索引、坐标、尺寸和图块必须逐条等于原 SAT 链最后六项。独立排序审计见 [arena-a-aux-order-audit.json](../data/arena-a-aux-order-audit.json)。本批 20 个样本的第一条 raw 预算为 `16..32`；其他预算状态由独立呈现测试覆盖。

producer 直接读取原 VRAM 计算 A／B／窗口的透明性与优先级，确认原视频中附加精灵的实际贡献。非 A 精灵的 mask 只取原 SAT 中其余项的实际非零图块像素，合计排除 64,746 个像素；不排除矩形，也不把全部附加精灵一起排除。非 A 精灵即使被高优先级背景遮挡，其非零像素仍排除。重叠处沿用原精灵顺序，六条 A 精灵位于末尾。

Go 验收命令从严格的选中场地背景、独立 48 块 aux bank 和 callback 时的六条 raw 开始，依次调用生产 `ComposeArenaASprite`、`RasterizeSpritesForArena`、`RasterizeArena` 和 `CompositeArenaSprites`。逐像素核对颜色，并逐像素核对最终 owner 是否为附加精灵。最终 29,063 个可见附加精灵像素的位置与原参考完全一致，排除了「颜色偶然相同」或「把 A 全部 mask 掉」的假通过。

新增夹具压缩后为 318,272 bytes，SHA-256 为 `6d64523e849ea5f22432fad8c110c45f67ac0a795175b9e8189d710361339b28`。夹具保存六条语义 raw、两个 bitmask 和未排除的原 RGB565 期望，复用已保存的色盘、逐行滚动和窗口 indexed 语义夹具，没有重复存 RAM／VRAM。两次导出的 JSON、gzip 与 manifest 逐 byte 相同。

## 失败验证

[失败场景](ARENA_A_AUX_PIXELS_FAILURES.md) 先于本验收命令定义。去掉附加精灵的路径首先留下真实 red：28,224 个颜色差异、29,063 个可见性差异。冻结的 producer 和 Go harness 在 red 与最终验收之间没有改变。

Native 与 Wasm 两端各验证以下六项拒绝：

| 破坏 | 结果 |
| --- | --- |
| 完全不提交 aux | 28,224 个颜色差异、29,063 个可见性差异 |
| 误走不带 aux bank 的旧精灵入口 | 拒绝缺失的 aux 图块 |
| 六条 X 坐标各增加 1 | 14,089 个颜色差异 |
| 三组局部图块循环错用 | 23,081 个颜色差异 |
| 将非 A mask 扩大到全画面 | 拒绝覆盖原可见 aux 像素的 mask |
| 每帧翻转 1 个原期望像素 | 共 20 个颜色差异 |

`go vet` 通过。详细负向命令、退出码和报告摘要保留在 [arena-a-aux-pixel-negatives.json](testdata/arena-a-aux-pixel-negatives.json)。

## 复跑与范围

在 worktree 根目录运行完整生产呈现对照：

```sh
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go run -race ./cmd/verify-yuyu-arena-aux
```

使用同样命令增加 `-mode no-aux` 可复现真实失败。其他负向模式为 `wrong-bank`、`bad-x`、`bad-tile`、`mask-all` 和 `bad-expected`；这些命令应返回非零退出码。

Node／Wasm 的精确最小环境和命令保留在 [arena-a-aux-pixel-wasm-command.json](testdata/arena-a-aux-pixel-wasm-command.json)，可按同目录 [呈现入口验收](ARENA_PROFILES_VERIFICATION.md) 中的方法读取该记录复跑。重新从已保存的原始来源生成夹具：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/build_arena_a_aux_pixel_fixture.py \
  --out .local/md-yuyuhakusho/arena-a-aux-pixel-replay
```

窗口仍使用原 indexed 语义参考，因此本批验证背景／窗口与六条 A 精灵的最终合成，不新增独立 HUD 规则事件的覆盖。其他人物精灵、完整世界调度、扫描线溢出、未采样输入、浏览器与联机行为不在本批结论内。
