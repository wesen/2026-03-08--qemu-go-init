# qemu-go-init

`qemu-go-init` builds a statically linked Go `/init` binary, packs it into an initramfs, and boots it under QEMU so the guest serves an embedded webpage on port `8080`.

The current runtime configures guest IPv4 networking in userspace with a Go DHCP client and Linux netlink calls, so it no longer depends on kernel `ip=dhcp` support.

## Quick start

```bash
make test
make run
```

Then open `http://127.0.0.1:8080/` on the host.

If your distro keeps `/boot/vmlinuz-*` root-only, pass a readable kernel image explicitly:

```bash
KERNEL_IMAGE=/path/to/readable/bzImage make run
```

## Deep documentation

- [System Analysis, Design, Implementation, and Postmortem](SYSTEM-ANALYSIS-DESIGN-IMPLEMENTATION-POSTMORTEM.md)

The init binary also accepts:

- `GO_INIT_NETWORK_INTERFACE=eth0` to force a specific NIC
- `GO_INIT_DHCP_TIMEOUT=20s` to adjust DHCP wait time
- `GO_INIT_DHCP_RETRY=5` to adjust retry count

QEMU launch also accepts:

- `QEMU_ENABLE_VIRTIO_RNG=0` to disable the default `virtio-rng` device in repo run paths
- `QEMU_RNG_OBJECT=...` to override the QEMU RNG backend object
- `QEMU_RNG_DEVICE=...` to override the attached RNG device
- `QEMU_REQUIRE_VIRTIO_RNG=0` to let the smoke script skip the default assertion that the guest actually activates `virtio-rng`

Initramfs creation also accepts:

- `INITRAMFS_ENABLE_VIRTIO_RNG_MODULE=0` to skip embedding the `virtio_rng` kernel module
- `INITRAMFS_VIRTIO_RNG_MODULE_SRC=/path/to/virtio-rng.ko.zst` to override the host module that gets unpacked into the initramfs as `/lib/modules/virtio_rng.ko`

## Key artifacts

- `cmd/init`: Linux PID 1 HTTP server
- `cmd/mkinitramfs`: rootless initramfs builder that emits `newc` cpio archives
- `internal/boot`: early boot helpers for mounts and PID 1 signal handling
- `internal/initramfs`: `newc` archive writer and tests
- `internal/webui`: embedded HTML frontend and JSON status endpoint
- `scripts/qemu-smoke.sh`: boots QEMU, curls the guest, and shuts it down
