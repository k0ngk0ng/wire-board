# 四代试玩运行记录

2026-11-11。已解包并尝试在本机运行原版试玩，尚未取得可验证的游戏画面。

| 内容 | 实际字节数 | 十进制 MB |
| --- | ---: | ---: |
| 下载的 InstallShield 安装包 | 15,083,878 | 15.1 |
| 解包后的 112 个游戏文件 | 22,739,262 | 22.7 |
| France 赛道资源，含试玩提供的昼夜文件 | 4,408,489 | 4.4 |
| P911 与 FM50 两套车辆资源合计 | 8,868,656 | 8.9 |

以上不包含 Wine、编译缓存和辅助工具，也不是网页转换后的体积。试玩说明明确写明驾驶改装 Porsche 911 Turbo，场景为欧洲村庄与乡间道路，每次启动交替使用白天和夜间。试玩只提供有限内容，15.1 MB 不是完整版大小。

安装包先由其自解压程序释放 InstallShield 文件，再使用 unshield 1.6.2 提取 112 个文件并恢复目录。unshield 源码来自官方项目，归档 SHA-256 为 `a937ef596ad94d16e7ed2c8553ad7be305798dcdcfd65ae60210b1e54ab51a2f`。游戏文件位于仓库内 `.local/racing-nfs-reference/wineprefix/drive_c/NFSHSD/`，没有修改游戏可执行文件。

运行使用本机已有的 Wine 11.0，通过 Rosetta 执行 x86 Windows 游戏。Wine 前缀、临时目录和缓存放在本次仓库目录内；前缀内用户文件夹改为本地目录，Z 盘映射到仓库。补齐试玩使用的 32 位安装和渲染注册表项后，使用 `nfshsd.exe -PatchRestart` 通过启动参数检查。具体参数解析的 REA 静态证据为 `ev_cece4227602a50b86e522f4b4e0f9655f36f282ebf152b600761d6e3901c76b5`，见 [startup-decompile.json](startup-decompile.json)。这份伪代码只证明解析分支的静态结构。

运行观察：参数、安装信息和渲染配置检查已通过。旧游戏的磁盘空间检查出现「less than 1MB」警告；确认后游戏继续执行，Wine 随后报告 `GL_INVALID_FRAMEBUFFER_OPERATION`。仅凭进程持续执行，不能证明赛道画面或操控正常。

当时的捕获阻碍是主机锁屏：`CGSessionCopyCurrentDictionary` 返回 `CGSSessionScreenIsLocked=1`。系统窗口捕获失败，ScreenCaptureKit 返回 `-3811`；Wine 内部截图只得到黑色，失败图片已删除。REA 原生 UI 捕获还报告 Wine 子进程缺少可用的启动时间，因此未产生运行画面的 REA 证据。后续捕获尝试仍未取得可验证的原版赛道画面。2026-11-12 已关闭本任务启动的 Wine 与 NFS 测试进程，不再自动重启。现有黑图、外部参考图和网页重制截图均不能作为原版实机画面。

实际大小和运行限制记录在 [artifact-manifest.json](artifact-manifest.json)，启动过程的相关调用与图形错误保存在 [runtime-startup.log](runtime-startup.log)。

复刻改用原始模型、贴图、赛道和参数的离线转换与逐项校验，运行验收针对独立网页实现。没有取得原版加速、制动或转向实测，因此不声称操控与原版物理完全一致。
