# Changelog

## 2026-03-08

- Initial workspace created
- Scoped the ticket around explicit QEMU entropy support, guest entropy diagnostics, and operator-visible runtime status
- Added `virtio-rng` support to the repo's QEMU run paths and validated that the smoke boot still succeeds
- Added a guest entropy probe package plus `/api/status` and webpage exposure for entropy state
- Validated that the current guest sees `/dev/hwrng` and kernel entropy metrics, but does not activate a virtio RNG backend because `CONFIG_HW_RANDOM_VIRTIO=m` in the host kernel used for the guest boot
