# Changelog

## 2026-03-09

- Initial workspace created
- Added `internal/aichat` runtime-debug helpers that dump the resolved Pinocchio profile runtime, raw registry/config file contents, effective `StepSettings`, and provider wiring including API keys for debugging.
- Added `/api/debug/aichat/runtime` and `/api/debug/aichat/https-probe` to the guest status server so the host can inspect the live chat configuration and run a traced outbound HTTPS probe against the resolved provider.
- Validated the new endpoints from a live QEMU guest. Findings:
  - `/var/lib/go-init/shared/pinocchio/config.yaml` was missing in the guest, so the resolved `openai-api-key` was empty.
  - The HTTPS probe reached `https://api.openai.com/v1/models` but failed TLS verification with `x509: certificate signed by unknown authority`, indicating the guest currently lacks a CA trust store usable by Go's TLS stack.
