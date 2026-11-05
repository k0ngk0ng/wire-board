# 共享呈现状态与客户端 schema 2 的先行失败范围

本清单先于对应实现及测试。原生／Wasm 完整对局与真实浏览器为主要验收；少量独立验证覆盖格式、身份、原子性和持久 owner。

- 新资源经 JS parse／stringify、trim 或换行改变后，仍以服务器 metadata 的原字节身份接受。规则、精灵、HUD、metadata、选中场地、aux 与 descriptor 任一错绑或缺失。
- 同时持久保存旧图块槽／HUD 像素与新 SpriteSlots／HUD refs；将 World.GraphicCache 复制到另一缓存 owner；从像素相等反查不唯一 ref。
- 旧 schema 1 的有效存档、字段顺序或完整字节改变；错误地将旧像素存档猜成 schema 2 refs。新存档包含 tile_slots／HUD pixel，或遗漏新的 metadata／descriptor 身份。
- callback-time draw 的暂时渲染与共享 Advance 的 cache／slot 不一致；80 片预算、culling、SkipTiles、预载继承、slot wrap、后续重用 slot 的 guard 或魔性之森 aux 顺序错误。
- HUD base／bar／glyph／portrait ref 越界、缺少 key 或 alias 被猜错；材质化修改了唯一持久 ref。
- 在开局加载期间真正推进规则、音效、输入历史、缓存或帧；预览 fork 改动权威 frame 0；原数据未就绪导致 Canvas 首帧与公共 ready 互等。
- 恢复 checkpoint 错 arena／options／identity／descriptor／epoch／uint64 frame，或错误地用 16 位游戏计数作为网络帧；恢复后 renderer 没有当前帧，或需要注入历史参考像素才能继续。
- 在同一有效 tick 之外悄悄切换 automation；清零或 mask 掉 World.Controllers 历史；错误输入／晚期渲染失败提交半帧；restore 或 preview 修改调用方内存。
- 原 49 条已冻结完整轨迹（14 legacy、28 arena、7 pan）的世界、缓存、槽、HUD 与像素改变；新 schema 2 中途存档恢复或非零 pan 恢复不连续。
- 旧资源入口和 schema 1 兼容路径受到新网络身份策略影响；新加载器预加载不相关场地或向正式服务器嵌入 client payload。

边界：本阶段不注册大厅模块，不部署，不宣称完整网络延迟或音频合成已完成。普通网络消息仍只使用输入／确认／摘要；完整 checkpoint 是单独恢复通道。
