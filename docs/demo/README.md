# GarudaPass end-to-end demo

## What this covers

A full user journey from citizen identity registration through corporate
entity verification, document signing, and audit trail validation —
exercising every backend service.

## Prerequisites

1. **Postgres** on `localhost:5433` (via the repo's docker-compose override):
   ```
   docker compose up -d postgres redis prometheus grafana
   ```
2. **All 17 migrations** applied:
   ```
   for f in infrastructure/db/migrations/*.sql; do
     docker exec -i gpass-postgres-1 psql -U garudapass -d garudapass < "$f"
   done
   ```
3. **All 11 backend services** booted on ports 4001–4011.

## Running the demo

```bash
./docs/demo/e2e-happy-path.sh
```

Exit code 0 = every critical step passed. Any non-zero exit prints the
failing step and HTTP code.

## What it verifies

| Step | Service | Purpose |
|------|---------|---------|
| 0 | all 11 | Liveness probes respond 200 on `/health` |
| 1 | dukcapil-sim | NIK lookup (Dukcapil population data) |
| 2 | identity | User registration with NIK tokenization |
| 3 | garudainfo | UU PDP consent grant with field-level scopes |
| 4 | ahu-sim | AHU/SABH legal entity lookup |
| 5 | garudacorp | PT/CV/Yayasan registration |
| 6 | garudasign | EJBCA certificate issuance |
| 7 | garudasign | PDF upload with SHA-256 hash + size guards |
| 8 | garudasign | PAdES-B-LTA signing (ETSI EN 319 142) |
| 9 | garudaaudit | PP 71/2019 audit trail query |
| 10 | garudaportal | Developer app + API key lifecycle |

## Compliance mapping

- **UU PDP No. 27/2022** — field-level consent, tokenized NIK, encrypted
  PII at rest (steps 2, 3)
- **PP 71/2019** — immutable 5-year audit retention (step 9 validates
  that every mutating action produced an audit record)
- **PP 13/2018** — beneficial ownership capture on entity register
  (step 5)
- **ETSI EN 319 142** — PAdES-B-LTA long-term signatures (step 8)

## Troubleshooting

- `identity register → 400/500`: Keycloak must be running and the admin
  user must exist. The demo tolerates failure here because keycloak
  is optional for the rest of the flow.
- `audit events: 0`: garudaaudit's Append endpoint isn't being called.
  Check that each service has `GARUDAAUDIT_URL` set in its env.
- `garudacorp register → 409`: the AHU SK number already exists from a
  previous demo run — harmless, the script accepts this.
