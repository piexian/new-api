# Host-scoped Cloudflare Cache Rules

## Scope

- Cache Rules are zone-scoped. Different Cloudflare zones are unaffected.
- Without an `http.host` condition, a rule can match every hostname in the same zone.
- For the New API deployment, scope both rules to `api.pie-xian.com`.

## Recommended Expressions

Dynamic and HTML bypass:

```text
(http.host eq "api.pie-xian.com")
and not starts_with(http.request.uri.path, "/static/")
```

Hashed frontend static cache:

```text
(http.host eq "api.pie-xian.com")
and starts_with(http.request.uri.path, "/static/")
```

These expressions are mutually exclusive and do not affect other hostnames in the zone.

If several hostnames run the same New API instance, replace the equality check with:

```text
http.host in {"api.pie-xian.com" "api2.pie-xian.com"}
```

If another application shares the exact same hostname under a path prefix, exclude that prefix from the bypass rule and give that application separate host-and-path-scoped cache rules.
