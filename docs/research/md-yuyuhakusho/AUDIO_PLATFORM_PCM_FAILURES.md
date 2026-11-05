# 平台原生 PCM 流的先行失败谱

本清单、类型、完整真实平台 E2E 和受控竞态测试均先于 runtime。入口正式名为 `BoardProps.audio.openPCMStream`，播放器、生成轨道、AudioData 与清理由平台管理，不导入研究 probe 的私有播放器，不创建 AudioContext，不读取麦克风或外部声音。

- 缺入口或能力仅存在但不实际播放；unsupported／构造失败时截断有效 MSE／PCM；成功新输出却与旧 mixed 输出并存。legacy finite／music 必须独立。
- 非 44,100 Hz、双声道、s16le；非法 ID、安全整数、sequence／start、timestamp 溢出；packet 不在 1～1,024 帧，或字节数不等于帧数乘 4；ahead 超过 2,048。
- caller mutation 改变 pending 音频；不同片 busy 时无界排队；精确 pending 重复不复用 Promise；重复最后片重播，冲突重复或旧历史被接受。
- 同 owner／ID／配置刷新重建原生资源；同 ID 配置冲突毁坏有效流；旧 handle／owner 停止新 epoch。
- ready 等待尚无输入的 play，导致 producer 死锁；write 尚未成功就提交接收游标；stop 后迟到 resolve／reject／playing 复活或提交旧流。
- 借 native currentTime 给源输入授信；非法／busy／重复请求重设时钟原点；暂停的墙钟继续形成欠账，resume 后批量追补。
- 超过固定 4,410 samples（100 ms）源迟到仍静默追赶、跳样或伪造 drop。未声明 endInput 的供给缺口及 pending 超时都需失败并清理；endInput 后不再检查有限输入缺口，但 pending 仍受迟到预算约束。
- endInput 未锁新输入、丢掉 pending、在名义源结束前 close writer；把 writer close／track ended／HTML currentTime 当成已播完；nativePlayedSamples 或 nativeDroppedSamples 被填成 0 或接收计数。
- 输入关闭误标 drained／ended；released 被当成音频 drain，或仍有 track／writer／srcObject／AudioData 时就报告 released。
- pause／resume、mute／hidden／离桌／dispose 和 MSE／PCM 模式切换未释放所有资源；取消后旧 callback 重新开始，或解除静音补播历史。
- 构造的 AudioData 格式、帧数、时间戳或 copyTo 字节不等于原 PCM；成功／失败／取消路径漏 close，writer 未解锁或轨道未 ended。
- 预取源文件被冒称原生预载全部音频；只看 write ACK，不看真实 playing／连续媒体时钟；以源连续证明 native 或 speaker 无 drop／bit-exact。

v1 源时钟从第一片合法 append 建立，pause 冻结。默认／最大 ahead 2,048 samples，packet 最多 1,024 帧，一片 pending；只留最后接受片。100 ms 是明确失败预算，不是输出丢样本测量。endInput 成功仅标 input-ended；host 必须显式 stop，released 只证明资源释放。真实浏览器至少覆盖原 12 秒 1,024／441 分片、35 ms 抖动、暂停、mute、stop、真实 hidden、源超迟到及与 MSE 互斥，明确保留不可观测输出计数为 null。
