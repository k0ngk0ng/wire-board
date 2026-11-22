# CS 计分板对照

本轮以固定第三方归档中的客户端、界面资源和本地化文本作为静态依据，恢复 Tab 计分板的阵营分组和字段。没有启动或运行归档中的游戏，也未取得 Valve 官方签名或摘要。

## 样本与证据

归档为 `martinsam16/cstrike` 的提交 `651d0d248ba04028b1b8167b156888b522bbcb73`，与既有 HUD 研究使用同一提交。客户端 `client.dll` 的 SHA-256 为 `2dad31c59dabbddfaae5c9df0577899fda5f3f68bce6b492ef3d280faf34e5e7`。

- REA 的静态 PE 资源检查记录为 `ev_e05a6f74b2b803644001b4816e88c7e139ae65762c44f9f5c8b6686a689e8bc7`：该 DLL 没有 PE 资源目录。外部 VGUI 资源另行读取，不把空 PE 资源表解释为游戏没有界面。
- 同一归档的 `cstrike/resource/UI/ScoreBoard.res` 声明 `SectionedListPanel`，基础大小为 520 × 340，水平居中，顶部位置为 48。资源原文和 Git blob 身份保存在 `.local/cs16/research/scoreboard/`。
- 客户端字符串包含 `name`、`frags`、`deaths`、`ping`、T／CT 阵营名称及 `DEAD`、`BOMB`。静态引用附近的指令把 `#PlayerScore`、`#PlayerDeath`、`#PlayerPing` 分别与对应字段放在同一列创建路径；宽度参数分别为 40、46、46，状态列为 56。地址与反汇编来自本机已有的 LLVM `objdump`，不冒称 REA 原生反编译结果。
- 同一归档 `valve/resource/valve_english.txt` 把字段解释为 `Name`、`Score`、`Deaths`、`Latency`；`cstrike/resource/cstrike_english.txt` 提供阵营人数、阵亡和 C4 状态文本。
- `cstrike/config.cfg` 的 `TAB` 绑定为 `+showscores`，客户端同时包含 `+showscores`／`-showscores`。`valve/default.cfg` 属于另一套默认配置，不能拿其中的 Tab 绑定替代 CS 配置。

上述资源支持分区列表、列含义、尺寸参数和按住命令对。精确字体、像素比例、完整主题颜色和运行时排序尚未通过原版运行验证；当前浏览器样式按平面半透明面板、红蓝阵营、本人行高亮实现，排序使用击杀降序、死亡升序及席位作为稳定次序。

## 平台实现

计分板沿用玩家、状态、得分、死亡和延迟列，阵营标题显示实际人数及回合总分。不增加卡片商城式布局、常驻操作说明或 PC 关闭按钮。手机通过已有「更多 → 战绩」打开，保留可访问的关闭目标和面板内部滚动。

玩家延迟读取平台已有的网络 RTT，量化为整数毫秒；电脑显示 `BOT`，没有采样时显示 `—`。平台 RTT 的测量机制不等同于 GoldSrc ping，不伪造原版网络数值。状态只使用当前投影已经允许的数据，不读取敌方私有库存。

Tab 仍通过平台控制器映射处理，按下显示、松开关闭，repeat 不切换状态。计分板在阵亡和观战时仍可读取，实际移动和开火继续使用原控制权门槛；恢复控制前清除禁用期间的旧采样。切换标签页、失焦、打开购买菜单和切换全屏释放旧输入。

CS 在全屏内额外取消未被游戏控制器消费的 Tab／Shift+Tab 默认行为，仅调用 `preventDefault`，不阻断传播。全屏中的聊天、输入框和可编辑内容保持当前焦点与文字，不打开计分板；普通页面编辑器和其他游戏不受影响。Ctrl、Alt、Meta 等系统组合键不拦截。

## 验收边界与复跑

先登记 [失败场景](scoreboard-failures.md)，再建立浏览器 RED，随后修改实现。浏览器使用生产 `Board`、公共 `GameController`、`GameFullscreen` 和可信键盘／触摸事件；仅替换资源加载和 GPU 渲染。该夹具不替代游戏规则、正式地图、网络性能或实体手机验收。

复跑命令为 `CS16_SCOREBOARD_REPORT=replay node games/cs16/tests/scoreboard-browser.mjs`。结果、原生事件和截图保存在 `.local/cs16/verification/scoreboard/`；原始资源、摘要、字段引用和 REA 结果保存在 `.local/cs16/research/scoreboard/`。每轮关闭独立浏览器与服务并删除临时编译、缓存和 profile，保留必要证据。
