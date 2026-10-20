# 坦克方向素材修正结果

## 结论

用户反馈属实。旧生成器把每车型的四方向、两履带帧误当成两方向，再靠翻转生成另外两方向。原 ROM 实际为每车型 32 个 CHR tile；四个方向分别绘制，不能替换成镜像。另外，基础型、快速型、火力型的旧基址混用了相邻车型或向下帧。80 张双方配色帧中，64 张与原生渲染不一致。

修正后，五车型 × 四方向 × 两履带帧 × 两队配色共 80 张图全部与原 ROM 原生 OAM/PPU 取样逐像素一致。图集使用新的不可变 `battlecity/v2` 路径，已发布 v1 对象和声音不变。

Canvas 还有一处独立问题：位置用本地预测，贴图方向却仍读上一份服务端方向。已将取帧方向改为同一预测姿态的 `tank.dir`，避免网络确认前朝向滞后。

## 原版证据与边界

- 用户提供 ROM：`.local/battlecity/rom/Battle City (J).nes`，SHA-256 `a869aead5b6957fc62002ff9636e048cc34baf0100d629b07dc51aa18f220c0c`。
- 复用此前 REA 调查保留的 `.local/battlecity/rea/disasm.txt`。NROM-128 的 `$9FEB–$9FFF` 与运行地址 `$DFEB–$DFFF` 为镜像。代码用 `$A8,X & $F0` 加 `$B0,X` 得到车型／履带基址，再将 `$A0,X & 3` 左移三位作为方向偏移。`$DA7B` 把两个 8×16 精灵写入 OAM。
- 原生方向顺序为上、左、下、右；平台方向序号为上、右、下、左。素材按方向名称绑定，不直接复用原生序号。
- 运行 FCEUX 2.6.6 的原 ROM 渲染器。取样夹具只在 `$DFE9` 设置玩家的车型、方向、履带帧和位置 RAM，不修改 ROM 指令；读取实际 OAM、PPU 和原生截图。该取样证明渲染与素材排列，不把受控 RAM 场景宣称为自然完整对局。
- 原生取样与完整哈希：`.local/battlecity/reference/directions-v2/cases.json`，40 组 OAM/PPU/GD 截图，`record.lua` 可复查。双方红／蓝配色为项目既有映射；像素检查保留各颜色索引、轮廓和透明度，不声称该双方颜色是原游戏默认颜色。

| 项目车型 | CHR 基址 | 上／左／下／右偏移 |
| --- | --- | --- |
| 均衡 | `$20` | `$00 / $08 / $10 / $18` |
| 基础 | `$80` | 同上 |
| 快速 | `$A0` | 同上 |
| 火力 | `$C0` | 同上 |
| 装甲 | `$E0` | 同上 |

每个方向第 2 履带帧再加 `$04`，每帧按 TL=base、BL=base+1、TR=base+2、BR=base+3 组合，CHR bank 为 0，不执行镜像。

## 验收与复跑

失败清单先于实现写入 `PLAN.md`。先运行像素测试保留 64/80 失败，以及前端方向、预测姿态和 v2 URL 三项失败；随后修复实现。

```sh
. scripts/env.sh
python3 -B scripts/battlecity/capture-directions.py
.local/board-art-env/bin/python -B scripts/battlecity/prepare_assets.py --install-local
python3 -B scripts/battlecity/package_assets.py --atlas-v2
.local/board-art-env/bin/python -B scripts/battlecity/check-directions.py \
  --atlas .local/battlecity-pack/battlecity/v2 \
  --out .local/battlecity/directions-green
npm --prefix web test -- battlecity
python3 -B scripts/battlecity/verify-assets.py
```

- `.local/battlecity/directions-red/report.json`：修复前，80 帧中 64 帧像素不一致；保留旧图对照，删除冗长失败日志。
- `.local/battlecity/directions-green/report.json`：80 帧、0 不一致，包含原生 OAM 和每张期望像素 SHA-256。
- `.local/battlecity/directions-green/directions.png`：五车型双队配色的全部四方向和两帧，已人工查看。
- `.local/battlecity/directions-green/frontend.txt`：17 项前端测试通过，覆盖 80 个 Canvas 源矩形、预测姿态转向、版本 URL 和既有输入／碰撞预测。
- `.local/battlecity/directions-green/assets.txt`：历史 v1 13 个对象及新 v2 两个对象的 SHA／格式校验通过。图集 502×70、101 帧，现有裁切坐标保持不变。
- `.local/battlecity/directions-green/clean-checkout.json`：仅保留提交的脚本与 metadata、没有 ROM／本地媒体／dist 的隔离工作区，`verify-assets.py --dist` 通过；CI 不依赖 `.local` 或商业媒体。隔离临时目录已清理。
- 新图集 SHA-256：`640c854f1a4b561e07c51a557dffaeaeed267526eeb0a34033136aa7f7d7dc83`。两对象合计 45,518 bytes，清单 `games/battlecity/assets/manifest-v2.json`。

本次只修改素材与前端朝向，不修改服务端规则或物理。主任务已完成统一构建、320px／390px 原生触屏验收、OSS 不可变上传与源站／CDN 回读（`.local/battlecity/oss-v2-publish.json`）。tag／Release／生产结果统一记录在本轮发布记录。
