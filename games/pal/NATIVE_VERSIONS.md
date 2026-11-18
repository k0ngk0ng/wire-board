# 原生版本保留与续档

长期存档继续遵循[平台存档规则](../../rules/saved-games.md)。本机制按存档中的原生核心和资源摘要选择对应运行器，保持原字节身份；新游戏使用明确指定的当前核心。旧内存不交给新核心解释，也不改写存档摘要。

## 构建身份与不可变归档

- 新核心包含 `wire-board.pal.build` 元数据和只读 `wb_build_info` 原生接口。格式、原版／开发加血配置、用途必须一致；元数据不能单独把加血核心标为原版。
- `engine/legacy-runtimes.json` 只登记此前已验证的原版及 HP9999 剧情核心。未分类旧核心可继续用于独立参考测试，但不能自动加入长期存档版本目录。
- 结局专用构建使用 `ending-fixture` 用途，编译编号为 5。构建清单必须同时记录 `endingFixture: true`，不能伪装成普通剧情核心；运行器目录拒绝接入该夹具。包含结局能力的正常剧情候选仍使用 `campaign`，须从真实新游戏验收，不能接续其他核心的内存。
- 构建完成后，`rules.wasm` 和 `build.json` 保存到 `.local/pal/runtime-versions/<实际 SHA-256>/`。重复构建校验已有字节，不覆盖原件；并发写入使用工作区内的临时目录和原子目录重命名。已有归档损坏时明确失败。
- 当前目录只归档原生核心及构建元数据。正式发布仍须同时保留其资源版本、客户端不可变资源地址及备份；不能把本地核心归档当作完整发布验收。

本轮新候选位于 `.local/pal/runtime-candidates/original/` 和 `.local/pal/runtime-candidates/hp-9999/`，未替换当前剧情测试使用的旧核心。

| 配置 | 已保留旧核心 | 新候选核心 |
| --- | --- | --- |
| 原版 | `7ca83b5bcf13e71c1020ef287d9817e5675adf2cbdf7c627644f4cb3566dd03a` | `041c1c47061eff8cb950bd2278ec40232915827e52af2a80a8f8c9ee01704b78` |
| 开发 HP9999 | `68d893938bc623a69a33d9ca402741b04a7be2191c7349f4081b5e94b755bec7` | `770df9f8fbed730d03f9043cf8ee19221858c5c3b6d13b8e7f44de361df182b8` |

新候选增加构建身份接口与元数据，玩法数值沿用各自原版或已授权的开发配置。血池波动尚待修复，不在本轮通过范围内。

## 游戏接入

分别创建当前核心和各个保留核心的 `engine.Runtime`，资源文件系统绑定其实际版本。使用 `engine.NewRuntimeCatalog(current, retained...)` 建立不可变目录，再通过 `pal.NewVersionedModule(catalog)` 接入平台。

目录以 `(BuildID, ResourceID)` 为键。同一核心的不同资源版本可以并存；缺少任一摘要对应的运行器时拒绝续档，原档保持不变。原版与开发加血配置不能放进同一个目录；字体参考、战斗夹具和其他未分类核心不能作为剧情核心登记。运行器生命周期由创建者管理。

## 已通过的验证

- 两个真实不同构建的原版核心：旧地图／菜单继续结果与旧核心直接执行逐字节一致，新游戏保留新核心摘要。
- 开发配置的第 9 段真实存档通过旧核心继续；原版目录拒绝开发存档，开发目录拒绝原版存档。
- 同一核心不同资源摘要的分派、缺失核心／资源、校验损坏、重复登记、独立并发事务和编译配置伪装拒绝。
- 真实账号、HTTP、SQLite 与计时器：升级重启后缺失旧核心时续档失败，持久化行字节不变、账号仍空闲；重新加载旧核心后本人立即继续原阅读页，原 `MatchID`、队伍、财物和控制边界保留，无结算记录。
- 缺失版本的玩家提示说明原档已保留；内部构建摘要不作为普通玩家操作文案。
- 上述引擎及 HTTP 流程包含 race 检查。它们证明保留运行器方式的续玩，不证明原生内存迁移，也不代替正式发布包和备份／回滚验收。

## 复现

仓库根目录执行：

```sh
. scripts/env.sh
python3 -B -m unittest discover -s games/pal/build -p test_runtime_archive.py -v
python3 -B games/pal/build/native.py --out .local/pal/runtime-candidates/original
python3 -B games/pal/build/native.py --test-battle-hp 9999 --out .local/pal/runtime-candidates/hp-9999
PAL_VERSION_CATALOG=1 go test -race -v ./games/pal/engine -run '^TestOriginal(RetainedRuntimeCatalogContinuations|RuntimeCompiledProfileRejectsForgedStamp)$' -count=1 -timeout=8m
PAL_VERSION_CATALOG=1 go test -race -v ./internal/server -run '^TestPALRetainedNativeVersionsRestartHTTP$' -count=1 -timeout=4m
```

