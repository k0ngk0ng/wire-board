# 客户端手动 gzip 发布失败场景

本清单先于公共发布扫描改动和对应测试建立。范围为客户端 manifest 标准资源记录声明的独立 `.json.gz` 与 `.glb.gz`；应用层 gzip 与 HTTP `Content-Encoding` 分别处理。

- 没有 manifest、资源未声明、声明缺少必要字段、路径不规范或越过所属游戏目录。
- 同一文件多次声明的压缩或还原身份冲突；相同记录在索引与分类中重复出现却被误判。
- 声明的压缩大小、压缩摘要、还原大小或还原摘要与实际文件不同，或用布尔值冒充整数。
- 资源 MIME 与后缀或还原格式不符；JSON 无法解析；GLB 头、版本、总长、块范围或 JSON 块无效。
- gzip 头、CRC、流尾损坏，存在尾随数据或多个成员，或压缩／还原超过有界上限。
- 声明或资源文件通过符号链接读取工作区外数据；声明的资源缺失。
- 应用层 `.gz` 被再次设置 HTTP gzip，导致客户端收到自动解码后的错误载荷；扫描记录误用还原后的大小或摘要。
- 原有 JS／GLB 的 HTTP 压缩与 sidecar 校验被破坏；存在原文件时把明确声明的手动 gzip 错当成 sidecar 丢失。
- 上传前扫描与剥离之间资源或 manifest 改变，或正式服务器包仍残留客户端资源。
- 只修改公共发布入口未执行本检查，或因此拉起全部游戏测试。

复跑：`TMPDIR="$PWD/.local/cs16/tmp" python3 -B scripts/test_cdn_manual_gzip.py` 和同环境下的 `scripts/test_cdn_delivery.py`。产物位于 `.local/cs16/verification/cdn/`，测试仅使用工作区文件与回环 HTTP，不上传且不访问 OSS。
