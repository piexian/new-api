# NodeLoc 内置 OAuth 登录

提供 NodeLoc 注册、登录、个人账号绑定和管理员解绑。协议依据：[NodeLoc OAuth 对接文档](https://docs.nodeloc.com/api-reference/introduction)。

## 创建 NodeLoc 应用

1. 打开 [OAuth 应用管理](https://www.nodeloc.com/oauth-provider/applications) 创建应用（NodeLoc 对创建者有会员等级要求，本站不据此限制登录用户）。
2. 仅申请 `openid profile` 权限，不申请 `email`。
3. 回调地址填写为本站的 **服务器地址 + `/oauth/nodeloc`**，例如 `https://api.example.com/oauth/nodeloc`，没有 `/api` 前缀和末尾斜杠。
4. 保存 Client ID 和仅显示一次的 Client Secret。

## 配置 new-api

1. 启动后 GORM 自动为 users 表新增 `nodeloc_id` 字段和唯一索引；未绑定值为 NULL。
2. 系统设置中保存正式的服务器地址（生产用 HTTPS，不含路径、查询参数）。
3. 填写 Client ID、Client Secret 并保存；Secret 保存后不再回显，留空保留原值。
4. 核对界面显示的回调 URL 与 NodeLoc 应用登记值完全一致。
5. 打开"允许通过 NodeLoc 账户登录 & 注册"。

请从服务器地址对应的站点访问；别名域名会在授权前提示切换到正式站点，避免会话 Cookie 与 state 丢失。

## 账号行为

- 首次授权按本站注册开关和邀请码规则创建账号；已绑定身份可继续登录，不受注册开关影响。
- 以 NodeLoc 数字用户 ID 识别账号，不按用户名合并；不读取邮箱、不同步头像、不设等级限制。
- 已有账号在个人设置中绑定；NodeLoc ID 已被占用时绑定失败。
- 管理员可在用户绑定管理中查看或清除 NodeLoc 绑定。
- 禁用和已删除账号不能借此登录；Access Token 仅用于当次回调，不持久化。

## 接口及配置

| 项目 | 值 |
|---|---|
| 提供商标识 | `nodeloc` |
| 配置项 | `nodeloc.enabled`、`nodeloc.client_id`、`nodeloc.client_secret` |
| 默认状态 | 关闭 |
| 公共状态字段 | `nodeloc_oauth`、`nodeloc_client_id`、`nodeloc_redirect_uri` |
| 用户字段 | `nodeloc_id` |
| 授权端点 | `https://www.nodeloc.com/oauth-provider/authorize` |
| Token 端点 | `https://www.nodeloc.com/oauth-provider/token` |
| 用户信息端点 | `https://www.nodeloc.com/oauth-provider/userinfo` |
| 浏览器回调 | `/oauth/nodeloc` |
| 后端回调处理 | `/api/oauth/nodeloc` |

常见失败原因：未先保存凭据便启用、服务器地址无效、NodeLoc 应用回调不匹配、从别名域名发起授权、上游准入拒绝。修改配置后需重新发起授权，避免复用过期授权码。
