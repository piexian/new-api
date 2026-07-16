# Review

## Result

- Critical: none.
- Warning: none.
- Info: `web/default` 全仓 `bun run lint` 受既有基线错误阻塞；本次变更文件的定向 oxlint 通过。
- Review mode: single-agent self-review, per the repository opt-in override.

## Verified behavior

- 注册必填凭证统一支持普通邀请码、一次性邀请码和多次注册码。
- 用户创建与凭证消费处于同一事务，注册码使用条件更新避免并发超用。
- OAuth、微信和密码注册共用凭证解析逻辑。
- 注册码不能用于余额或套餐兑换，且创建时不依赖支付合规确认。
- 两套前端均支持注册码创建、展示、注册输入和一次性邀请码生成。
- `registration_code_id` 与 `registration_source` 经用户 JSON 响应返回。
- `exports/` 与本地工作流确认文件不会进入版本控制。

## Validation

- `go test ./...`
- `go test ./controller ./model`
- `web/default: bun run typecheck`
- `web/default: bun run i18n:sync` (all locales missing/extras/untranslated = 0)
- `web/default: targeted oxlint and oxfmt checks`
- `web/default: bun run build`
- `web/classic: targeted ESLint and Prettier checks`
- `web/classic: bun run i18n:status`
- `web/classic: bun run build`
- `git diff --check`
