# 原版场地菜单控制组

本任务只记录原 ROM 的合法物理输入与原版冷启动结果，不使用内存、随机种子、初始化请求或函数注入选择场地。

## 捕获前的失败场景

- 输入位、端口或控制器注册不一致，造成菜单选择偏移。
- 胜拔战路径没有场地菜单，却把角色菜单方向键当作选场地。
- 只验证菜单中的 `A060`，没有验证比赛里的场地主对象与原画面。
- 连续切换过快，仍在前一次淡出／重载过程中，方向键或确认键没有被处理。
- 使用了带观察器的核心、恢复快照或修改 RAM 的运行，却把它称为冷启动原版控制组。
- 输入、原 ROM、原核心、捕获脚本与输出文件未绑定 SHA，无法独立复验。
- 第二次全新进程的原版重放与发布的逐帧 RAM／视频哈希不同。

## 已观察的合法入口

默认胜拔战没有经过场地选择。练习试合路径经过报名、角色选择、场地选择，再进入比赛。七场正式控制组均使用相同的物理入口，按键端口为 `0`，控制器注册为 `[513, 513]`，未启用 Team Player。

冷启动探索中，`1720..1722` 的物理 Right 使 `A060.word` 在第 1721 帧变为 `2`。第 1850 帧的原画面是场地选择「霊界」，第 2800 帧为该场地的真实比赛。

原版场地菜单对象为 `DB68`，输入子对象为 `DCC8`。练习试合主容器通过 `A10E` 将 `DB68` 写入 `8040`。`DD2C` 写入 `A060`，方向键使其在 `0,2,4,6,8,A,C,E` 间循环；`DCB8` 将选择保存到 `FF28`，下次菜单初始化在 `DBA4` 读取。见[有界静态清单](static/arena-menu-writes.txt)与[工具身份](static/arena-menu-static-manifest.json)。这些静态地址与运行时菜单观察互相核对，不把静态反汇编称为运行观察。

## 正式控制组

正式控制组由 [capture_arena_menu_reference.py](../../../scripts/yuyuhakusho/capture_arena_menu_reference.py) 唯一产生。每场发布一个 `original3602` 帧数组及 manifest；复验进程只逐帧比较既有数组，不再发布第二份控制组。场地 `0` 复用既有基线，本任务未为 `0` 新建控制组。

菜单原名直接从第 2110 帧原图读取。方向键从第 1720 帧开始，每次按住 3 帧，间隔 100 帧；第 2120 帧按 B 确认。每场第 2377 帧均进入真实 `F2A4` 比赛管理器。第 2800 帧校验主容器 `A01E`、主阶段 `32`、对应场地 callback 与两个非空角色对象。

| ID（十六进制） | 原版菜单名称 | 菜单方向输入 | 场地 callback | 比赛管理器阶段 `14` 首帧 | 菜单／实战原图 |
| --- | --- | --- | --- | --- | --- |
| `2` | 霊界 | Right × 1 | `20AE0` | 2672 | [菜单](arena-2-selection.png)／[实战](arena-2-original.png) |
| `4` | 亜空間1 | Right × 2 | `20CB2` | 2668 | [菜单](arena-4-selection.png)／[实战](arena-4-original.png) |
| `6` | 魔界 | Right × 3 | `20E28` | 2672 | [菜单](arena-6-selection.png)／[实战](arena-6-original.png) |
| `8` | 洞窟 | Right × 4 | `20FB4` | 2673 | [菜单](arena-8-selection.png)／[实战](arena-8-original.png) |
| `a` | 魔性の森 | Left × 3 | `21126` | 2674 | [菜单](arena-a-selection.png)／[实战](arena-a-original.png) |
| `c` | 亜空間2 | Left × 2 | `212F8` | 2673 | [菜单](arena-c-selection.png)／[实战](arena-c-original.png) |
| `e` | 山頂 | Left × 1 | `21412` | 2676 | [菜单](arena-e-selection.png)／[实战](arena-e-original.png) |

每场具有以下文件，`<id>` 使用表中的小写十六进制编号：

- `docs/research/md-yuyuhakusho/arena-<id>-inputs.json`：完整物理输入。
- `docs/research/md-yuyuhakusho/arena-<id>-selection.json`：主容器、比赛管理器、`A000..A1FF` 原始 header、四个完整角色槽、菜单／比赛状态转换，以及 9 个采样帧的 RAM／视频哈希。
- `docs/research/md-yuyuhakusho/arena-<id>-coldproof.json`：第二个全新 Python 进程与唯一控制组逐帧比较的结果。
- `games/yuyuhakusho/engine/testdata/arena-<id>-reference-frames.json`：3602 个 `{frame,ram,video,dimensions}` 对象。
- `games/yuyuhakusho/engine/testdata/arena-<id>-reference-manifest.json`：原核心、ROM、输入、输出、捕获脚本与冷重放结果的 SHA 绑定；`entry_frame_limit=3600`，`drain_frames=2`，`observer_attached=false`。

七场共发布 25,214 帧。第二进程逐帧比较的 25,214 个 RAM 哈希、25,214 个视频哈希，以及 63 个完整状态快照全部一致。菜单和比赛共 14 张原图已人工查看。场地素材与独立规则的通过结果由各自的验收任务记录，本控制组只证明原版合法可达与重放一致。

## 复验

在工作区根目录运行：

```sh
python3 -B scripts/yuyuhakusho/capture_arena_menu_reference.py --arena 2 --verify
```

将 `2` 替换为其他表中编号可复验对应场地。复验读取既有输入与控制组，执行全新原版冷启动；任一帧的 RAM、视频或完整状态不一致时立即失败。复验不会覆盖原帧数组。

需要在干净工作区重新生成时，省略 `--verify`。生成器会拒绝替换已存在的输入、帧数组、manifest、selection 或 coldproof，避免同一范围出现多个生产来源。全部文件的交叉绑定检查见 [arena-menu-controls-validation.json](../../../games/yuyuhakusho/engine/testdata/arena-menu-controls-validation.json)。
