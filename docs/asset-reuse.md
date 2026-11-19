# 资源外置与配置

游戏资源使用 `bga/<game-id>/`，公共资源使用 `bga/_common/`。文件按内容 SHA-256 命名；HTTP gzip 表示增加 `-gzip`，防止同一 URL 对应不同响应格式。路径不含日期、发布版本或应用提交号。构建来源通过清单中的 `sourceSHA`、大小、摘要和构建收据校验。

## 源码与恢复

Git 只保留源码、服务端规则数据和资源元数据。图片、音视频、字体、客户端二进制、压缩包、生成的公开地图／图集文件及 Base64 媒体载荷不进入当前树或历史。测试图案通过代码生成。

`assets/resources.json` 记录当前源码需要的文件及内容地址，`assets/resource-history.json` 保存历史路径、原 Blob ID 和资源摘要，`assets/external-resources.json` 保存现有游戏的运行资源映射。清单不包含资源正文。

从仓库根目录执行：

```sh
. scripts/env.sh
python3 -B scripts/resource_store.py hydrate
npm --prefix web ci
npm --prefix web run build
```

只恢复单款游戏及公共源码资源时，增加 `--game <game-id>`。恢复过程检查 SHA-256、大小和 MIME，拒绝目录逃逸、符号链接和不一致内容。恢复后的资源由 Git 忽略。

## 业务配置

本地配置保存在 Git 忽略的 `.local/business.json`，使用以下字段：

| 字段 | 用途 |
| --- | --- |
| `PUBLIC_ORIGIN` | 站点 HTTPS Origin，用于在线验收、CORS 和已部署清单读取 |
| `CDN_ORIGIN` | CDN HTTPS Origin |
| `OSS_BUCKET` | OSS Bucket |
| `OSS_ENDPOINT` | OSS Bucket 的服务主机名 |
| `ASSETS_BASE_URL` | 游戏素材入口，使用 `${CDN_ORIGIN}/bga` |

`. scripts/env.sh` 从 JSON 生成 `.local/business.env` 并导出变量，不能只维护生成的环境文件。GitHub 测试和发布通过同名仓库变量注入；Actions 不读取本地配置，也不使用代码中的线上站点域名作为默认值。OSS 凭据继续通过现有私密文件或 Actions Secrets 注入，不写入元数据。

## 发布与兼容

前端构建登记程序、样式及资源的游戏归属；跨游戏的公共代码使用 `_common`。`scripts/asset_reuse.py` 将逻辑路径映射到内容地址，再更新 JS、CSS、HTML 和加载清单。未变内容继续使用同一 URL。

`python3 -B scripts/resource_store.py publish --game <game-id> --manifest <清单路径>` 默认执行完整预检，增加 `--execute` 后才写入 OSS。已有对象先校验，新增对象禁止覆盖，写入后回读。旧发布脚本统一使用同一内容地址规则。

应用的 `/bga/<逻辑路径>` 只返回清单中的 307 跳转，资源正文来自 OSS/CDN。旧逻辑文件名可以保留用于兼容和本地恢复；最终 CDN 地址使用游戏或公共资源命名空间。不能把未进入当前清单的旧素材地址直接改写到清单入口。

Actions 上传和检查 OSS 后生成候选包。CDN 的 SHA-256、大小、MIME、CORS 和音频 Range 回读仍在本地执行；验证成功后才开放该提交的发布门禁。正式服务器包只保留入口和资源元数据。历史 OSS 对象保留，Git 历史清理不触发对象删除。

## 验证

先行失败场景见 [资源复用失败清单](../scripts/asset-reuse-failures.md)。主要入口：

```sh
. scripts/env.sh
python3 -B scripts/resource_store_e2e_test.py
python3 -B scripts/canonical_publication_e2e_test.py
python3 -B scripts/asset_reuse_e2e_test.py
python3 -B scripts/test_cdn_delivery.py
python3 -B scripts/test_cdn_local_ready.py
python3 -B scripts/resource_store.py check
node games/racing/build/lint.mjs --config games/racing/build/asset-reuse-lint.json --self-test
go test ./internal/server -run '^TestStaticMedia|^TestPublicVersion' -count=1
```

检查覆盖内容复用、改名、编码区分、错误内容禁止覆盖、公开资源恢复、配置来源、路径归属、程序依赖、HTTP 跳转和 Range。测试结果保存在工作区 `.local`；具体 lint 范围在配置中登记，不代表全仓库语言 lint。
