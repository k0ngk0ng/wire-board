# 八场地呈现入口验收

已恢复八个已知场地的严格素材选择入口、场地 a 的 48 块附加素材、选中场地的 HUD 资源事件和共同逐行显示方案。本包完整 20 项测试在 Native race 与 Node／Wasm 两端通过，分别用时 56.186 秒和 14.768 秒。验收索引见 [arena-profiles-validation.json](testdata/arena-profiles-validation.json)。这份记录只覆盖呈现入口，不代表世界调度、浏览器操作、完整玩法或联机已经验收。

## 入口和身份

| 入口 | 约束 |
| --- | --- |
| `DecodeArenaCatalogForArena(raw, arena)` | 按明确选择校验 ROM、schema、顶层身份、嵌套身份、资源清单、图块、平面、单元和色盘 |
| `ArenaCatalog.Identity()` | 返回注册身份的值拷贝，私有元数据不改变素材对象的 JSON 字段 |
| `DecodeArenaAAuxCatalog(raw)` | 只接受场地 `a`、callback `21258`、源 `0x297056`，输出恰好 48 个顺序局部图块 |
| `ApplyHUDWorldEventsForArena(state, events, cells, catalog, arena)` | 共用原 HUD 素材；资源事件必须匹配选中场地，失败保留调用前状态 |
| `ArenaRowsForArena(arena, backdrop, palette, horizontal, vertical)` | 只接受八场注册和 `0..63` 的逻辑背色索引，当前滚动、色盘由调用方提供 |

原 `DecodeArenaCatalog`、`ApplyHUDWorldEvents` 和 `DefaultArenaRows` 保留默认入口。HUD 素材本身仍绑定原 `21720` 来源，未放宽为任意资源。附加图块与背景图块分开保存，三组 4×4 图块的局部起点为 `0/16/32`，不以硬件图块号直接索引背景。

| 场地偏移 | 场景 callback | 资源清单 |
| --- | --- | --- |
| `0` | `20918` | `21720` |
| `2` | `20ae0` | `2175c` |
| `4` | `20cb2` | `21798` |
| `6` | `20e28` | `217cc` |
| `8` | `20fb4` | `21800` |
| `a` | `21126` | `21834` |
| `c` | `212f8` | `21870` |
| `e` | `21412` | `218a4` |

## 测试与原版证据

[失败场景](ARENA_PROFILES_FAILURES.md) 和测试先于实现冻结。保留缺少符号的编译失败，以及仅委托默认入口时的实际行为失败：错误选择被接受、七场资源被默认 decoder 拒绝、附加素材缺失、HUD 资源不符、背色未传递。最终测试 SHA-256 为 `48164ba34588f797b3fc51f87eb33d93a1cc9da2ecbe0814930a5497320c47a8`，实现后未改断言。

七场各 20 个原样本合计 140 帧、31,360 行。Go 独立背景绘制对照 9,535,530 个未被实际不透明精灵遮挡的原视频像素，差异为 0。可见行的完整色盘、行显示／窗口标志均匹配。ID4 的 48 个下方空白行在未显示的颜色 9～11 提前变相；本入口保持可见色盘和实际像素等价，不声称这些空白行的全部色盘等于帧末快照。

测试夹具从已经验收的原始参考做语义投影，不重新运行内核或生成第二份原版控制组。夹具仅保存当前色盘、逐行滚动、原窗口 indexed 参考、精灵排除 bitmask 和原视频期望摘要，不作为生产素材。两次转换后的 JSON、gzip 和 manifest 逐 byte 相同；gzip 为 370,686 bytes，SHA-256 为 `b9c135c02699e87b3a92c97ed8853fbe6afa6b0ca0faaf488172a3d9cdf78af8`。原完整 RAM／VRAM 和视频仍由 [素材来源说明](../../../docs/research/md-yuyuhakusho/ADDITIONAL_ARENA_ASSETS.md) 的独立采集证据保留。

本次还有每场 23 项格式／身份破坏、56 组场地交叉错误选择、56 组 HUD 交叉资源拒绝、10 项附加素材破坏及逻辑背色／色盘越界验证。旧默认素材、原 HUD 和原精灵呈现测试随本包一起回归，`go vet` 通过。独立精灵最终合成、世界接入和浏览器操作由后续整合验收覆盖。

## 复跑

在 worktree 根目录执行 Native race：

```sh
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -json -count=1 ./games/yuyuhakusho/presentation
```

Node／Wasm 的精确命令和最小环境保留在 [arena-profiles-wasm-command.json](testdata/arena-profiles-wasm-command.json)，在同一机器可直接复跑：

```sh
python3 - <<'PY'
import json, os, subprocess
from pathlib import Path
record = json.loads(Path('games/yuyuhakusho/presentation/testdata/arena-profiles-wasm-command.json').read_text())
environment = {key: os.environ[key] for key in ['HOME'] if key in os.environ}
environment.update(record['environment'])
raise SystemExit(subprocess.run(record['argv'], env=environment).returncode)
PY
```

重新投影原版夹具需要已经保存的七份原始参考及其 manifest：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/compact_additional_arena_presentation_reference.py \
  --out .local/md-yuyuhakusho/arena-presentation-replay
```

本次仅清理本组 377,469 bytes 的重复夹具和重复转换输出。原始来源、测试失败、最终日志、冻结记录与可复跑夹具均保留；没有提交或发布。
