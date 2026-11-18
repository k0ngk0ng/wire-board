# 仙剑奇侠传

当前交付范围为《仙剑奇侠传 98 柔情篇》的单人复刻及个人长期存档。玩家手动操作全部原版队员，保存后释放本人席位，使用原账号即可继续。多人席位、共同阅读、领队交接和等待原队友不再作为本次功能。

**主线剧情进度：24/24。** 按用户确认，以开发加血流程通过为剧情验收依据，不再要求原版难度托管通关。完整分段与证据见 [剧情进度表](STORY_PROGRESS.md)；全部主线及同次真实结局页面已通过，继续功能与交付检查。声音、界面、存档和性能另行验收，不计入剧情段数。

**开发加血流程：24/24。** 最新核心已从真实新游戏连续完成最终章、原版影片和谢幕，分段证据见 [剧情进度表](STORY_PROGRESS.md)。开发时使用用户授权的[加血测试构建](DEVELOPMENT_TESTING.md)推进剧情和存档验证；测试构建、存档与报告均独立存放，加血通过计入本次剧情验收，正式游戏数值仍按原版保留。

人数配置和前端目录已收敛为 1 人，单人页面不再显示领队或共同阅读状态。原版三队员战斗夹具已验证同一玩家手动选招、战斗中保存和全新运行器续行；真实浏览器已验证原版开场的 23 次阅读、PC 无虚拟按键、手机原生触摸、个人保存／继续及陌生账号拒绝。完整剧情与性能验收仍未完成。主程序已接入正式组合入口，真实进程保存／重启续玩通过；分发包和镜像状态见[版本保留与应用接入](NATIVE_VERSIONS.md)。

头像登记已修复共享缓冲区复用问题：新的资源读取会使重叠的旧图像身份失效，避免把 RGM 头像识别为 FBP 背景。原版头像、道具图标和未知类型人物缓冲区的三种读取／复用回归通过。新构建已从单人新游戏推进到与林月如相遇、两场原版战斗、受伤恢复及苏州城内，新增相遇段覆盖 76 次阅读和 10 次续档；受伤场景的真实浏览器已核对正确头像、背景、人物形象和个人续档。完整流程仍未完成。

苏州招亲与林府夜变已从新游戏重跑通过：救刘晋元、客栈过夜、唤醒灵儿、逍遥单独打擂、林府剧情、灵儿失踪和月如正式同行。本段覆盖 216 次阅读、3 场原版战斗、22 次全新运行器续档，113 文奖励不重复。修复了缩编队伍后开战动画保留旧菜单游标所触发的错误校验；等待阶段保留原版状态，正式菜单仍拒绝已离队角色及越界游标。

当前单人验收可重复执行以下命令。原版剧情测试会从新游戏重跑，并生成受伤和月如同行浏览器入口使用的个人存档；原版三队员战斗是独立夹具，不代表剧情已到三人队伍：

```sh
. scripts/env.sh
python3 -B games/pal/build/native.py
python3 -B games/pal/build/native.py --battle-fixture 1
node games/pal/tests/portrait-reuse-native.mjs
go test -v ./games/pal/tests -run '^TestSoloOriginalPartyManualAndSavedContinuation$' -count=1 -timeout=3m
go test -v ./games/pal/tests -run '^TestOriginalCampaignSuzhouTournamentAndManorSave$' -count=1 -timeout=18m
go test -v ./games/pal/engine -run '^TestOriginalSoloDuelAfterPartyShrinks$' -count=1
go test -v ./internal/server -run '^TestPALOriginalSoloPrivateSaveRestartHTTP$' -count=1 -timeout=3m
npm --prefix web test pal
npm --prefix web run build
```

真实页面分别验证开场、受伤和月如正式同行。启动一个夹具后，在另一个终端执行对应脚本；脚本结束后会关闭夹具并清理自己的浏览器 profile：

```sh
. scripts/env.sh
PAL_BOARD_BROWSER=1 PAL_BOARD_ENTRY=solo go test -v ./internal/server -run '^TestPALBoardBrowserFixture$' -count=1 -timeout=35m
# 另一个终端
. scripts/env.sh
python3 -B games/pal/tests/solo-board-browser.py

# 受伤场景：先启动夹具，再在另一个终端执行带 --injury 的脚本
PAL_BOARD_BROWSER=1 PAL_BOARD_ENTRY=solo-injury go test -v ./internal/server -run '^TestPALBoardBrowserFixture$' -count=1 -timeout=35m
python3 -B games/pal/tests/solo-board-browser.py --injury

# 月如正式同行：同样使用独立终端运行脚本
PAL_BOARD_BROWSER=1 PAL_BOARD_ENTRY=solo-companion go test -v ./internal/server -run '^TestPALBoardBrowserFixture$' -count=1 -timeout=35m
python3 -B games/pal/tests/solo-board-browser.py --companion
```