结果为 `.local/pal/verification/runtime-catalog-e2e.json` 和 `native-version-http.json`。当前已验证的所有运行器使用同一份 98 原始资源；资源打包方式改变后的身份和发布验收须另行覆盖。

## 服务端规则包加载前失败场景

1. 正式启动仍依赖原始图像、音频、视频或字体文件；规则包携带真实客户端像素、采样或影片。
2. 剥离后的原生文件、文本、绑定顺序或规则资源身份改变，导致同一核心的原始存档无法继续。
3. 未登记、损坏或伪造的包能够指定任意旧资源标识；通过改写存档 BuildID／ResourceID 冒充兼容。
4. 包缺文件、带额外文件、路径越界或符号链接；压缩／解压大小不受限。
5. 新游戏原生内存、对白、音频日志、场景或首次行动与原始数据加载路径不同。
6. 读取已有后段存档后改变剧情、队伍、物品、随机数或原生内存；源目录在初始化后仍被访问。
7. 调用者修改传入包数据后影响正在运行的游戏，或混用原版与开发加血核心。

服务端使用经过原始数据等价验证的规则包，按包字节摘要绑定原资源标识。包只包含规则表、GBK 文本、碰撞／高度及精灵尺寸等计算数据；实际像素、声音和影片仍由客户端直接从 CDN 加载。正式目录只接入原版配置，开发加血包只用于独立验收。

## 正式加载入口的实现前失败场景

1. 缺少核心、规则包或发布清单仍返回可注册游戏，或者遇错静默换用原始素材目录。
2. 文件字节、声明摘要、原资源标识或版本名不符；重复核心、路径越界或额外客户端素材进入正式包。
3. 开发 HP9999、字体／战斗／结局夹具被正式入口接收，或只信清单中的「原版」文案而不核对实际编译身份。
4. 当前核心与保留核心混用原生内存；新开局选择旧核心，或恢复旧档时强制改写标识。
5. 初始化部分失败后留下运行器；关闭或加载失败改变已有存档；初始化后继续读取来源文件。

正式包由规则 ZIP、原版剧情核心及版本清单组成。加载入口返回独立的模块与运行器生命周期，只有真实原版 `campaign` 核心能注册。基础封装验证不代替完整剧情、客户端素材边界或应用正式注册。


## 已验证的服务端规则包

规则包仅 584,355 字节，含 13 份已剥离像素的原生规则文件，以及原始 `word.dat`、`m.msg`。包 SHA-256 为 `c63f058aa4c050deafa5d68171027a4f420ce7b58d6bd2276d846f3a7312c508`，继续绑定原资源标识 `58b3fddfd1bcdc37b6816cfd3ddba8a821683ff6f13b4cd5cbbab6f8ffe7255c`；登记于 `assets/authority-release.json` 和 `engine/authority_catalog.go`。未知包、缺项、额外文件、越界路径、符号链接及超限展开均拒绝，不允许由传入包自行声明一个旧资源标识。

原始数据路径与规则包路径已通过完整开场 23 页的全部原生内存和投影比较，3 处原始数据存档在新规则包运行器中原样恢复；同一音频候选第 15 段的真实存档及后续原生步骤也一致。包初始化后不再读取原始目录，调用者修改输入包字节不影响运行器。结果为 `.local/pal/verification/authority-archive-equivalence.json`，竞态记录为 `authority-archive-race-run.txt`。

实际 HTTP／SQLite 验证从原始数据保存，重启后改用仅接收规则包字节的运行器，原账号、房间、阅读页、队伍与金钱保持，加载门槛、陌生账号拒绝、旧输入拒绝、继续阅读及手动回退通过。结果为 `.local/pal/verification/authority-http.json`。没有修改存档中的 BuildID 或 ResourceID。

```sh
. scripts/env.sh
python3 -B games/pal/tests/authority-build-e2e.py
python3 -B games/pal/build/authority.py
PAL_AUTHORITY_E2E=1 go test -race -v ./games/pal/engine -run '^Test(OriginalAuthorityArchiveEquivalentAndSavedContinuation|AuthorityArchiveRejectsUnknownMissingExtraPathsAndOversizedFiles)$' -count=1 -timeout=8m
PAL_AUTHORITY_HTTP=1 go test -race -v ./internal/server -run '^TestPALAuthorityArchiveRestartAndManualCheckpointHTTP$' -count=1 -timeout=5m
```

