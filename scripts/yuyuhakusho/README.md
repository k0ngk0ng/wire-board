# 《魔强统一战》原版取证与规则验收

先阅读 [失败场景与验收范围](REFERENCE_VALIDATION.md)。当前工具在 macOS arm64 验证；libretro 内部结构与固定的 Genesis Plus GX 提交绑定，不能换内核后直接沿用其 ABI。独立游戏代码不依赖这些工具。

## 独立本地对局

幻海、藏马、桑原、幽助、阵、户愚吕兄、酎、户愚吕弟／默认场地已有独立浏览器研究页，运行与复跑命令见 [LOCAL_PLAYER.md](../../docs/research/md-yuyuhakusho/LOCAL_PLAYER.md)。`serve_local_player.py` 只在 loopback 提供独立 Wasm 和索引素材；它不提供 ROM 或模拟器。

```sh
sh scripts/yuyuhakusho/verify_chu_sessions.sh both
sh scripts/yuyuhakusho/verify_chu_runtime.sh both
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_local_player.py
sh scripts/yuyuhakusho/verify_fighter_sessions.sh both
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_fighter_player.py
sh scripts/yuyuhakusho/verify_genkai_sessions.sh both
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_genkai_player.py
sh scripts/yuyuhakusho/verify_yusuke_sessions.sh both
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_yusuke_player.py
sh scripts/yuyuhakusho/verify_kuwabara_sessions.sh both
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_kuwabara_player.py
sh scripts/yuyuhakusho/verify_elder_sessions.sh both
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_elder_player.py
sh scripts/yuyuhakusho/verify_kurama_sessions.sh both
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_kurama_player.py
sh scripts/yuyuhakusho/verify_jin_sessions.sh both
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_jin_player.py
```

来源 8／10 的兄弟姓名已按原 HUD 放大字形订正，旧阶段文件名保留历史标签，当前姓名以 [CHARACTER_IDS.md](../../docs/research/md-yuyuhakusho/CHARACTER_IDS.md) 为准。

完成所需检查后，`summarize_independent_session.py` 核对原生／Wasm 报告、当前源码、测试日志和浏览器结果并生成绑定摘要；它本身不运行游戏或替代检查。

## 输入文件

- ROM：`.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin`，SHA-256 `c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06`。
- 原内核：`.local/md-yuyuhakusho/tools/genesis_plus_gx_libretro.dylib`，SHA-256 `1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da`。
- 原版输入序列和冷启动基线：`docs/research/md-yuyuhakusho/inputs.json`、`frame-hashes.json`。
- 内核源代码：`https://github.com/libretro/Genesis-Plus-GX/tree/60bee4c724859710ce05672239e5b601ad01382e`，本机位于 `.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/`。源码与许可仅供研究工具，不能把许可当作游戏素材发布许可。

## 原版冷启动与恢复

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/accept_reference.py \
  --out .local/md-yuyuhakusho/reference-acceptance
```

`reference.py` 只将 `m68k.refresh_cycles` 作为模拟器存档附加字段恢复，不写入原版 RAM 或游戏得分。报告分别列出未修正和修正路径；完整 RAM、视频与 CPU 寄存器必须逐帧比较，不能只看画面。

## 构建被动观察内核

下列目录均位于当前 worktree。`HOOK_CPU` 是上游已有构建选项，未修改其模拟器源代码。

```sh
set -e
MD_WORK_ROOT="$PWD"
mkdir -p .local/md-yuyuhakusho/tmp .local/md-yuyuhakusho/clang-cache
TMPDIR="$MD_WORK_ROOT/.local/md-yuyuhakusho/tmp" \
CLANG_MODULE_CACHE_PATH="$MD_WORK_ROOT/.local/md-yuyuhakusho/clang-cache" \
make -C .local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e \
  -f Makefile.libretro -j4 platform=osx arch=arm64 HOOK_CPU=1 GIT_VERSION=60bee4c
TMPDIR="$MD_WORK_ROOT/.local/md-yuyuhakusho/tmp" \
clang -dynamiclib -O2 scripts/yuyuhakusho/hook_filter.c \
  -o .local/md-yuyuhakusho/tools/hook_filter.dylib
```

若此前同一源码目录用不同编译选项构建，先清理本目录的对象文件或加 `-B` 完整重编译。工具不能把旧文件的存在当作本次构建成功；编译失败时停止。

## 采集原输入函数

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_inputs.py \
  --out .local/md-yuyuhakusho/input-capture
```

