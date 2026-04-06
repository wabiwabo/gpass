# GarudaPass Operations Runbook

## Service Management

### Start All Services (Development)
```bash
make up                    # Docker Compose
```

### Stop All Services
```bash
docker compose down        # Stop containers
docker compose down -v     # Stop + remove volumes (DESTROYS DATA)
```

### View Logs
```bash
docker compose logs -f bff          # Single service
docker compose logs -f --tail=100   # All services, last 100 lines
```

### Health Checks
```bash
# Quick check all services
for port in 4000 4001 4002 4003 4004 4005 4006 4007 4008 4009; do
  echo -n "Port $port: "
  curl -s http://localhost:$port/health | jq -r '.status'
done

# k6 smoke test
k6 run tests/load/smoke.js
```

## Database Operations

### Run Migrations
```bash
# Apply all pending migrations
for f in infrastructure/db/migrations/*.sql; do
  echo "Applying: $f"
  psql $DATABASE_URL -f "$f"
done
```

### Check Migration Status
```bash
psql $DATABASE_URL -c "SELECT * FROM schema_migrations ORDER BY version;"
```

### Database Backup
```bash
pg_dump $DATABASE_URL | gzip > backup_$(date +%Y%m%d_%H%M).sql.gz
```

### Database Restore
```bash
gunzip -c backup_20260406_0200.sql.gz | psql $DATABASE_URL
```

## Monitoring

### Access Dashboards
- **Grafana:** http://localhost:3002 (admin/garudapass)
- **Prometheus:** http://localhost:9090
- **Jaeger:** http://localhost:16686
- **Kong Admin:** http://localhost:8001
- **Keycloak Admin:** http://localhost:8080 (admin/admin)

### Check Error Rates
```bash
# Prometheus query: error rate by service
curl -s 'http://localhost:9090/api/v1/query?query=rate(http_requests_total{status_code=~"5.."}[5m])' | jq
```

### Check Circuit Breakers
```bash
# Look for open circuit breakers in logs
docker compose logs --since 5m | grep "circuit breaker"
```

## Incident Response

### High Error Rate (>1%)
1. Check Grafana: which service has errors?
2. Check logs: `docker compose logs -f <service> --tail=50`
3. Check dependencies: is PostgreSQL/Redis/Kafka healthy?
4. Check circuit breakers: are external APIs down?
5. If service crash loop: `docker compose restart <service>`

### High Latency (P99 > 2s)
1. Check database connections: `SELECT count(*) FROM pg_stat_activity;`
2. Check Redis memory: `redis-cli -a garudapass-redis-dev INFO memory`
3. Check for long-running queries: `SELECT * FROM pg_stat_activity WHERE state = 'active';`
4. Scale horizontally if needed (K8s HPA will auto-scale)

### Session Issues
1. Check Redis is running: `redis-cli -a garudapass-redis-dev ping`
2. Check session encryption key hasn't changed
3. Users may need to re-authenticate after Redis restart

### Signing Failures
1. Check signing-sim health: `curl http://localhost:4008/health`
2. Check document storage: `ls -la /data/signing/`
3. Check certificate status in logs
4. For production: verify EJBCA + DSS connectivity

## Deployment

### Deploy New Version
```bash
# Build all images
make docker-build

# Tag and push
docker tag garudapass/bff:latest registry.garudapass.id/bff:v1.2.3
docker push registry.garudapass.id/bff:v1.2.3

# Kubernetes rolling update
kubectl set image deployment/bff bff=registry.garudapass.id/bff:v1.2.3 -n garudapass

# Verify
kubectl rollout status deployment/bff -n garudapass
```

### Rollback
```bash
kubectl rollout undo deployment/bff -n garudapass
kubectl rollout status deployment/bff -n garudapass
```

## Security Operations

### Rotate Session Secret
1. Generate new secret: `openssl rand -base64 48`
2. Update BFF_SESSION_SECRET in environment
3. Restart BFF: all existing sessions will be invalidated
4. Users will need to re-authenticate

### Rotate NIK Encryption Key
1. **WARNING:** This requires re-tokenizing all stored NIK tokens
2. Generate new key: `openssl rand -hex 32`
3. Run migration script to re-tokenize with new key
4. Update SERVER_NIK_KEY and restart services

### Revoke All API Keys for an App
```bash
# Via GarudaPortal API
curl -X DELETE http://localhost:4009/api/v1/portal/apps/{app_id}/keys \
  -H "X-User-ID: admin-user-id"
```
