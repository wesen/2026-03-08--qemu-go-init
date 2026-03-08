# qemu-go-init

`qemu-go-init` builds a statically linked Go `/init` binary, packs it into an initramfs, and boots it under QEMU so the guest serves an embedded webpage on port `8080`.

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

## Key artifacts

- `cmd/init`: Linux PID 1 HTTP server
- `cmd/mkinitramfs`: rootless initramfs builder that emits `newc` cpio archives
- `internal/boot`: early boot helpers for mounts and PID 1 signal handling
- `internal/initramfs`: `newc` archive writer and tests
- `internal/webui`: embedded HTML frontend and JSON status endpoint
- `scripts/qemu-smoke.sh`: boots QEMU, curls the guest, and shuts it down