观察器只在四个原地址接收事件：三键进入／返回 `0x2d5e/0x2d94`，六键进入／返回 `0x2dce/0x2e32`。记录输入数据、入口寄存器、A/C 交换设置、通道、前后状态及完整调用边界，并把带钩子的 3,600 帧 RAM／画面与原内核基线比较。成功报告和提炼后的夹具已保存在游戏目录；未完成或重入异常不得悄悄丢弃。

## 独立 Go 与 Wasm

```sh
mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json ./games/yuyuhakusho/engine ./games/yuyuhakusho/cmd/verify-animation-catalog

GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
GOOS=js GOARCH=wasm \
go test -count=1 -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
  ./games/yuyuhakusho/engine ./games/yuyuhakusho/cmd/verify-animation-catalog
```

各阶段先列失败场景、保存原版输入／输出并编写测试，首次因缺少实现失败后才加入独立规则。输入、动作、命令、动画和碰撞阶段均有各自覆盖边界；完整集成回归仍不代表网络同步、渲染或完整对局通过。

## 命令、全角色目录与碰撞

下列脚本均使用 `--out` 指向 worktree 内 `.local/md-yuyuhakusho/` 子目录。原版自然记录须保持 3,600 帧 RAM／画面基线一致；明确声明的隔离用例只修改私有研究进程输入。生产引擎不读取 ROM、模拟器或原版输出。

| 脚本 | 范围与证据 |
| --- | --- |
| `capture_commands.py` | [通用命令和酎准备](../../games/yuyuhakusho/engine/COMMANDS_VERIFICATION.md) |
| `extract_animation_catalog.py` | [11 个来源集合及逐帧原 CPU 对照](../../docs/research/md-yuyuhakusho/animation-catalog.md) |
| `trace_shape_consumers.py` | [自然执行中的形状读取](../../docs/research/md-yuyuhakusho/COLLISION_RESEARCH.md) |
| `capture_body_separation.py` | [完整身体分离](../../games/yuyuhakusho/engine/BODY_SEPARATION_VERIFICATION.md) |
| `capture_clash.py` | [完整攻击相消](../../games/yuyuhakusho/engine/CLASH_VERIFICATION.md) |
| `capture_throw_contacts.py` | [完整投技接触](../../games/yuyuhakusho/engine/THROW_CONTACTS_VERIFICATION.md) |
| `capture_attack_contacts.py` | [防御、伤害及反应初始化](../../games/yuyuhakusho/engine/ATTACK_CONTACTS_VERIFICATION.md) |
| `capture_collision_manager.py` | [完整管理器组合与尾段](../../games/yuyuhakusho/engine/COLLISION_MANAGER_VERIFICATION.md) |

各夹具和来源摘要在 `games/yuyuhakusho/engine/testdata/`；本阶段整体测试与覆盖见 [collision-stage.json](../../games/yuyuhakusho/verification/collision-stage.json)。尚未恢复的招式分派、原版电脑、完整人物／世界步和呈现列在[进度记录](../../games/yuyuhakusho/PROGRESS.md)。

## 角色管线后续阶段

这些脚本沿用上面的固定 ROM、内核与 worktree 内输出目录。每阶段先保留原版夹具与测试失败，再加入独立规则；表中的“完整”仅指所列函数或阶段边界。

| 脚本 | 验证范围 |
| --- | --- |
| `capture_character_commands.py` | [11 个来源角色的命令准备](../../games/yuyuhakusho/engine/CHARACTER_COMMANDS_VERIFICATION.md) |
| `capture_fighter_preparation.py` | [共用输入准备及攻击值随机尾段](../../games/yuyuhakusho/engine/FIGHTER_PREPARATION_VERIFICATION.md) |
| `capture_chu_attacks.py` | [酎四种攻击选择器及完整构造](../../games/yuyuhakusho/engine/CHU_ATTACK_VERIFICATION.md) |
| `trace_throw_records.py`、`capture_throw_records.py` | [投技写入定位与酎完整提交](../../games/yuyuhakusho/engine/THROW_RECORDS_VERIFICATION.md) |
| `capture_reaction_motion.py` | [攻击完成、受击及恢复阶段](../../games/yuyuhakusho/engine/REACTION_MOTION_VERIFICATION.md) |
| `capture_chu_special_motion.py` | [酎持续特殊动作与三种子对象建立](../../games/yuyuhakusho/engine/CHU_SPECIAL_MOTION_VERIFICATION.md) |
| `capture_chu_tail.py` | [正常／停顿共享尾段完整返回](../../games/yuyuhakusho/engine/CHU_TAIL_VERIFICATION.md) |
| `capture_sound_queue.py` | [声音优先级消费者与历史差异关联](../../games/yuyuhakusho/engine/SOUND_QUEUE_VERIFICATION.md) |
| `trace_sprite_consumers.py`、`extract_sprites.py` | [图形帧组合、图块与选定像素对照](../../docs/research/md-yuyuhakusho/SPRITE_RESEARCH.md) |