结果、截图和确切入口存档位于 `.local/pal/solo-board-browser/`、`.local/pal/solo-injury-browser/` 与 `.local/pal/solo-companion-browser/`。月如同行页面已验证 PC 1280×800、320px 手机 320×200 的最大比例全屏画布、覆盖按键、原生操作、本人续档及陌生账号拒绝。本阶段摘要与归档见 `.local/pal/verification/tournament-stage.json`，此前单人范围记录为 `solo-scope-stage.json`。个人存档 HTTP 验收另覆盖 SQLite 读取／保存失败回滚、保存后切换游戏、服务器重启、本人立即继续和旧页面拒绝。

以下保留此前多人研究的阶段记录及复现命令，用于追溯原版引擎、资源和存档实现，不代表当前开放多人模式。旧多人存档和旧原生构建的研究存档不转换为个人存档。当前验收范围以 [ACCEPTANCE.md](ACCEPTANCE.md) 为准，进度见 [IMPLEMENTATION.md](IMPLEMENTATION.md)。

## 原版资源

```sh
python3 -B games/pal/assets/fetch.py
. scripts/env.sh
go test -v ./games/pal/resources -count=1
```

下载目录为 `.local/pal/original-98/`，固定来源提交和各文件的大小、Git 内容摘要及 SHA-256 保存在 `assets/source-manifest.json`。下载器只取得数据文件，不下载或运行原版可执行文件；已缓存文件同样校验全部内容。

目前读取 295 个场景、5,077 个事件对象、565 个 Windows 格式对象、43,503 条脚本指令和 13,513 条消息。`resources` 保留原始 GBK 文本和控制码，不用编造文本替代原版消息。地图、人物和战斗图集、背景共 24 个 YJ2 压缩数据块已与 SDLPAL C 参考逐字节核对；图像和声音资源仍需按平台加载与发布规范转换。

原版图块、人物和调色板已转换为可校验的按需资源，位于 `.local/pal/visual/pal/v1/`。本地目录仅用于验收；当前图像、地图、字体、音源及结局媒体均已发布到 CDN，实际页面使用已验证映射，见 [视觉资源发布](VISUAL_PUBLICATION.md) 与 [音频验收](AUDIO_TESTING.md)。

来源仓库提供的附加媒体使用独立清单和目录，包含 6 个 AVI 演出和 8 个 Ogg CD 音轨，共约 204 MB。下载并检查缓存内容与音视频流：

```sh
. scripts/env.sh
python3 -B games/pal/assets/fetch.py --media
python3 -B games/pal/tests/media_e2e.py
```

下载器校验 `assets/media-source-manifest.json` 的固定提交、大小、Git 内容摘要和 SHA-256；文件位于 `.local/pal/original-98-media/`。媒体验收需要本地 `ffprobe`，结果为 `.local/pal/verification/media.json`。这些资源尚未接入平台音频或浏览器演出。

## 多人核心验收

先准备原版资源和原生规则构建，再执行：

```sh
. scripts/env.sh
python3 -B games/pal/build/native.py
go test -v ./games/pal/tests -run OriginalCampaignCooperative -count=1 -timeout=5m
go test -race -v ./internal/server -run PALOriginalSharedSaveHTTP -count=1 -timeout=3m
```

