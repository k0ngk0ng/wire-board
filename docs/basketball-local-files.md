# 热血篮球本地文件

热血篮球的本地文件统一保存在 `.local/basketball/`，目录约束见[通用规范](../rules/common.md)。

| 子目录 | 用途 |
| --- | --- |
| `assets/` | 提取素材、音频母版、封面和参考图 |
| `research/` | 原始 ROM、存档、采样和分析记录 |
| `cache/` | 可重建的资源、Wasm 和工具缓存 |
| `verification/` | 测试结果、截图、日志和迁移校验记录 |
| `history/` | 历史发布资料和保留记录 |
| `tmp/` | 测试夹具、浏览器配置和临时 socket |

通用研究工具统一保存在 `.local/tools/nes-research/`，由篮球和其他游戏共用。原始采样、历史日志和证据清单保留原始内容；其中记录的旧路径通过迁移清单追溯。执行脚本、当前文档和目录软链接使用新路径。

## 迁移失败场景

- 新目录与已有文件冲突，或覆盖其他任务的修改。
- 文件正在被进程使用，或迁移涉及仍在使用的 worktree。
- 遗漏脚本、动态拼接路径或浏览器资源路由，运行后重新创建旧目录。
- 共享工具移动后，其他游戏无法导入研究客户端或找到 Ghidra。
- 目录软链接仍指向旧路径，或解析到工作区外。
- 原始素材、独立采样、历史证据发生丢失或内容变化。
- 缓存地址变化后误用旧构建，或 Wasm 与当前源码及发布清单不匹配。
- 路径检查遗漏某种开发语言，或将测试负例当作实际输出路径。
- 历史证据中的相对或绝对路径无法解析到迁移后的原始采样目录；路径解析接受越界、目录遍历或指向工作区外的软链接。

## 检查范围

`scripts/check_local_game_paths.py` 检查仓库中 Python、JavaScript、TypeScript、Go、Shell、Lua、HTML 和 CSS 源文件的篮球本地路径，并检查 `.local/` 根目录是否出现旧篮球目录。历史证据和采样文件不作为执行代码扫描。该检查接入游戏边界检查；正负例由 `scripts/test_check_local_game_paths.py` 重放。

资源缓存与封面缓存的目录位置由现有资源测试验证；迁移清单和保留文件摘要写入 `.local/basketball/verification/local-layout/`。仅调整本地文件和工具路径，不改变游戏规则或公开资源地址。

```sh
python3 -B scripts/check_local_game_paths.py
python3 -B scripts/test_check_local_game_paths.py
python3 -B scripts/basketball/test_local_paths.py
.local/board-art-env/bin/python -B scripts/basketball/selection_assets/test_export.py
python3 -B scripts/basketball/accept_asset_fallback.py
```

浏览器结果和截图保存在 `.local/basketball/verification/asset-fallback/`。`retained.sha256` 可用 `shasum -a 256 -c .local/basketball/verification/local-layout/retained.sha256` 重复校验。