本批独立规则使用上述 `engine` 与 `verify-animation-catalog` 回归命令。精灵验证器另运行 `go test -race -count=1 ./games/yuyuhakusho/cmd/verify-sprites`，Wasm 使用同一执行器；它覆盖研究数据组合与选定像素，尚未接入正式渲染器。测试日志、当前来源摘要和明确剩余工作见 [actor-pipeline.json](../../games/yuyuhakusho/verification/actor-pipeline.json)。

## 完整人物、世界基础与索引素材

新增脚本均沿用固定 ROM／内核和 worktree 内 `--out` 目录。它们覆盖所列完整规则边界；绘制请求不等于最终精灵提交，单对象或单阶段流不等于整个世界循环。

| 脚本 | 验证范围 |
| --- | --- |
| `capture_chu_action.py` | [完整酎动作调用](../../games/yuyuhakusho/engine/CHU_ACTION_VERIFICATION.md) |
| `capture_chu_ai.py`、`capture_chu_input.py` | [真实 AI](../../games/yuyuhakusho/engine/CHU_AI_VERIFICATION.md)与[完整输入](../../games/yuyuhakusho/engine/CHU_INPUT_VERIFICATION.md) |
| `capture_fighter_envelope.py`、`capture_fighter_bounds.py`、`capture_chu_actor.py` | [包络](../../games/yuyuhakusho/engine/FIGHTER_ENVELOPE_VERIFICATION.md)、[边界](../../games/yuyuhakusho/engine/FIGHTER_BOUNDS_VERIFICATION.md)和[完整活跃人物规则](../../games/yuyuhakusho/engine/CHU_ACTOR_VERIFICATION.md) |
| `capture_fighter_initialization.py` | [全角色初始化与规格](../../games/yuyuhakusho/engine/FIGHTER_INITIALIZATION_VERIFICATION.md) |
| `capture_chu_projectile.py`、`capture_chu_attachments.py` | [弹体](../../games/yuyuhakusho/engine/CHU_PROJECTILE_VERIFICATION.md)与[附件](../../games/yuyuhakusho/engine/CHU_ATTACHMENTS_VERIFICATION.md)规则生命周期 |
| `capture_effect_motion.py`、`capture_attached_effects.py` | [通用 FX](../../games/yuyuhakusho/engine/EFFECT_MOTION_VERIFICATION.md)与[数字／附着 FX](../../games/yuyuhakusho/engine/ATTACHED_EFFECTS_VERIFICATION.md) |
| `capture_match_lifecycle.py`、`capture_world_clock.py` | [退场／结算阶段](../../games/yuyuhakusho/engine/MATCH_LIFECYCLE_VERIFICATION.md)与[计数／暂停／反馈](../../games/yuyuhakusho/engine/WORLD_CLOCK_VERIFICATION.md) |
| `extract_character_assets.py`、`extract_arena_assets.py` | [角色索引素材](../../docs/research/md-yuyuhakusho/CHARACTER_ASSETS.md)与[默认场景](../../docs/research/md-yuyuhakusho/ARENA_ASSETS.md) |

角色／背景验证分别运行 `go test -race -count=1 ./games/yuyuhakusho/cmd/verify-character-assets` 和 `go test -race -count=1 ./games/yuyuhakusho/cmd/verify-arena-assets`，Wasm 采用前述执行器。已验证的客户端索引资产仍是本地研究／接入材料，本阶段不上传 OSS、不发布应用，也不把它们嵌入服务器。

整体结果与来源绑定见 [world-foundations.json](../../games/yuyuhakusho/verification/world-foundations.json)。仍未完成世界循环、完整独立对局和平台 E2E；这些门槛保留在进度记录中。

飞影／九角色检查点的完整规则与呈现矩阵：