结果位于 `.local/pal/verification/cooperation-{1,2,3}.json` 和 `shared-save-http.json`。测试使用真实原版核心，按原版的 23 次阅读停顿确认；不代表完整菜单、战斗或全流程已经完成。字体与浏览器画面对照见 [原版核心说明](native/README.md#对话与字体)。

### 客栈后续剧情与基础战斗托管

规则层已通过普通移动、共同阅读推进到原版客栈大堂，完成苗人包下客栈的 10 次阅读停顿并获得 500 文钱；随后沿原版楼梯和厨房通道取菜，给楼上客人送菜并取得桂花酒。1～3 人继续验证向醉道士使用酒品、原版道具消耗、人物离场和山神庙约定；对话确认、端菜人物形象和非空行囊选择均可在全新运行器中继续。三人还已接到买虾任务、取得原版 50 文钱并离开客栈进入村庄；任务对话和村庄位置均可保存续行，尚未验证买到虾。行囊续档另核对完整绘制指令，奖励不重复发放。测试路线只用于剧情验收，不作为电脑读取未来剧情的策略。

三人剧情验收已继续到码头：鱼嫂说明风浪大、没有鲜虾，张四说明没有捕到鱼。与水生叔交谈后，张四会改为谈论仙灵岛；返回村庄，香兰按原版剧情告知李大娘昏倒。码头、传闻和昏倒对白的部分阅读，以及返回村庄后的进度，均可在全新运行器恢复，钱数保持 550 文。

1～3 人另已通过原版求药和登岛剧情：返回病房听洪大夫诊断、小虎提供仙灵岛线索，离开病房后领取破天锤并服下忘忧散，再请张四驾船前往仙灵岛。赠物后再次交谈，行囊仍显示一把破天锤；数量通过公开物品名称和原版数字图块核对。诊断、求药线索、赠物、登岸对白及行囊均可在全新运行器恢复，多人保留部分已读状态；乘船准备脚本和登岸位置同样可续行。赠物至登岛段每种人数覆盖 6 次存档，钱数保持 550 文。

1～3 人继续完成入口区域的四场原版战斗，再乘移动荷叶逐一破坏六座石像，核对破天锤消耗和通路开启。战斗选招使用本席可见信息及普通输入，结算画面合计显示 12 点经验、0 文钱，钱包保持 550 文。战斗中、选中破天锤时、荷叶移动中、破坏三座石像后、最终共同阅读及通路开启后，各人数分别在全新运行器续档。多人的物品权限和部分已读状态保持一致。地图路线仅用于 E2E 验收，连续探索托管仍只验收到交酒；这些后续场景的浏览器画面、头目和队员加入仍待验证。

1～3 人另已通过仙灵洞天入口、与灵儿相遇及水月宫取药。剧情中的原版人物形象变化可在脚本中途保存，继续后恢复正常形象；入口、引路和取药对白保留部分已读状态。紫金丹按原版赠送，重复交谈后行囊仍显示一颗，非领队不能操作共享行囊。相遇至宫外段每种人数覆盖 4 次续档，取药段覆盖 6 次续档，钱包保持 550 文。取药阶段灵儿尚未入队；这一段为普通输入驱动的规则层验收，后续实际入队验证见下文。

1～3 人已继续完成水月宫留宿、原路返回林地、张四返航、回村入客栈、向李大娘使用紫金丹，以及夜间醒来。每种人数覆盖 4 次全新运行器续档，分别检查选药菜单、治疗对白、治疗后的行囊和夜间位置；多人保留部分已读状态，旧输入被拒绝。治疗后确认紫金丹已消耗，空行囊仍可保存。路线允许触发原版怪物和通道脚本，战斗仍使用普通指令。

夜间山路验证发现部分敌人图集在名义帧数之外还保存一帧有效动画。规则资源和客户端图集现保留这帧，名义帧数保持原值；新增原版 C 光栅对照及逐像素导出检查。

1～3 人已从全新开局继续完成山神庙学剑、御剑演出、原路返回、暗道回房及客栈救援战，灵儿按原版脚本加入队伍。单人操作两名队员；双人和三人分别由前两席控制李逍遥、赵灵儿，三人共同阅读名册保持不变。每种人数另覆盖学剑阶段 5 次续档、入队阶段 3 次续档，包含演出中途和临时入队；入队后领队交接、旧输入拒绝和整队移动通过。救援战及此前战斗现使用生产电脑策略，探索路线仍由 E2E 普通输入驱动。

基础战斗策略只使用本席位公开画面和普通输入：在根菜单使用原版单角色「自动选招」，由原版选择已学、可负担的攻击法术或普通攻击，并在原版消费按键前等待。接管未完成的道具、仙术或杂项菜单时先返回，避免误花资源。独立原版战斗覆盖 1～3 席纯电脑、混合席位、手动接管和全新运行器续档；真实 HTTP／SQLite 测试另覆盖添加电脑、主动开启托管、保存重启、旧页面拒绝和取消托管。

1～3 人的实际救援战检查点已由生产电脑完成，核对法力消耗、原版 96 文奖励、施法中全新运行器续档及排队指令被手动接管取消。真实 HTTP／SQLite 案例另覆盖三位原账号托管施法中保存、首位确认后冻结、释放席位、重启后全员确认、旧页面拒绝和原版救援结果；测试入口使用真实剧情检查点，完整探索托管仍未完成。

1～3 人还已从全新开局继续完成李大娘的护送交代、张四返岛对白、登船和再次登岸。原版登岸脚本把队伍调整为灵儿在前、李逍遥在后，控制权仍按角色保留；第三席继续参与共同阅读。每种人数另覆盖 5 次续档，包含部分阅读、乘船脚本和队伍重排，并验证重排后整队移动。

返岛后继续经过原版林地和已打开的迷阵，完成水月宫遇袭、灵儿暂时离队、姥姥临终、告别和归队。1～3 人各完成 32～34 次共同阅读及 7 次全新运行器续档，包含实际战斗、离队、临终对白和归队；李逍遥的控制者保持不变，灵儿归队后按空闲席位重新分配，原真人名册继续共同阅读。结果为 `campaign-homecoming*.json`，不代表完整剧情或连续探索托管。

1～3 人继续返航回村，完成第二次客栈伏击、灵儿离队、李大娘介入及回房剧情。战斗由生产电脑正常获胜，钱数从 677 增至 1,013；每种人数覆盖 5 次续档，包含船程阅读、战斗、单人队伍和回房对白，核对原真人名册、李逍遥控制权及旧输入拒绝。结果为 `campaign-ambush*.json`，终点为第 2 场景、第 12 张地图。

1～3 人从新游戏继续完成当夜、次日、李大娘的远行交代、打开包袱、灵儿归队和方老板送至苏州入口。包袱通过原版行囊使用，显示两份金创药、两份还神丹及绣花鞋、玉镯、铁剑、手卷；包袱消耗，500 文只发放一次。每种人数完成 53 次共同阅读和 8 次全新运行器续档，覆盖夜间与次日阅读、选中包袱、奖励后的阅读、归队、船程对白、航行脚本和到达位置。结果为 `campaign-departure*.json`，这仍是普通输入驱动的剧情验收。

战斗电脑会为生命值低于 65% 的存活队友寻找可用治疗仙术，只读取公开生命值和菜单高亮文字，通过原版按键选法术、选队友。队友已提交的治疗会覆盖对应目标，避免两人反复治疗同一个人；其他队员继续攻击或治疗尚未覆盖的伤员。三人真实伏击检查点另验证选中「观音咒」时保存、全新运行器恢复及实际生命增加。此策略尚未覆盖复活、补充真气和完整剧情战斗。

救援电脑检查点由首次入队测试生成；返岛测试会从全新开局重跑前置流程：

```sh
. scripts/env.sh
go test -v ./games/pal/tests -run '^TestOriginalCampaignFirstPartyJoinAndOwnersSave$' -parallel=3 -count=1 -timeout=15m
go test -v ./games/pal/tests -run '^TestOriginalRescueBattleBotsMagicAndSavedTakeover$' -parallel=3 -count=1 -timeout=4m
go test -v ./games/pal/tests -run '^TestOriginalCampaignEscortReturnBoatAndPartyOrderSave$' -parallel=3 -count=1 -timeout=15m
go test -v ./games/pal/tests -run '^TestOriginalCampaignInnAmbushAndSinglePartySave$' -parallel=3 -count=1 -timeout=18m
go test -v ./games/pal/tests -run '^TestOriginalCampaignFarewellBundleAndSuzhouSave$' -parallel=3 -count=1 -timeout=20m
go test -v ./games/pal/tests -run '^TestOriginalAmbushRecoveryBotSavedHealing$' -count=1 -timeout=4m
go test -v ./internal/server -run '^TestPALOriginalRescueMagicAutoplaySaveHTTP$' -count=1 -timeout=3m
```

结果位于 `.local/pal/verification/battle-bot-rescue*.json`、`campaign-return-island*.json`、`campaign-homecoming*.json`、`campaign-ambush*.json`、`campaign-departure*.json`、`battle-bot-recovery.json` 和 `rescue-magic-save-http.json`。远行测试从新游戏重跑前置流程；客栈伏击测试生成独立治疗验收使用的真实战斗检查点。`departure-stage.json` 记录当前范围、构建身份、归档结果和复现命令；`homecoming-ambush-stage.json`、`escort-bot-stage.json` 与 `rng-first-party-stage.json` 保留此前阶段的构建记录。

基础探索策略只使用当前公开碰撞范围、可见人物、已显示对白及先前观察到的交互，通过普通移动和确认推进。记录最多保留 16 次交互、一个附近人物、一个最近位置、32 个通道端点和 256 个行走位置；返程导航另保留最多 1,024 个可操作位置及 4,096 条已观察路径。不读取原生内存、未来脚本或验收路线坐标。导航记录脚本位移的两端，避开不可步行的插值位置；返程时寻找通道附近的实际触发范围，允许沿当前可达地面缩短旧路线。反向路径仍须由普通输入和原版碰撞验证。刚交谈的人物在门口行动时，短暂等待其原版自动脚本继续。

1～3 席已从原版起床检查点自动走到大堂、完成共同阅读并取得 500 文奖励，还已从包店后的原版检查点自动完成李大娘、醉道士和厨房取菜互动。「把桌上酒菜端上楼」的公开对白会让电脑重新检查场景物件。行走记录、任务意图及接任务时的阅读进度均可在全新运行器中恢复；多人恢复时保留已确认和未确认的席位。真实 HTTP 另验证主动托管、添加电脑、保存重启和手动接管。双人、三人取消非当前席位的托管时，保留当前玩家已接受的移动／攻击及操作时钟。后续探索任务、恢复／施法策略、剧情战斗及电脑完整通关仍未完成，正式注册继续受完整验收约束。

自动送菜已覆盖 1～3 席：从原版端菜检查点走到客房，取得桂花酒，并通过普通行囊操作核对可用酒品；移动中和酒品对话均可保存续行。道具策略另根据已显示的讨酒对白和当前菜单高亮文字寻找桂花酒，普通输入负责接近、转向、选择和使用。1～3 席已验证保留大蒜等无关物品、手动接管后重新托管，以及未生效的菜单方向键、选中酒品和饮酒对话的续档；释放旧输入时只重算扫描线索，保留原版菜单选择与任务。空行囊会退出检查并继续探索。这些分段验证另外覆盖接管、道具选择和恢复边界。

1～3 席另已从 `Start` 全新开局连续托管至原版山神庙约定，途中完成包店奖励、取菜、送菜和交酒，只使用普通输入、阅读确认和计时器，不载入预制剧情检查点或路线提示。每种人数均两次在全新运行器中续档；多人额外保留部分已读状态。客房出口和楼梯返程不会再把脚本位移终点当成精确门口，也不会因旧路线绕进厨房而反复进出。这仍是开场到交酒的连续验收，后续任务及完整电脑通关尚未完成。

准备原版核心后运行剧情验收；六石像路线另需导出的原版地图，独立战斗验收需构建对应夹具：

```sh
. scripts/env.sh
go test -v ./games/pal/tests -run '^TestOriginalCampaign(LeavesInnBedroom|InnGuestsRewardSurvivesSave|KitchenTraySavedContinuation|GuestWineInventorySave|DrunkardWineUseAndSave|MarketQuestAndVillageSave|HarborShrimpAndBoatmanSave|FairyRumorAndAuntCollapseSave)$' -count=1 -timeout=10m
go test -v ./games/pal/tests -run '^TestOriginalCampaign(DoctorAndIslandMedicineSave|HammerAndIslandLandingSave)$' -count=1 -timeout=10m
go test -v ./games/pal/resources -run '^TestOriginalVisualExport$' -count=1
go test -v ./games/pal/tests -run '^TestOriginalCampaignSixStatuesAndSavedMaze$' -count=1 -timeout=12m
go test -v ./games/pal/tests -run '^TestOriginalCampaignFairyMeetingAndPalaceEntranceSave$' -count=1 -timeout=12m
go test -v ./games/pal/tests -run '^TestOriginalCampaignPalaceMedicineAndNamedFairySave$' -count=1 -timeout=12m
go test -v ./games/pal/tests -run '^TestOriginalCampaignReturnMedicineAndAuntCureSave$' -parallel=3 -count=1 -timeout=15m
go test -v ./games/pal/tests -run '^TestOriginalFieldBot' -count=1 -timeout=12m
go test -v ./games/pal/tests -run '^TestOriginalContinuousAutoplayThroughWineQuest$' -count=1 -timeout=10m
python3 -B games/pal/build/native.py --battle-fixture 1
go test -v ./games/pal/tests -run '^(TestOriginalBattleBot|TestOriginalIdleReclaim)' -count=1 -timeout=4m
go test -v ./internal/server -run '^TestPAL(OriginalBattleBotsHTTP|IdleReclaimKeepsHumanFieldInputHTTP|FieldAutoplaySavedContinuationHTTP)$' -count=1 -timeout=3m
```

结果位于 `.local/pal/verification/campaign-inn-{exit,guests}.json`、`campaign-kitchen-tray.json`、`campaign-guest-wine.json`、`campaign-drunkard-wine.json`、`campaign-market-village.json`、`field-bots-inn-{1,2,3}.json`、`field-bot-kitchen-{1,2,3}.json`、`field-bot-delivery-{1,2,3}.json`、`field-bot-wine-{1,2,3}.json`、`field-bot-wine-missing.json`、`battle-bots-{1,2,3}-{all-bots,takeover,mixed}.json` 和 `battle-bots-http-{1,2,3}.json`。客栈与交酒剧情的单人、双人结果分别带 `-1`、`-2` 后缀；单人不生成混合席位结果。真实 HTTP 的探索与接管记录位于 `.local/realtime-efficiency/pal-field-autoplay-save.json` 和 `pal-idle-reclaim-{2,3}.json`。测试日志保留原版对话、各席位操作数量和奖励，独立战斗结果不代表剧情通关。

三人码头及传闻结果为 `.local/pal/verification/campaign-harbor-shrimp.json` 和 `campaign-fairy-rumor-collapse.json`。

返回用药结果为 `.local/pal/verification/campaign-return-medicine.json` 及单人、双人的 `-1`、`-2` 文件。早期返回用药构建记录见 `return-medicine-stage.json`，当前构建见 `rng-first-party-stage.json`。敌人额外帧的原版 C 对照可用 `node games/pal/tests/rle-oracle.mjs` 重新生成，再运行 `go test -v ./games/pal/resources -count=1`；31 个光栅向量包含第 2 号敌人图集的额外帧。

御剑演出与首次入队的阶段记录见 `.local/pal/verification/rng-first-party-stage.json`，当前证据见 `escort-bot-stage.json`。首次入队可用 `go test -v ./games/pal/tests -run '^TestOriginalCampaignFirstPartyJoinAndOwnersSave$' -parallel=3 -count=1 -timeout=15m` 重新执行；报告为 `campaign-night-lesson*.json` 和 `campaign-first-party*.json`。原版 `PAL_swprintf` 已链接，未知接口继续报错；窄字符转换、其他演出阶段和跨原生构建存档迁移仍未验收。

### 原版 RNG 演出与续档

`native/reference/rngplay.c` 是同一固定提交的原始演出实现，来源摘要位于相邻 `manifest.json`。构建只在派生代码插入绘制和等待观测，保留原版播放循环、分段端点、结束判断与计时。12 段原版动画的 1,464 个有效画面与 C 解码逐帧核对；每段末尾的空记录保持为停止标记。

服务端规则归档只保留动画索引与有效帧标记，共 42,604 字节；客户端按需加载无损压缩的完整绘制指令。演出开始前的场景和备份以公开绘制指令保存，原始像素不进入规则包。御剑演出的 410 个有效画面在 320px 竖屏、820px 横屏和桌面 Canvas 分别与原版 C 像素一致，画布保持原比例和最大可用尺寸。独立原版播放循环逐帧计时、25,627 毫秒总时钟和 4 个全新运行器续档点通过；这些浏览器检查仍是演出夹具，完整平台剧情页面还需扩大验收。

准备原版资源后，按顺序生成参考、资源和演出夹具：

```sh
. scripts/env.sh
python3 -B games/pal/tests/rng-oracle.py
go test -v ./games/pal/resources -count=1 -timeout=4m
python3 -B games/pal/build/fonts.py
go run ./games/pal/build/cmd/rules
python3 -B games/pal/build/native.py --rng-fixture 1
python3 -B games/pal/build/native.py --rng-fixture 1 --reference-font
PAL_NATIVE_WASM=.local/pal/rng-fixtures/1-reference/rules.wasm PAL_ORIGINAL_PIXELS=1 PAL_REFERENCE_FONT=1 node games/pal/tests/rng-oracle.mjs
PAL_NATIVE_WASM=.local/pal/rng-fixtures/1/rules.wasm node games/pal/tests/rng-oracle.mjs
go test -v ./games/pal/engine -run '^TestOriginalRNGSegmentedPlaybackSavedContinuation$' -count=1 -timeout=4m
node games/pal/tests/rng-render-e2e.mjs
python3 -B games/pal/tests/visual-browser.py --rng
```

像素和计时结果位于 `.local/pal/verification/rng-{pixels,archives,native-save,native-projection,render-js}.json`，浏览器结果及截图位于 `.local/pal/rng-browser/`。AVI、音效和音乐播放、其他剧情演出、完整电脑流程与发布仍未完成。

求药和登岛结果为 `.local/pal/verification/campaign-island-medicine.json` 和 `campaign-island-landing.json`；单人、双人文件带 `-1`、`-2` 后缀，同人数的传闻及昏倒前置流程也会重新验证。

入口战斗和六石像机关结果为 `.local/pal/verification/campaign-six-statues.json`，单人、双人文件同样带 `-1`、`-2` 后缀。记录包含结算画面的经验／铜钱、普通战斗输入数、机关数量、道具消耗及 6 个续档阶段。

相遇及取药结果为 `.local/pal/verification/campaign-fairy-meeting.json` 和 `campaign-palace-medicine.json`，单人、双人文件带 `-1`、`-2` 后缀。相遇记录只包含事件标记、人物形象编号和阅读数量。

全新开局连续托管结果为 `.local/pal/verification/continuous-wine-{1,2,3}.json`，记录实际对白、输入数量、存档次数、奖励及完成的原版事件。

音效与 MIDI 源检查：`python3 -B games/pal/tests/audio_source_check.py`。结果为 `.local/pal/verification/audio-sources.json`，覆盖来源摘要、363 段 PCM 音效的采样参数／样本摘要、87 份 MIDI 文件的轨道边界，以及原始归档对齐字节。此检查不代表声音播放、MIDI 合成、无损压缩或 CDN 发布已经接入。

YJ2 解压算法依据 SDLPAL `yj1.c` 移植，保留 GPL-3.0 许可和原作者声明，许可正文位于 [LICENSE.SDLPAL](resources/LICENSE.SDLPAL)。参考输出可通过 `python3 -B games/pal/tests/resource_oracle.py` 重新生成；此命令仅编译本地只读参考源码中的解压例程。

## 平台页面与覆盖式按键验收

准备原版核心和[客户端场景、字体资源](native/README.md#对话与字体)后，在一个终端启动真实房间夹具：

```sh
. scripts/env.sh
npm --prefix web run build
PAL_BOARD_BROWSER=1 go test -v ./internal/server -run '^TestPALBoardBrowserFixture$' -count=1 -timeout=35m
```

在另一个终端执行操作与布局验收。第一条命令写入后续布局验收需要的测试账号记录；夹具运行期间保留这些账号，测试不使用真实玩家账号。

```sh
. scripts/env.sh
python3 -B games/pal/tests/board-browser.py
python3 -B games/pal/tests/layout-room-browser.py
python3 -B games/pal/tests/layout-room-browser.py --fullscreen
```

结果和截图分别位于 `.local/pal/board-browser/`、`.local/pal/layout-room/` 和 `.local/pal/fullscreen-room/`。手机原生触控验证覆盖共同阅读、全屏内保存和取消保存；桌面与手机横竖屏核对同一个 Canvas、完整原版比例和覆盖式控件。全屏画布须达到视口内原比例的最大尺寸，平台保存与退出入口也不占用画面高度。

苏州入口另有三位真实账号的正式 `Board` 验收，入口使用剧情测试产生的原版续档。验证桌面键盘、手机原生触摸、第三席不能越权移动、全员保存释放席位和三位原玩家共同续档。PC 不显示虚拟按键；320×820 竖屏画布为 320×200，820×320 横屏画布为 512×320，半透明按键覆盖在画面内，旋转保留同一个 Canvas。此验收不代表完整浏览器剧情或响应性能达标。

先运行上述远行剧情测试生成续档，准备前端构建后启动夹具，再在另一个终端执行浏览器测试；脚本结束后会自动关闭夹具：

```sh
. scripts/env.sh
PAL_BOARD_BROWSER=1 PAL_BOARD_ENTRY=departure go test -v ./internal/server -run '^TestPALBoardBrowserFixture$' -count=1 -timeout=35m
# 另一个终端
. scripts/env.sh
python3 -B games/pal/tests/departure-board-browser.py
```

结果、截图和确切的浏览器入口存档保存在 `.local/pal/departure-board-browser/`。脚本会关闭自己创建的浏览器并清理测试 profile；`result.json` 保留入口摘要、实际移动和存档结果。

同一房间夹具和原账号仍在运行时，可验证连接建立前的 HTTP 输入。测试先保持真实 WebSocket 握手等待，使用原生键盘短按，再故意延迟第一次 HTTP 确认 800 毫秒；方向脉冲仍只移动一个原版步长，后续松开可以继续通过 HTTP 或恢复的 WebSocket 到达。这是正确性测试，不作为响应延迟达标证据。

```sh
. scripts/env.sh
python3 -B games/pal/tests/input-transport-browser.py
```

结果、原生键盘时间和实际网络请求记录位于 `.local/pal/input-transport-browser/`。同一测试还在连接恢复后发送带物理键码的浏览器原生按下／松开，验证持续移动，以及服务器应用松开后位置保持停止。规则层的 1～3 席到达时间、排队、有效期和存档验证见[方向短按说明](native/README.md#方向短按与持续移动)。

以下多人战斗入口为历史研究记录，不能作为当前单人交付或剧情通关证据。三人战斗单独使用原版战斗夹具。先构建，再启动另一套房间夹具；随后在另一终端运行浏览器测试：

```sh
. scripts/env.sh
python3 -B games/pal/build/native.py --battle-fixture 1 --fixture-field 18
PAL_BOARD_BROWSER=1 PAL_BOARD_ENTRY=battle go test -v ./internal/server -run '^TestPALBoardBrowserFixture$' -count=1 -timeout=35m
# 另一终端
. scripts/env.sh
python3 -B games/pal/tests/battle-board-browser.py
```

战斗结果位于 `.local/pal/battle-board-browser/`，覆盖三个真实账号的角色权限、连续方向短按、合击部分确认保存、原队友续档和原版奖励。测试结束后，在对应输出目录创建 `stop` 文件可正常结束夹具。

## 响应性能测量

预算与当前测量范围见 [PERFORMANCE.md](PERFORMANCE.md)。当前使用正式原版核心的服务端规则包和两个独立单人房间；各自从真实开场进入客栈，手机直接轻触人物周围。先准备已发布规则包与当前前端，再启动夹具：

```sh
. scripts/env.sh
python3 -B games/pal/build/prepare-server.py
PAL_BOARD_BROWSER=1 PAL_BOARD_ENTRY=performance PAL_VISUAL_CDN=1 go test -v ./internal/server -run '^TestPALBoardBrowserFixture$' -count=1 -timeout=24m
```

在另一个终端执行。每轮重新启动夹具；测量期间不运行编译或其他重型测试。

```sh
. scripts/env.sh
PAL_VISUAL_CDN=1 python3 -B games/pal/tests/performance-browser.py --tag original-solo-current
touch .local/pal/cdn-performance-board-browser/stop
```

结果、原始样本和截图写入 `.local/pal/performance/original-solo-current/`。未达标时命令失败；加 `--baseline` 只允许记录失败而正常退出，结果仍为 `false`。覆盖正常连接、约 200 毫秒 RTT、另一独立房间及 320px 手机。实际 Canvas 场景像素、全部 rAF、原生 Ping/Pong、输入确认、消息大小和规则回调分别记录，不能用 ACK 代替可见响应。此前双人／虚拟按键性能和网络生命周期脚本保留为历史研究，不作为当前单人操作验收。

存档编解码另行隔离测量，使用当前原版核心、12 份连续客栈状态和独立的解码运行器：

```sh
. scripts/env.sh
PAL_CODEC_BENCH=1 PAL_CODEC_TAG=original-current go test -v ./games/pal/engine -run '^TestNativeSnapshotCodecBudget$' -count=1 -timeout=3m
```

结果为 `.local/pal/performance/codec-original-current.json`；编码和解码各 5 毫秒的预算已通过。完整性能仍须覆盖菜单／战斗响应、持续动画、随机丢包和其他阶段的恢复，不能由客栈或独立编解码结果推断。

## 存档基础能力

公共模块通过 `SupportsSave` 显式接入，其他游戏不会自动开放存档。实时模块还须提供 `ResumeSaved`，在恢复时清理旧控制，保持原进度和剩余时间。详见 [公共存档说明](../../docs/saved-games.md)。

本地只读参考源码位于 `/Users/jason/Sources/github.com/sdlpal/sdlpal`。资源表格式对照其 `global.h`、`global.c`；后续剧情与战斗分别对照 `script.c`、`battle.c` 和 `fight.c`。