生成器验证全部 104 份来源文件及 15 个输出文件的逐字节摘要，重复生成相同 ZIP；保护目录和符号链接拒绝通过。正式加载封装 `server.OpenProductionBundle` 同时校验每份核心真实编译身份及源包，接收原版 `campaign`，拒绝真实 HP9999 构建。保留原版旧核心后，第 4 段真实旧存档可继续且原字节不变；缺核心、改字节、错误资源、重复核心和夹带客户端素材均拒绝。结果为 `production-authority-bundle.json`。

```sh
. scripts/env.sh
PAL_PRODUCTION_E2E=1 go test -race -v ./games/pal/tests -run '^TestProductionAuthorityBundleRejectsDevelopmentAndPreservesLegacySave$' -count=1 -timeout=5m
python3 -B games/pal/tests/server-package-e2e.py
python3 -B games/pal/build/server-package.py
```

当前物理包位于 `.local/pal/server-packages/a9234deb9d91a84e0d273f0d0ef3e90e81a7fbdf052c41ed4fcb0c91a35566ac/`，只含版本清单、规则 ZIP 和原版服务端核心。该核心用于服务端规则计算，不是浏览器运行时。重复打包保留原目录及字节，拒绝开发构建或额外客户端素材；核验入口为 `build/cmd/verify-production`。上述验证不代表完整应用镜像、后端正式注册或部署已经完成。

原资源导出 E2E 已改用 Go 临时目录；实际清单与字体条目不会再被回归检查覆盖。`tests/visual-isolation-e2e.py` 验证完整 3,485 个工作输入摘要保持不变，结果为 `.local/pal/verification/visual-export-isolation.json`。工作素材仍用于后续客户端 CDN 发布；临时导出随测试清理。

## 应用正式组合入口的实现前失败场景

1. 主程序仍使用不含仙剑的兼容模块列表，或者初始化失败后静默省略仙剑。
2. 规则目录缺失、被替换或夹带客户端素材，主程序却创建数据库或开始监听。
3. 应用注册开放多席位、丢失保存／回退能力、控制器定义或重复注册游戏。
4. 规则运行器在 HTTP 请求结束前关闭，退出后仍有可运行的原生实例，或者重复关闭导致错误。
5. 完整应用进程保存退出后，重启无法用本人账号继续，陌生账号能够发现／恢复存档，或恢复时绕过加载门槛。
6. 原对局、阅读位置、队伍、财物或控制代次在重启后改变；旧页面输入仍被接受；手动回退不能返回原保存点。
7. 二进制使用开发加血核心，或者依赖工作区中的原始媒体目录作为正式回退。

应用接入验收使用真实组合入口和实际编译的主程序。进程存档通过不代替正式客户端载荷剥离、镜像、发布与部署验收。

## 服务端规则分发的实现前失败场景

1. 分发记录对应旧原生源码或未实际复编译的核心；改变输出路径导致摘要变化，却改写已有核心身份；编译／优化失败后没有恢复被部分覆盖的已验收核心与构建记录。
2. 重复发布、增加保留核心或只改变清单时，重新上传未变规则 ZIP／核心或替换原地址。
3. 下载收到错误 MIME／编码、截断、损坏内容、超限响应或 gzip 炸弹；未经逐文件校验就登记就绪。
4. 清单含越界路径、重复核心或额外图像／音频，分发器写入工作区外或跟随符号链接。
5. 下载一半失败就留下可启动的半包，或重复准备覆盖已有包；当前版本遗漏以前已保留的核心。
6. CI 或镜像没有携带规则目录，主程序只能从开发素材启动；tag 阶段又重新生成／下载另一套规则。

## 当前应用接入与分发结果

`internal/production.Open` 先校验既有模块身份，再加载仙剑规则包；初始化失败不返回部分列表。主程序使用该入口注册仙剑及平台控制器，在 HTTP 请求完成、应用关闭后释放原生运行器。缺省读取可执行文件旁的 `pal/`，本地可通过 `PAL_RULES_DIR` 指定目录；没有原始素材目录回退。

真实主程序已在无原始素材的独立工作目录通过单人创建、加载门槛、个人保存、进程关闭／重启、本人继续、陌生账号拒绝、同一原对局／阅读／队伍／财物保持、旧页面拒绝和手动回退。测试同时验证缺包在创建数据库前失败、运行器关闭及重复关闭。外层竞态与已有启动／控制器契约共 5 项通过；同次规则包的保留核心恢复了真实第 4 段林府终点，未修改旧档。证据为 `.local/pal/verification/production-application.json` 和 `production-application-retained-run.txt`；测试子进程为实际主程序，外层 `-race` 不表示子进程也已插桩。

