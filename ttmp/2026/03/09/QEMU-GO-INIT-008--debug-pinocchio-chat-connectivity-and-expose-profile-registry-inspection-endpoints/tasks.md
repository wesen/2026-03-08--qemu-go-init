# Tasks

## DONE

- [x] Add Pinocchio runtime introspection helpers that expose resolved config, registries, and effective step settings for the BBS chat mode
- [x] Expose HTTP debug endpoints for raw profile registry/config dumps and an HTTPS provider probe against the resolved OpenAI endpoint
- [x] Validate the new debug endpoints locally, then record the investigation, findings, and review guidance in the ticket diary and changelog

## DONE

- [x] Copy the host Pinocchio `config.yaml` from the correct location into shared guest state, without breaking the existing `profiles.yaml` source path
- [x] Add a CA bundle to the initramfs and verify `/api/debug/aichat/https-probe` succeeds against the resolved OpenAI endpoint
- [x] Reduce Bobatea/Pinocchio trace logging in the SSH chat path so the TUI stays readable during live debugging
- [x] Surface provider transport failures as an explicit chat error state or visible timeline entry instead of only returning `BackendFinishedMsg`
