# Integration Tests

End-to-end integration tests that validate cross-service flows.

These tests start embedded HTTP servers for each service and verify
the complete request path works correctly.

## Running

```bash
cd tests/integration
go test ./... -v -count=1
```

## Test Flows

1. **Identity Flow**: Register user → Verify NIK via Dukcapil → Get profile
2. **Corporate Flow**: Register entity via AHU → Assign roles → Get entity
3. **Signing Flow**: Request certificate → Upload document → Sign → Download
4. **Portal Flow**: Create app → Generate API key → Validate key