当前服务端分发 ID 为 `0245efd5cc4dc948d9250e12391fc623ab9c16008be079f1da94ad3c93210764`，包含版本清单、规则 ZIP、当前原版核心 `4aeeca…` 和保留原版核心 `7ca83b…`。逐文件 URL 与摘要记录于 `assets/server-release.json`；这些是服务端计算依赖，不是客户端媒体。增加保留核心时只新增保留核心和变化清单，未变规则 ZIP／当前核心继续使用首批 URL；重复发布 4 个文件全部复用，新增和上传字节为 0。旧分发记录和对象继续保留。

原版核心在原构建路径重新编译后，字节仍为 `4aeeca84606436fc071b1b9004014ca11767ec048e8c3be5d411c6c0370d6ba3`。发布工具每次先实际重编译并绑定原生源码摘要，再核对原版 `campaign` 身份；不同输出路径中的原版断言文件名可能改变 Wasm 字节，不能据此改写旧核心标识。外部优化器部分覆盖程序后失败的真实 CLI 注入验收通过，旧核心、构建记录及发布记录保持原字节。

`prepare-server.py` 仅消费已在本地发布和回读通过的服务端依赖，核对当前源码、固定对象摘要、大小及格式；不上传，也不更新发布验收状态。默认在 `.local/pal/prepared-server/<分发 ID>/pal/` 创建完整目录，不覆盖旧分发；显式输出用于干净的应用候选目录。下载失败不留下半包，重复准备复用已校验的字节。解码、超限 gzip、损坏、符号链接、夹带客户端文件等 14 项拒绝已验证。

```sh
. scripts/env.sh
python3 -B games/pal/tests/server-distribution-e2e.py
python3 -B games/pal/tests/server-attestation-e2e.py
python3 -B games/pal/build/prepare-server.py
python3 -B games/pal/tests/server-retention-e2e.py --publish
python3 -B games/pal/tests/release-package-contract.py
```

`--publish` 是显式远端发布入口；普通准备只下载已有依赖。当前分发和零重复上传的结果分别为 `server-publication-retained-first.json`、`server-publication-retained-repeat.json` 和 `server-retention-e2e.json`。首次增加保留核心新增 2 个对象、335,336 字节；原 ZIP 和当前核心的 URL 保持。默认本地准备在版本改变后使用新的分发目录，旧目录保留。

CI 构建和源码镜像构建已接入服务端规则准备，最终运行目录与主程序默认路径一致。版本 tag 从同一已验证构建提取原二进制、`pal/` 和许可。许可正文随应用包及镜像维护。

## 本地候选镜像与回滚演练

使用发布流程的运行阶段构建并启动真实 Linux amd64 镜像。镜像层只包含应用二进制、服务端规则包和许可；客户端素材通过清单地址跳转，原始资源和服务端规则不能通过 HTTP 下载。候选核心为 `fa8d77…`，保留 `4aeeca…`、`7ca83b…`。相同应用二进制分别使用上一份已发布规则包和本次候选规则包，验证旧档随升级继续、新档随重启继续、本人权限、跨游戏、禁止旧输入和回退到手动存档。SQLite 在线备份在独立数据卷恢复成功，带新存档的数据卷保持完整。

另以仓库上一版正式镜像 `v1.60.0` 演练首次加入仙剑：原有对局随升级保留；将含仙剑存档的数据库副本交给 `v1.60.0` 时，程序明确拒绝启动，新版仍可从原字节恢复存档。回到 `v1.60.0` 只能使用独立数据卷中的上线前备份。一旦接收了仙剑存档，优先恢复能读取当前存档和规则版本的应用；需要使用上线前备份时，保留升级后的数据库，不能用备份覆盖它。首次更新前应控制流量，避免健康验收完成前创建新版存档。

复现脚本和 JSON 证据位于 `.local/pal/verification/linux-image-local-v1/`。依次运行 `build-package.py`、`assemble.py`、`container-save-e2e.py`；`released-rollback-e2e.py` 另需按报告摘要拉取 `v1.60.0` 镜像。结果见 `candidate-image-boundary.json`、`container-save-e2e.json` 和 `released-rollback-e2e.json`。这些是本地工作树和隔离数据卷的结果，网页模块使用离线清单验证剥离与跳转。正式发布仍须验证同一提交的 CI 构建、完整 CDN 回读、最终发布镜像及实际服务器。
