# 《魔强统一战》原版取证与规则验收

先阅读 [失败场景与验收范围](REFERENCE_VALIDATION.md)。当前工具在 macOS arm64 验证；libretro 内部结构与固定的 Genesis Plus GX 提交绑定，不能换内核后直接沿用其 ABI。独立游戏代码不依赖这些工具。

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
