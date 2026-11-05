# 实时服务端模块与共享 tick 的先行失败范围

本清单先于完整模块、资源加载器及共享 tick 实现。服务端验收不代表客户端预测、真实慢连接隔离或平台上架已经完成。

- 不支持人数／配置仍开局；seat、原版 slot、输入通道或胜方映射混淆，尤其两人 0／3、三人 0／1／3 与四人队伍 1／1／4／4。
- 模块回调读取文件／客户端像素；计算资源被错绑，metadata／rules 摘要不一致。
- 初始真人未全部就绪推进世界；电脑或观战者阻塞；重复 loaded、重连或恢复存档重新给全员加载 gate／重置比赛。
- 直接按 16-bit 原帧计数同步，网络帧回绕；虚拟 NTSC 时间累计丢失余数；系统追赶无界；客户端 ACK 限制服务器推进。
- epoch、seat、按钮 high bits、JSON／重复字段、seq 跳跃或旧请求未经验证；错误请求在晚期失败后仍部分提交时钟、输入、世界或呈现。
- 同一系统 tick 前的 press／release 被压成 neutral 而丢短按；状态队列无界，重连后补播旧队列。
- 空闲、超时、后台或断线被改为 AI；释放 transport 时清掉规则 Controller 历史；automation 同帧改 world 但 replay 没记录。
- Reclaim 改变动作、HP、位置、RNG 或其他席位；旧 AI 控制／输入残留，或者取消托管后重新开始比赛。
- 共享 tick 由服务器和客户端采用不同 automation／采样顺序；未验证输入先改 world；late renderer error 破坏调用者原状态。
- 普通 View 泄漏完整 World／save／像素／无限历史；恢复 checkpoint 缺 cache、继承 sprite slot 或有序 HUD ref；两个 digest 错误 mask 规则历史。
- all-bot 无法完整比赛；真人与电脑混合、人数／场地／人物／组队之一接入后不可完成；恢复后的规则及呈现轨迹不同。
- WinnerCode 直接减 1 当平台 seat；和局用双方都赢代替；含电脑／曾托管的对局计竞技积分；清场前提前结束。
- 存档格式、计数器、object 数、输入队列、历史或图形 refs 不受限；clock regression／异常状态被静默推进；restore 改变已完成结果。

共享 tick 将平台控制变化限定在有效网络帧开始：Match.Controllers 和 live InputChannel 是各自既有物理 owner；不修改 World.Controllers 的历史，不另存世界的重复控制字段。所有稳定素材仅为计算数据，图片／音频／Wasm 不进入服务端资源。
