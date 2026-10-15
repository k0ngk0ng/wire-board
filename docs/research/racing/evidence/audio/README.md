# 赛车音效来源核验

2026-10-15 核验。本文只确认可用来源与许可，未把音效接入游戏、上传 OSS 或发布。样品均可直接下载，无需创建素材网站账号。

## 建议选型

| 用途 | 来源 | 许可 | 当前状态 |
| --- | --- | --- | --- |
| 随转速变化的循环引擎声 | [domasx2 — racing car engine sound loops](https://opengameart.org/content/racing-car-engine-sound-loops) | CC0 | 保存低、高音两个原始 WAV；页面共六个音高版本 |
| 连续漂移轮胎声 | [Tom Haigh / qubodup — Car tire squeal skid loop](https://opengameart.org/content/car-tire-squeal-skid-loop) | CC BY 3.0 | 保存独立 3 秒循环 WAV；需要署名、许可链接和修改说明 |
| 车体／护栏、轮胎墙碰撞 | [Kenney — Impact Sounds](https://kenney.nl/assets/impact-sounds) | CC0 | 已从官网 ZIP 提取金属与软碰撞样品，保留包内许可 |
| 倒计时、发车、过线 | 项目原创短提示音 | 项目原创 | 建议离线合成；本次尚未生成 |
| 短音效备选 | [Scribe — SuperTuxKart sound effects](https://opengameart.org/content/supertuxkart-sound-effects) | CC0 | 保存完整 81.9 秒 MP3；需要逐段试听、切片 |
| 引擎声备选 | [qubodup — Car Engine Loop 96kHz, 4s](https://opengameart.org/content/car-engine-loop-96khz-4s) | CC BY 3.0 | 作者自录的 4 秒循环；已核验页面，未下载音频 |

优先组合为 CC0 引擎、CC BY 轮胎、CC0 碰撞与原创提示音。CC BY 允许商业使用和改编，需按许可署名。无需为了全用 CC0 把单个音效合集误当作可直接使用的独立漂移循环。

## 样品与证据

打开 [试听页](audition.html)，或直接播放：

- [低音引擎循环](engine-loop-0.wav)，0.864 秒。
- [高音引擎循环](engine-loop-5.wav)，0.630 秒。
- [漂移轮胎循环](tire-loop.wav)，3 秒。
- [金属碰撞](impact-metal-medium.ogg)，0.272 秒。
- [软碰撞](impact-soft-heavy.ogg)，0.505 秒。
- [完整赛车音效合集](kart-sfx-original.mp3)，81.868 秒。

[sources.json](sources.json) 记录作者、用途、许可和限制；[downloads.json](downloads.json) 记录准确下载地址、SHA-256 和 ZIP 内路径。原始许可页面保存为 `.html.txt`，只作为来源证据，不作为同源可执行网页公开。样品均未裁剪、调音或转码。

## 已知限制

- 引擎作者说明底材来自 pdsounds.org 的公有领域录音，并在评论中记录已用公有领域素材重做。当前页面标为 CC0；底材旧站外链本次没有复核。六个文件只是音高不同，不是六种车辆引擎。
- 引擎循环仍需检查接缝、持续听感和转速过渡。本次完成文件解码与时长核验，没有作主观听感验收，不能把「来源已找到」等同于「最终声音已完成」。
- Scribe 合集的页面标签包含倒计时、轮胎、引擎和启动，但没有逐个文件或时间索引。本次未逐段试听，不给未确认片段命名，也不把按钮声当作引擎声。
- 轮胎声来自 audible-edge（Tom Haigh），qubodup 将它处理成循环。发布时保留两者信息；若再降采样、转码或裁剪，额外记录修改。

## 平台接入边界

当前 `web/src/audio.ts` 提供单次音效与固定音量的背景音乐循环，没有引擎所需的循环效果音高、音量调节接口。实现赛车时，应在公共平台补充受统一静音、页面隐藏和离桌清理约束的原生媒体循环能力，游戏只提供音频地址、转速和漂移强度。

引擎声需要随模拟转速平滑变化，并核实原生媒体的音高处理；不能只改变播放速度却保留原音高。漂移声按侧滑程度淡入、淡出，碰撞和倒计时保留独立短音效通道。所有声音仍使用平台 `BoardProps.audio`，不在赛车目录创建播放器或 `AudioContext`。

素材正式处理后放入 OSS 不可变版本目录，保留本地回退、来源、生成／处理命令和新文件 SHA-256。本次没有上传。
