# GarudaPass CORS & Security Headers Policy

## Production Security Headers

All services behind Kong API Gateway enforce:

```
Strict-Transport-Security: max-age=63072000; includeSubDomains; preload
Content-Security-Policy: default-src 'none'; frame-ancestors 'none'
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 0
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: camera=(), microphone=(), geolocation=(), payment=()
Cache-Control: no-store, no-cache, must-revalidate
```

## CORS Policy

- **Allowed Origins:** Only explicitly configured origins (no wildcard)
- **Allowed Methods:** GET, POST, PUT, PATCH, DELETE, OPTIONS
- **Allowed Headers:** Content-Type, Authorization, X-API-Key, X-Request-Id, X-CSRF-Token
- **Max Age:** 86400 (24 hours)
- **Credentials:** true (for cookie-based auth via BFF)

## API Key Headers

External API consumers authenticate via:
- `X-API-Key: gp_live_...` header, OR
- `Authorization: Bearer gp_live_...` header

Internal service-to-service calls use:
- `X-User-ID` header (set by BFF after session validation)
- `X-Request-Id` header (propagated for distributed tracing)

## Rate Limiting

| Tier | Daily Limit | Per-Minute Burst |
|------|------------|-----------------|
| Free | 100 | 10 |
| Starter | 10,000 | 100 |
| Growth | 100,000 | 1,000 |
| Enterprise | Custom | Custom |
