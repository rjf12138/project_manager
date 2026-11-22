# 设计稿（未实现）

本目录下都是**尚未实现**的设计文档，代码里没有对应功能。放在这里是为了和
[已实现的参考文档](../package.md)区分开——阅读时请勿当作待办计划。

| 文档 | 主题 | 状态 |
|---|---|---|
| [registry.md](registry.md) | 远端共享仓库：HTTP 协议、`index.json` 索引、上传/下载/鉴权、客户端缓存 | 暂缓实施 |
| [platform.md](platform.md) | 平台（OS / 架构）维度：`<os>-<arch>` 标识、包内 `lib/<platform>/` 布局、安装回退规则 | 未实现 |

当前实际行为：

- 只使用**本地**仓库 `~/.project_manager/repo/`，没有远端存储；
- 包只对应**构建它的那台机器**，配置里没有 OS / 架构维度。

已实现的内容见 [usage.md](../../usage.md)、[../package.md](../package.md)、[../config.md](../config.md)。
