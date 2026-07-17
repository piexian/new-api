# Cloudflare Cache Rules Review

## Conclusion

当前规则应收缩为“动态内容全部绕过，仅缓存 `/static/*`”。`/pricing` 和 `/rankings` 是按主题 Cookie 生成的 SPA HTML 外壳，不是可共享的数据页；`/api/pricing` 与 `/api/rankings` 也都存在用户或查询维度的响应差异。

## Current Effective Behavior

Cloudflare Cache Rules 可叠加，同一设置由最后一条匹配规则决定。因此当前规则实际结果为：

| Request | Matching rules | Effective result |
| --- | --- | --- |
| `/` | 1 | bypass |
| `/pricing`, `/rankings` | 1, 3 | cache HTML for 10 minutes |
| `/static/*.js`, `/static/*.css` | 1, 4 | cache for 1 year |
| `/assets/*.js` | 1, 2, 4 | rule 4 overrides rule 2; cache for 1 year |
| `/logo.png`, `/favicon.ico` | 1, 4 | cache unversioned assets for 1 year |
| `/api/pricing?x=1` | none of the explicit URI patterns | falls back to other/default cache behavior |

Public verification on 2026-07-17:

- `/` returned `CF-Cache-Status: DYNAMIC` and `Cache-Control: no-cache`.
- `/pricing` and `/rankings` refreshed at the edge and then returned `CF-Cache-Status: HIT` with `Cache-Control: max-age=600`.
- Requests to `/pricing` with `new-api-frontend=default` and `new-api-frontend=classic` returned the same classic HTML, the same origin request ID, and the same CSP nonce.
- `/static/js/index.91160b5d6e.js` and `/logo.png` returned `CF-Cache-Status: HIT` with `Cache-Control: max-age=31536000`.

## Repository Evidence

- `router/web-router.go` selects both HTML and static files from the `new-api-frontend` cookie. HTML receives a fresh CSP nonce and `Cache-Control: no-cache` at the origin.
- Both builds reference `/static/js/*` and `/static/css/*`; neither build emits a public `/assets/*` directory. Source imports such as `@/assets` are bundled into build output and are not `/assets/*` URLs.
- Current shared paths under both `dist/static` trees have identical content; theme-specific assets use distinct content-hashed filenames. Static cache keys therefore do not need the frontend-theme cookie.
- `controller/pricing.go` filters models and group ratios according to the authenticated user's usable groups.
- `controller/rankings.go` varies by `period` and authenticated viewer; `service/rankings.go` already maintains a five-minute in-process snapshot.

## Risks

1. Rule 3 caches theme-dependent HTML and reuses CSP nonces across visitors. It directly recreates the wrong-theme and lazy-chunk failure.
2. Rule 4 applies a one-year browser TTL to stable filenames such as `/logo.png`; Cloudflare purge cannot clear copies already stored in browsers.
3. Rule 4 ignores origin cache directives across a broad extension list. With no status-code exception, its default Edge TTL can also apply to cacheable 404 responses; configure 4xx/5xx as not cacheable explicitly.
4. Rule 1 uses `http.request.uri`, which includes the query string. Exact wildcard values such as `/api/pricing` do not safely cover query variants. It also does not provide a general bypass for all `/api/*` or `/v1/*` traffic.
5. Rule 2 targets `/assets/*`, while the actual frontend output is `/static/*`; for files with listed extensions, rule 4 overrides its TTL anyway.

## Recommended Rules

### 1. Default bypass for dynamic content

Expression:

```text
not starts_with(http.request.uri.path, "/static/")
```

Action: `Bypass cache`.

This covers all HTML routes, `/api/*`, `/v1/*`, OAuth/payment callbacks, and unversioned root assets without relying on query-string-sensitive exceptions.

### 2. Cache content-hashed frontend assets

Expression:

```text
starts_with(http.request.uri.path, "/static/")
```

Settings:

- Cache eligibility: eligible.
- Edge TTL: ignore origin headers, 1 year.
- Status code TTL: 200-299 = 1 year; 300-599 = do not cache.
- Browser TTL: respect origin. The repository currently emits a one-week origin TTL, avoiding irreversible one-year browser caching of a failed request.
- Cache key: keep the default URL-based key; do not include `new-api-frontend`.
- Stale while revalidating: allowed for successful hashed assets.

Delete or disable the current `/assets/*`, `/pricing|/rankings`, and broad file-extension rules. Purge `/pricing`, `/rankings`, and HTML cache entries after deployment.

## Remaining Repository Work

Cloudflare configuration fixes the shared HTML cache, but two code issues remain outside this review's no-code scope:

- The default frontend system setting redirects after changing the global theme without synchronizing the `new-api-frontend` cookie.
- `serveThemeStatic` checks only the cookie-selected filesystem, so a theme-mismatched lazy chunk can still return 404 on a cold edge miss. The server should fall back to the other embedded frontend filesystem, or use theme-specific asset namespaces.

## Official References

- https://developers.cloudflare.com/cache/how-to/cache-rules/order/
- https://developers.cloudflare.com/ruleset-engine/rules-language/fields/reference/http.request.uri/
- https://developers.cloudflare.com/ruleset-engine/rules-language/fields/reference/http.request.uri.path/
- https://developers.cloudflare.com/ruleset-engine/rules-language/operators/
- https://developers.cloudflare.com/cache/how-to/cache-keys/
- https://developers.cloudflare.com/cache/concepts/vary/
- https://developers.cloudflare.com/cache/how-to/edge-browser-cache-ttl/
- https://developers.cloudflare.com/cache/how-to/cache-rules/settings/
- https://developers.cloudflare.com/cache/concepts/default-cache-behavior/
