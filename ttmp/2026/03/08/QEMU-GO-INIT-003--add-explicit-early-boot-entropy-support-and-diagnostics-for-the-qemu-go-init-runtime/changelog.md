# Changelog

## 2026-03-08

- Initial workspace created
- Scoped the ticket around explicit QEMU entropy support, guest entropy diagnostics, and operator-visible runtime status
- Added `virtio-rng` support to the repo's QEMU run paths and validated that the smoke boot still succeeds
- Added a guest entropy probe package plus `/api/status` and webpage exposure for entropy state
- Validated that the current guest sees `/dev/hwrng` and kernel entropy metrics, but does not activate a virtio RNG backend because `CONFIG_HW_RANDOM_VIRTIO=m` in the host kernel used for the guest boot
- Implemented initramfs embedding plus early-boot loading of `virtio_rng` so the current distro kernel can activate a real guest entropy backend without recompiling the kernel
- Fixed the first module-loading attempt after the guest reported `exec format error` and `Invalid ELF header magic`; the initramfs builder now decompresses `.ko.zst` inputs into an ELF `.ko` before packaging
- Validated the final runtime state with `make test`, `make initramfs`, and `make smoke KERNEL_IMAGE=/tmp/qemu-vmlinuz QEMU_HOST_PORT=18080`, which now returns `rngCurrent: "virtio_rng.0"` and `virtioRngVisible: true`
- `docmgr doctor --ticket QEMU-GO-INIT-003 --stale-after 30` passed cleanly, and the ticket bundle was uploaded to reMarkable at `/ai/2026/03/08/QEMU-GO-INIT-003`
- Fixed a PDF-rendering issue during upload by removing a literal control character from the diary's copied kernel log snippet
- Added a new full-system design doc that explains the current repository architecture, boot sequence, API contract, operator workflow, and extension points for new engineers
- Refreshed the reMarkable bundle to include the new full-system guide and verified the remote listing again