```sh
sh scripts/yuyuhakusho/verify_hiei_sessions.sh both
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_hiei_player.py
python3 scripts/yuyuhakusho/audit_fighter4_dragon_assets.py --out games/yuyuhakusho/verification/fighter4-dragon-assets.json
```

矩阵为 21 组规则配置和 12 组完整呈现；旧角色回归使用 `verify_hiei_legacy.py native` 和 `verify_hiei_legacy.py wasm`，共 192 份结果。全量规则／目录检查集合、四个原生分片和来源 SHA 在 `verification/hiei-native-shards.json`；版本元数据兼容复跑及全部原始日志由 [hiei-session.json](../../games/yuyuhakusho/verification/hiei-session.json)绑定。使用相同规则包，原生和 Wasm 的完整状态、事件和像素轨迹一致。

飞影特殊原世界使用 `capture_fighter4_world.py`，双真人练习就绪世界使用 `capture_fighter4_duo_world.py`。两者必须通过 `--reference`、`--reference-manifest` 复用已冻结原 core 控制组，不重新产生第二份控制组；还需显式传入 `--inputs`、`--out`、`--start`、`--end`、`--allow-open-end` 和 `--name`。九根场景是 `1800..3600`、3602 控制步，Duo 是实际 `a01e/stage32` 的 `2500..5500`、5502 控制步。各自 manifest 保存唯一输入／控制组路径和 SHA；原版菜单早期状态不属于已验证 Go 等待阶段。

仙水／十角色最新检查点：

```sh
sh scripts/yuyuhakusho/verify_sensui_sessions.sh both
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_sensui_player.py
python3 scripts/yuyuhakusho/audit_fighter5_projectile_assets.py --out games/yuyuhakusho/verification/fighter5-projectile-assets.json
```

矩阵为 23 组规则配置和 13 组完整呈现。旧九角色回归使用 `verify_sensui_legacy.py native` 和 `verify_sensui_legacy.py wasm`，共 234 份结果。完整 351 引擎＋3 目录测试的来源／分片在 `verification/sensui-native-shards.json`；Wasm 初次达到20分钟限制后，仅对剩余75项续跑，精确清单、压缩正则、最小环境和原始日志在 `sensui-wasm-continuation.json`。GoWasm 的 argv＋环境有8KiB启动区，长正则与无关宿主变量可能超过上限，不把没有执行测试的启动失败当作规则失败。完整通过集合与原始记录由 [sensui-session.json](../../games/yuyuhakusho/verification/sensui-session.json)绑定。

Source5 普通世界沿用未修改的 `capture_chu_world.py`，特殊世界使用 `capture_fighter5_world.py` 及唯一原 core3602控制组。两条输入／原 SHA 均在对应 manifest；特殊采样范围为1800..3600，另两步排空。固定输入的发现与复放说明见 [SOURCE5_SPECIAL_DISCOVERY.md](../../docs/research/md-yuyuhakusho/SOURCE5_SPECIAL_DISCOVERY.md)。

全11人物最新检查点：

```sh
sh scripts/yuyuhakusho/verify_itsuki_sessions.sh both
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_itsuki_player.py
python3 scripts/yuyuhakusho/audit_fighter7_children_assets.py --out games/yuyuhakusho/verification/fighter7-children-assets.json
```

矩阵25组规则／14组完整呈现；旧十人物回归用 `verify_itsuki_legacy.py native` 与 `verify_itsuki_legacy.py wasm`，共280份报告。380引擎＋3目录顶层检查按同一冻结清单分片，见 `itsuki-native-shards.json`／`itsuki-wasm-shards.json`。Wasm4片完整通过；原生20分钟预算后61项续跑的精确集合、源码及命令在 `itsuki-native-continuation.json`，没有跳过未完成项。原始超时、全部最终通过集合和owner／音频适配器修正由 [itsuki-session.json](../../games/yuyuhakusho/verification/itsuki-session.json)绑定。

树的特殊原控制为5502步，所有14个玩家根自然成功；分派初态、回调、FX和跨4父Health／停顿均保留完整64KiB审计。世界原始音频队列是独立异步消费者边界，组合v2另以实际request/write/drop/clear事件精确验证，不整队列遮蔽。滚动和录制输入共用一个Stream owner，旧PanA/PanB只作JSON派生视图；六份旧完整JSON和旧对局精确兼容，原始4054帧失败及先行测试保留。
