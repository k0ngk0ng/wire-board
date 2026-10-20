# 客户端与资源取证边界

本次按用户要求查找历史原客户端，并在官方下载验证码无法继续后，使用用户允许的互联网存档与网页还原版本补充参考。以下区分实际执行、静态提取与无法确认的部分。

## 历史客户端资源

来源：`https://archive.org/download/crazy_arcade_setups/2002.03.13./Crazy%20Arcade.zip`。互联网档案馆将其标注为 2002 年 Crazy Arcade 客户端。下载包 SHA-256 为 `326c9b89b17e3404e192073261f54f34e27e5225d3f955e39bc8d97204405ae6`。

REA 对 ZIP 进行了格式检查与安全提取，完整 Evidence 保存于 `.local/bubbleparty-original/original-2002-extraction.json`。提取的 `Fx.idx` 包含 1164 条资源记录；独立解析器验证索引完整消费、资源范围不重叠、数据范围不越界、源文件摘要以及 WAV／MP3 格式，提取了 46 个音频块。本项目选取其中 13 个，逐字节保留并仅增加正确扩展名。每个发布文件均在 `games/bubbleparty/assets/manifest-v2.json` 绑定原记录偏移、长度和 SHA-256。

原包和解出的客户端保留在 `.local/bubbleparty-original/`。清理临时环境时将客户端移到 `client-2002/`，`retained-client-location.json` 记录迁移前后路径和文件哈希；没有修改 REA 的原 Evidence。

可重复核验：

```sh
python3 -B scripts/bubbleparty/extract-classic-audio.py \
  --client-dir .local/bubbleparty-original/client-2002 \
  --out .local/bubbleparty-original/audio-verified
python3 -B scripts/bubbleparty/check-assets.py
```

## 实际运行与限制

- 指定官网可以访问，也能取得地图、操作和道具资料。V333 官方下载跳转要求滑块验证码；没有绕过验证，也没有据此断言官方已经停服。
- 2002 Windows 客户端尝试在仓库内的 Wine 前缀运行。Wine 要求在 `/tmp/.wine-501` 创建 socket，被本次只允许写仓库的沙箱拒绝，未放宽写入范围。该尝试没有进入原版对局，日志为 `.local/bubbleparty-original/original-launch.txt`。
- 另对历史 BnB Arcade 2019 macOS 移植包进行 REA 解包，并启动了其 320×268 窗口。REA 运行截图因应用缺少可核验的 launch time 返回 `process-identity-unavailable`；进程随后关闭。这个街机移植版不等于 Windows 网游，本项目未用它证明网游规则或手感。相关包、静态 Evidence 和失败原因保留在 `.local/bubbleparty-original/`。
- 作为用户允许的网页还原参考，`visolleon/bnb` 的固定提交已在本机运行，操作与录像保存在 `.local/bubbleparty-reference/remakes/`。源码经 REA 分析，完整 Evidence ID 为 `ev_9bd3d2e2ef37e2c94d94ee337e6258f62e70982ff13cfee021f26a5773bc24a6`。这个 Demo 默认小区 13，且自身规则存在简化；只使用其可核验图集和帧定义，不把它当原版规则真值。

因此，本次实际完成的是历史包资源提取、官方截图核对、网页还原运行参考，以及围桌新版的真实浏览器对局。没有声称已登录官方服务器或完成原版联网对局。困泡时间、保护时间和部分地形语义仍按项目参数明确记录，而不是假装已由原版运行确认。
