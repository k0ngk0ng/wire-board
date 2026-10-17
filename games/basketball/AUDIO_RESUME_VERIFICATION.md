# 新恢复动作音效验收

修改前的失败场景：

- 被捕获后跑动请求 ID 1，播放表缺失而返回 unsupported。
- 跑动释放请求 ID 48 的原声素材缺失；不能以其他攻击音代替。
- 跳跃 ID 23 和原始 9 × 32 攻击音表出现遗漏，或者旧素材被重录覆盖。
- 提取错 ROM、错误请求 ID、截断尾音、削波、两次提取不一致或 DPCM 取样落入包装代码。
- 播放表指向不存在文件、SHA 不匹配、未知音静默通过。
- 浏览器未真实解码、平台静音被绕过、重复事件重播、失败 CDN 没有本地回退。

先写映射断言和素材验收，再补充两项原声。原始 27 项清单及其 manifest 保持不变，新项沿用 additional-effects 清单。

## 本次结果

- 新增 ID 1（捕获状态跑动）和 ID 48（Action 68 的 handler 8 在 A 释放时），分别来自 `captured_player.go` 和 `world_auxiliary_running.go`。后者行为来源见 `AUXILIARY_RUNNING_B.md` 的原版控制器连续链。
- ID 23 跳跃沿用原文件。原攻击表 288 个入口涉及的全部 ID 已有素材，无需重录。定向审计覆盖 37 个显式音效调用点；当前播放表共有 34 个原音效。该静态审计不宣称证明任意动态数据流的覆盖。
- 两个新 ID 各执行两次 600 帧原驱动录音：PCM、APU 写入、驱动 RAM、调用记录、样本游标和序列跳转逐字节一致；无削波、尾部已静音、无 DPCM 包装区读取。仅按原 PCM 样本范围截取，未合成替代音。
- 旧 27 项原声及音乐 66 的 `verify_audio.py` 全部通过；原 catalog 和旧文件 SHA 保持不变。既有附加 ID 10、13、14、30、46 复用已保存且 SHA 校验成功的录音。
- 映射测试 16 项通过；原生 Chromium 媒体验收 15 项通过，覆盖 ID 1、23、48 的真实解码和播放结束、失败 CDN 到本地回退、重复事件忽略、平台静音、未知 ID 明确报告和卸载清理。

文件 SHA-256：

| ID | 文件 | SHA-256 |
| --- | --- | --- |
| 1 | sound-001.wav | `40d3e98b3c120e07c77d65a75733a1916b45ec0a452f4ad85b2fa828d1c40e4e` |
| 23 | sound-023.wav（沿用） | `138dd8a1c4e010204770cb507336778155dd6336b0813c42821b74c2a47af23d` |
| 48 | sound-048.wav | `c80b939e0e2e6e97b30ecff18c8b8a5f02e0e3d73a17e515e0404c0f64c029f6` |

所有媒体与原始记录在 `.local/basketball/audio/export/`。`additional-effects-manifest.json` 记录 ROM SHA、来源、样本范围和录音路径。失败与通过记录在 `.local/basketball/audio-resume/`；浏览器逐事件证据在 `.local/basketball/audio-effects-browser/result.json`。

复验命令（仓库根目录）：

```sh
source scripts/env.sh
node --experimental-strip-types --test games/basketball/tests/audio.test.mjs
python3 scripts/basketball/verify_additional_effects.py
python3 scripts/basketball/verify_audio.py
python3 scripts/basketball/verify_audio_effect_coverage.py
python3 scripts/basketball/accept_audio_effects.py
```

缺失原始录音时运行 `python3 scripts/basketball/export_additional_effects.py`，已有录音必须先通过保存的 SHA 检查才可复用。

本次未修改平台播放器、模拟逻辑或内部音频状态机。播放仍通过 `BoardProps.audio`。浏览器验收主动提交原始声 ID，不等于完整对局混音或物理手机听感验收；两者仍未完成。素材来源为用户提供 ROM，公开再分发授权尚未确认；没有上传 OSS，`accepted_for_release` 和 `listening_accepted` 均保持 `false`。
