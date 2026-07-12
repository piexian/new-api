# 首页星空落地 + 友链悬浮球 - Task List

## Implementation Tasks

- [x] 1. **双前端路径映射（Foundation）**
    - [x] 1.1. default `resolveAppRoute`
        - *Goal*: 逻辑 key → default 路径，禁止写死 classic 路径
        - *Details*: 新增 `web/default/src/lib/frontend-routes.ts`：`home|dashboard|pricing|rankings|sign_in|sign_up|keys|wallet|docs`；`docs` 读 `status.docs_link`；导出 `resolveAppRoute(key)`
        - *Requirements*: FR-1.1, FR-1.4
    - [x] 1.2. classic `resolveAppRoute`
        - *Goal*: 同一 key 映射到 classic 路径
        - *Details*: 在 `web/classic/src/helpers/` 增加对齐表（dashboard→`/console`，sign_in→`/login` 等）；禁止 default 路径写死
        - *Requirements*: FR-1.1, FR-1.4
    - [x] 1.3. 映射单测/冒烟
        - *Goal*: 关键 key 双端 path 正确
        - *Details*: 对 default helper 做最小单测或 node 断言；classic 同步冒烟
        - *Requirements*: FR-1.1；Testing Strategy

- [x] 2. **友链 option + 校验 + status（Foundation）**
    - [x] 2.1. 后端 option 键与校验
        - *Goal*: `console_setting.friend_links` + `friend_links_enabled` 可存可校验
        - *Details*: 对齐 `api_info` 模式；字段 name/url/icon/description/order/enabled；非法 URL、超 30 条拒绝；ValidateConsoleSettings 分支
        - *Requirements*: FR-4.1, FR-4.2, FR-4.5
    - [x] 2.2. status 下发
        - *Goal*: 前端可读已启用友链列表
        - *Details*: status 仅下发 enabled 且总开关开的条目，按 order 升序
        - *Requirements*: FR-4.1, FR-4.2
    - [x] 2.3. 校验单测
        - *Goal*: 非法 JSON/URL/超限有测试
        - *Details*: Go 单测覆盖合法/非法/空列表
        - *Requirements*: Testing Strategy

- [x] 3. **default 星空首页 + 统一能力 Tab（Default）**
    - [x] 3.1. 星空背景
        - *Goal*: 密星空；浅色降噪；深色可更密；`prefers-reduced-motion`
        - *Details*: `starfield-background.tsx`；不遮挡正文对比度
        - *Requirements*: FR-2.5, FR-3.7
    - [x] 3.2. Hero：基址 + 主 CTA
        - *Goal*: 标题下 BASE URL 复制；主 CTA 仅控制台/模型广场
        - *Details*: `server_address`/origin；复制反馈；CTA 用 `resolveAppRoute`；无快捷入口大卡；不自建顶栏（PublicLayout 顶栏）
        - *Requirements*: FR-1.2, FR-2.1, FR-2.2, FR-2.3, FR-2.6
    - [x] 3.3. 统一 Tab：Chat|Responses|Claude|Gemini|Codex|Claude Code
        - *Goal*: HeroTerminal 样式合并协议演示与第三方接入
        - *Details*: 协议在前、接入在后；浅/深跟随主题（禁止固定黑终端）；可复制区无注释；Codex=config.toml+auth.json+responses；Claude Code=禁用变量+模型；轮播仅协议
        - *Requirements*: FR-2.8, FR-2.9, FR-2.10, FR-3.*
    - [x] 3.4. 公益服务条最下方
        - *Goal*: 三条并排在协议+接入之后
        - *Details*: 极速响应？/稳定高可用？/公益免费；非 50+ 指标
        - *Requirements*: FR-2.4
    - [x] 3.5. Home 组装与清理
        - *Goal*: 默认首页结构符合 IA；去掉冲突旧区块
        - *Details*: 保留 custom home content 分支；默认布局：Hero→统一 Tab→公益条；仍用既有顶栏
        - *Requirements*: FR-1.2, FR-2.*

- [ ] 4. **default 悬浮球 + 系统设置友链（Default）**
    - [x] 4.1. FloatingBall
        - *Goal*: 左下友链球；可拖动；localStorage；视口夹紧
        - *Details*: 展示 icon/name/description；拖拽阈值；resize 夹紧；非第二顶栏；总开关关则隐藏友链区
        - *Requirements*: FR-4.4–FR-4.9, FR-1.5
    - [x] 4.2. 系统设置友链 UI
        - *Goal*: 运营可配置友链
        - *Details*: 总开关、列表、Dialog 增改、order 上下移、icon 预览；双端设置入口按项目惯例
        - *Requirements*: FR-4.1–FR-4.3

- [ ] 5. **classic 对齐（Classic）**
    - [x] 5.1. classic Home 星空简化版
        - *Goal*: 与 default 结构/文案/CTA/基址/统一 Tab 意图对齐
        - *Details*: CSS 星点即可；BASE URL；统一 Tab；公益条底部；复用 HeaderBar，不自建顶栏
        - *Requirements*: FR-1.*, FR-2.*, FR-3.*; Frontend Parity
    - [x] 5.2. classic FloatingBall + 设置友链
        - *Goal*: 行为与 default 对等
        - *Details*: 左下、拖动、记忆、夹紧；设置页友链 CRUD
        - *Requirements*: FR-4.*; Frontend Parity

- [x] 6. **验证 + 本地提交 + 阶段总结（Verify）**
    - [x] 6.1. 自动化验证
        - *Goal*: 改动相关 test/typecheck/lint 通过
        - *Details*: Go 友链/校验测试；default typecheck/lint（触及包）；缺测补测
        - *Requirements*: Testing Strategy；AGENTS.md
    - [x] 6.2. 行为冒烟
        - *Goal*: 验收清单可勾
        - *Details*: 顶栏不冲突、BASE URL 复制、Tab 浅/深、Codex responses、公益底部、悬浮球拖动记忆
        - *Requirements*: requirements §6
    - [x] 6.3. 本地 commit + phase summary
        - *Goal*: 本地提交；写 phase 总结；**禁止 push 远程**
        - *Details*: Conventional Commit；`docs/specs/phase-*-summary.md`；不 `git push`
        - *Requirements*: 用户明确约束

## Task Dependencies

- Task 1 与 Task 2 可并行（Foundation）
- Task 3 依赖 Task 1（CTA 路径）；建议 Task 2 完成后做 Task 4
- Task 4 依赖 Task 2（友链数据）
- Task 5 依赖 Task 1–4 的接口约定（可在 default 主路径稳定后对齐）
- Task 6 依赖 Task 1–5 完成

## Estimated Timeline

- Task 1: 2 hours
- Task 2: 3 hours
- Task 3: 5 hours
- Task 4: 3 hours
- Task 5: 4 hours
- Task 6: 2 hours
- **Total: ~19 hours**
