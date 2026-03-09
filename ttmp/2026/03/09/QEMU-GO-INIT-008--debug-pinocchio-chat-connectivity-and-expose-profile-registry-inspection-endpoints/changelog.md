# Changelog

## 2026-03-09

- Initial workspace created
- Added `internal/aichat` runtime-debug helpers that dump the resolved Pinocchio profile runtime, raw registry/config file contents, effective `StepSettings`, and provider wiring including API keys for debugging.
- Added `/api/debug/aichat/runtime` and `/api/debug/aichat/https-probe` to the guest status server so the host can inspect the live chat configuration and run a traced outbound HTTPS probe against the resolved provider.
- Validated the new endpoints from a live QEMU guest. Findings:
  - `/var/lib/go-init/shared/pinocchio/config.yaml` was missing in the guest, so the resolved `openai-api-key` was empty.
  - The HTTPS probe reached `https://api.openai.com/v1/models` but failed TLS verification with `x509: certificate signed by unknown authority`, indicating the guest currently lacks a CA trust store usable by Go's TLS stack.
- Added a continuation plan for the next debugging slice:
  - source `config.yaml` from the right host path,
  - ship CA roots into the guest,
  - cut down Bobatea trace log noise,
  - render backend transport failures visibly in the chat UI.
- Implemented the continuation slice:
  - split host config and profile sourcing so `config.yaml` can come from `~/.pinocchio/config.yaml` while `profiles.yaml` stays under `~/.config/pinocchio/profiles.yaml`,
  - baked `/etc/ssl/certs/ca-certificates.crt` into the initramfs and exported `SSL_CERT_FILE` in the guest when present,
  - set the qemu-go-init guest and host-native BBS processes to `zerolog.WarnLevel` by default unless `GO_INIT_ZEROLOG_LEVEL` overrides it,
  - changed Pinocchio's engine backend to return `boba_chat.ErrorMsg(err)` on inference failure instead of silently returning only `BackendFinishedMsg`,
  - tightened Bobatea's error dismissal flow so dismissing an error focuses input again.
- Revalidated from the running guest:
  - `/api/debug/aichat/runtime` now shows `/var/lib/go-init/shared/pinocchio/config.yaml` present and the resolved `openai-api-key` populated.
  - `/api/debug/aichat/https-probe` now returns `200 OK` against `https://api.openai.com/v1/models`.
