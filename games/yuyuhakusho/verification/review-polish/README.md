# 试玩反馈修正

- MD 默认 A＝J、B＝K；C＝L，X／Y／Z＝U／I／O。FC 默认和用户已保存配置保持原有独立规则。HTTP 默认／保存／隔离检查与 MD 输入验证通过。
- 去掉手柄底图的灰色装饰线、重复按键轮廓，保留 12 个交互按钮；封面改为浦饭幽助的灵丸动作。
- 选人提供 11 个已提取的原版头像，合计 4073 字节；选人名称与原版 HUD 统一为「浦饭」，说明首次使用全名。
- 创建房间与准备阶段共用八个场地图示和当前预览，使用已验证场景渲染的裁切缩略图，合计 98924 字节。
- 字段左边缘统一，手机专用样式残留的 8 px 内边距已删除。浮层避开父容器裁切，关闭时同步更新状态，支持 Esc、外部点击和焦点离开。

最终实际平台记录：[public-interface-fixed/result.json](public-interface-fixed/result.json)。真实注册、创建房间、选择场地与人物、读取实际后端配置及控制器设置均通过；桌面与 320 px 手机的字段名／选择框左偏差均为 0 px，无横向溢出。

截图：[桌面场地](public-interface-fixed/desktop-arenas.png)、[手机准备界面](public-interface-fixed/mobile-waiting.png)、[手机场地](public-interface-fixed/mobile-arenas.png)、[手机选人](public-interface-fixed/mobile-characters.png)、[手机控制器](public-interface-fixed/mobile-controller.png)。

真实组件预览的原生指针、完整头像、人物 ID、Esc 和边界检查见 [all-pickers-fixed/result.json](all-pickers-fixed/result.json)。实际场景字段与人物字段的选择结果仍经原有 `prepare` 接口提交，房主／本人／电脑席位权限沿用原逻辑。

重复实际平台验收：`python3 -B scripts/yuyuhakusho/check_review_interface.py --label <未使用的标签>`。该脚本使用已经验证的本地 dist，创建独立数据库及测试账号，不修改实际试玩房间。头像与场景图示分别由 `export_character_portraits.py` 和 `export_arena_previews.py` 生成；对应 JSON 记录源文件、角色／场地 ID、大小与 SHA-256。

实现前默认键位与缺少配图的记录、旧手机错位及过早点击旧浮层的失败均保留。最终以真实平台两种视区的记录为准。联机与慢端修正另见 [四客户端验收](../four-human-network/README.md)。本次仍是本地试玩更新，没有 OSS 上传、线上发布或部署。
