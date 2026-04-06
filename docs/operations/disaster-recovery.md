# GarudaPass Disaster Recovery Plan

## Recovery Objectives

| Metric | Target | Rationale |
|--------|--------|-----------|
| RPO (Recovery Point Objective) | 1 hour | Maximum acceptable data loss |
| RTO (Recovery Time Objective) | 4 hours | Maximum acceptable downtime |
| MTTR (Mean Time to Repair) | 2 hours | Average recovery time |

## Backup Strategy

### PostgreSQL

| Component | Method | Frequency | Retention |
|-----------|--------|-----------|-----------|
| Full backup | `pg_basebackup` | Daily 02:00 WIB | 30 days |
| WAL archiving | Continuous streaming | Real-time | 7 days |
| Point-in-time | WAL replay | As needed | 7 days |
| Logical backup | `pg_dump` | Weekly Sunday 03:00 | 90 days |

```bash
# Daily full backup
pg_basebackup -h postgresql -U garudapass -D /backups/daily/$(date +%Y%m%d) -Ft -z -P

# Weekly logical backup
pg_dump -h postgresql -U garudapass garudapass | gzip > /backups/weekly/$(date +%Y%m%d).sql.gz

# Restore from backup
pg_restore -h postgresql -U garudapass -d garudapass /backups/daily/20260406/base.tar
```

### Redis

| Component | Method | Frequency | Retention |
|-----------|--------|-----------|-----------|
| RDB snapshot | `BGSAVE` | Every 15 min | 24 hours |
| AOF persistence | `appendonly yes` | Real-time | 48 hours |

Redis data is ephemeral (sessions, OTP, rate limit counters). Full rebuild is acceptable from PostgreSQL.

### Document Storage (GarudaSign)

| Component | Method | Frequency | Retention |
|-----------|--------|-----------|-----------|
| Signed PDFs | Object storage replication | Real-time (S3 cross-region) | 10 years |
| Unsigned uploads | Local disk | Ephemeral (30min TTL) | Not backed up |

### Kafka/Redpanda

| Component | Method | Frequency | Retention |
|-----------|--------|-----------|-----------|
| Audit topics | Topic retention | Continuous | 30 days in Kafka, permanent in PostgreSQL |
| Event replay | Consumer group offset management | As needed | From earliest retained offset |

## Failure Scenarios

### 1. Single Service Failure

**Impact:** One microservice unavailable  
**Detection:** Kubernetes liveness probe fails, Prometheus alert fires  
**Recovery:** Automatic — Kubernetes restarts pod (< 30s)  
**Data loss:** None (stateless services, state in PostgreSQL/Redis)

### 2. Database Failure

**Impact:** All services degraded (read/write fails)  
**Detection:** Health check fails, circuit breakers open  
**Recovery:**
1. Promote standby replica (if HA configured): < 5 min
2. Restore from backup + WAL replay: < 2 hours
3. Run migrations: `make migrate`

### 3. Redis Failure

**Impact:** Sessions invalidated, rate limiting disabled, OTP verification fails  
**Detection:** Redis health check fails, BFF session errors spike  
**Recovery:**
1. Restart Redis pod: < 1 min
2. Sessions: users re-authenticate (acceptable)
3. Rate limits: rebuild from zero (graceful degradation)

### 4. Complete Cluster Failure

**Impact:** Total platform outage  
**Detection:** All health checks fail, external monitoring alerts  
**Recovery:**
1. Provision new cluster (Terraform/IaC): 30 min
2. Restore PostgreSQL from latest backup: 1 hour
3. Deploy all services (Kubernetes manifests): 15 min
4. Verify health endpoints: 15 min
5. DNS failover to new cluster: 5 min
**Total RTO:** ~2 hours

### 5. Data Corruption

**Impact:** Inconsistent data state  
**Detection:** Application errors, audit log discrepancies  
**Recovery:**
1. Stop affected service
2. Point-in-time recovery using WAL: restore to last consistent state
3. Replay audit events from Kafka if needed
4. Restart service

## Runbook

### Pre-incident

- [ ] Verify backups are completing (check `/backups/` timestamps)
- [ ] Test restore procedure quarterly
- [ ] Verify monitoring alerts are firing correctly
- [ ] Update incident response contacts

### During Incident

1. **Assess** — Which services are affected? Check Grafana dashboard
2. **Communicate** — Notify stakeholders via status page
3. **Isolate** — Remove affected components from load balancer
4. **Recover** — Follow relevant failure scenario above
5. **Verify** — Run smoke tests: `k6 run tests/load/smoke.js`
6. **Monitor** — Watch error rates for 30 min post-recovery

### Post-incident

- [ ] Write incident report (5 Whys analysis)
- [ ] Update this runbook with lessons learned
- [ ] Adjust monitoring/alerting thresholds if needed
- [ ] Schedule follow-up review in 1 week
