# Changelog

## 2026-03-08

- Initial workspace created
- Added persistence analysis, implementation plan, and intern guide for durable guest storage
- `docmgr doctor --ticket QEMU-GO-INIT-005 --stale-after 30` passed and the ticket bundle was uploaded to reMarkable at `/ai/2026/03/08/QEMU-GO-INIT-005`
- Added `internal/storage` for persistent block-device waiting, ext4 mounting, and durable state-directory preparation (commit `2ffc837`)
- Added create-once host-side `data.img` generation plus QEMU disk attachment for run/smoke workflows (commit `4ec71f9`)
- Mounted persistent storage during boot, exposed it in the status UI, and routed Wish host keys onto the mounted volume (commit `90a3439`)
- Fixed zero-byte host-key persistence by validating and atomically writing SSH host keys before starting Wish, then proved key stability across reboot
- Re-ran `docmgr doctor`, refreshed the bundles for tickets `004` and `005`, and marked the persistence ticket complete
