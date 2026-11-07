# 可玩版声音交付

本版按用户选择交付：玩法和画面保留，音乐、台词和动作音效跟随实际规则请求播放，允许与原版有细微差异。原硬件逐拍计时研究已冻结，不作为交付前提。

| 验收 | 结果与证据 |
| --- | --- |
| 实际规则和画面 | 620 步、21 个有序声音请求；收集前后的存档与像素完全一致，失败步骤不发布声音。[native.jsonl](native.jsonl) |
| 竞态与共享规则 | 新音频对局和既有共享规则、错误回滚检查通过。[native-race.jsonl](native-race.jsonl) |
| 无损媒体 | 143 个 FLAC，281815656 字节 WAV 压为 141537925 字节；44.1 kHz、双声道、16 bit、帧数与解码 PCM SHA-256 全部一致。[assets-validation.json](assets-validation.json) |
| 真实浏览器与平台音频 | 音乐、动作音效、暂停／恢复、静音／取消静音、真实后台切换和重连去重通过；稳定前台约 59.60 fps，帧间隔 p95 为 25 ms。[browser-06/result.json](browser-06/result.json) |
| 实际平台完整对局 | 正常登录、房间、电脑与主动托管完成默认场地整局，终局帧 2166；战斗音乐只启动一次，结算切换至 8F，无媒体错误。[normal-app/playing-state.json](normal-app/playing-state.json)、[声音事件](normal-app/playing.json) |
| 实际平台另一场地 | 魔性之森音乐 8A 正常播放；实测 186 次可见更新／3098.1 ms，约 60.04 fps，p95 为 22.6 ms；整局在帧 2573 结束。[normal-app/arena-a-playing.json](normal-app/arena-a-playing.json)、[结算](normal-app/arena-a-result.json) |
| 构建与类型 | TypeScript、Go vet 通过；13 份比赛素材、143 个音频对象与实际 Wasm 的摘要校验通过。[app-build-final.json](app-build-final.json) |

本地试玩房间已恢复为等待状态，真人席位为手动控制，已加入一名电脑。运行地址、测试账号和进程句柄保存在 Git 忽略的 `.local/md-yuyuhakusho/platform-app/review-service.json`。原试玩服务保留。实际页面截图：[比赛中](normal-app/arena-a-playing.png)、[准备就绪](normal-app/ready-for-review.png)。

重复浏览器验收：在仓库根目录运行 `python3 -B scripts/yuyuhakusho/check_playable_audio.py --label <未使用的标签>`。脚本使用真实 Go 模块和 Wasm、平台原生媒体通道与现有 FLAC，保留原生输入、声音事件、固定时间窗口帧率和截图；该脚本不代替登录、SQLite 与 WebSocket 验收。

重新生成无损资源：先运行 `python3 -B scripts/yuyuhakusho/export_playable_audio.py`，再运行 `python3 -B scripts/yuyuhakusho/check_playable_audio_assets.py --out <仓库内新结果路径>`。来源为已保留的原版采集文件，见 [media-provenance.json](media-provenance.json)。不重新运行模拟器，也不在产品运行时包含 ROM 或原机程序。

`missing-api-red.jsonl`、`missing-assets-red.log` 与 `missing-playback-red-02/result.json` 是实现前的实际缺功能记录。最初 socket 路径错误、标签页命令、性能观察器参数和暂停短按问题分别保留在对应运行目录；这些不算功能通过。旧性能记录误含后台停顿，最终按独立前台时间窗口计数，50 fps／40 ms 阈值保持。

当前声音采用媒体文件组合，原版声部抢占、混音起始相位和长音乐循环接点可能有差异。只加载当前音乐与实际触发的音效；后续按试玩反馈决定精修范围。本次为本地试玩交付，未执行 OSS 上传、线上发布或服务器更新。
