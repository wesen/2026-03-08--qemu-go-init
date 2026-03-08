The cleanest PoC is:

1. build one **statically linked Go binary** that becomes `/init`
2. put it in a tiny **initramfs**
3. boot a Linux kernel in QEMU with that initramfs
4. use QEMU **user networking + port forwarding** so the host can hit the guest HTTP server

That maps directly onto how Linux initramfs boot works: the kernel unpacks a cpio archive into `rootfs`, then executes `/init` as PID 1 if it exists. The kernel docs even suggest “a statically linked hello world as init” under QEMU as a first step. ([Linux Kernel Documentation][1])

## Minimal working shape

You are not really making “one file total”; you are making **one userspace binary**. The bootable system is either:

* `kernel + external initramfs`, or
* `kernel with built-in initramfs` via `CONFIG_INITRAMFS_SOURCE`. ([Linux Kernel Documentation][1])

For the first PoC, use the external initramfs path because it is fastest.

---

## 1) Write `/init` in Go

Create `init.go`:

```go
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"syscall"
)

func mount(name, target, fstype string, flags uintptr, data string) {
	_ = os.MkdirAll(target, 0o755)
	if err := syscall.Mount(name, target, fstype, flags, data); err != nil {
		log.Printf("mount %s on %s failed: %v", name, target, err)
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("go init starting")

	// Nice to have for debugging; not strictly required for the hello-world server.
	mount("proc", "/proc", "proc", 0, "")
	mount("sysfs", "/sys", "sysfs", 0, "")
	mount("devtmpfs", "/dev", "devtmpfs", 0, "mode=0755")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello world")
	})

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

Go’s `http.ListenAndServe` listens on the TCP address you give it and serves requests with your handler. ([Go Packages][2])

Build it as a static Linux binary:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags='-s -w' -o init ./init.go
```

---

## 2) Build a tiny initramfs

Make a tiny rootfs tree:

```bash
mkdir -p rootfs/{dev,proc,sys}
cp init rootfs/init
chmod +x rootfs/init
sudo mknod -m 600 rootfs/dev/console c 5 1
sudo mknod -m 666 rootfs/dev/null c 1 3
```

Pack it as initramfs:

```bash
(
  cd rootfs
  find . -print0 | cpio --null -ov --format=newc | gzip -9
) > initramfs.cpio.gz
```

Linux initramfs uses the `newc` cpio format, and the kernel supports compressed cpio archives for initramfs. ([Linux Kernel Documentation][3])

---

## 3) Boot it in QEMU

### Fastest try: use your host kernel

```bash
qemu-system-x86_64 \
  -m 512 \
  -nographic \
  -kernel /boot/vmlinuz-$(uname -r) \
  -initrd initramfs.cpio.gz \
  -append 'console=ttyS0 rdinit=/init ip=dhcp' \
  -nic user,model=virtio-net-pci,hostfwd=tcp::8080-:8080
```

What those bits are doing:

* `console=ttyS0` sends kernel/userspace console to the QEMU terminal
* `rdinit=/init` explicitly tells the initramfs boot to run `/init`
* `ip=dhcp` asks the kernel IP autoconfig code to configure networking via DHCP, assuming the kernel has IP_PNP/DHCP enabled
* `hostfwd=tcp::8080-:8080` forwards host port 8080 to guest port 8080 under QEMU user networking ([Linux Kernel Documentation][1])

Then from the host:

```bash
curl http://127.0.0.1:8080/
```

Expected response:

```text
hello world
```

---

## 4) If the host kernel path fails, build a tiny custom kernel

The host kernel often boots, but networking can fail if the NIC driver is modular rather than built-in. For a reliable PoC, build your own kernel with the few things you need built in.

From a Linux kernel tree:

```bash
make x86_64_defconfig

scripts/config \
  --enable BLK_DEV_INITRD \
  --enable DEVTMPFS \
  --enable DEVTMPFS_MOUNT \
  --enable SERIAL_8250 \
  --enable SERIAL_8250_CONSOLE \
  --enable NET \
  --enable INET \
  --enable IP_PNP \
  --enable IP_PNP_DHCP \
  --enable VIRTIO \
  --enable VIRTIO_PCI \
  --enable VIRTIO_NET

make -j"$(nproc)" bzImage
```

Then boot that kernel instead:

```bash
qemu-system-x86_64 \
  -m 512 \
  -nographic \
  -kernel arch/x86/boot/bzImage \
  -initrd initramfs.cpio.gz \
  -append 'console=ttyS0 rdinit=/init ip=dhcp' \
  -nic user,model=virtio-net-pci,hostfwd=tcp::8080-:8080
```

---

## 5) Turn it into a “single artifact” later

Once the external-initramfs version works, embed the initramfs into the kernel.

The kernel supports `CONFIG_INITRAMFS_SOURCE`, which can point at a cpio archive, a directory, or a spec file. ([Linux Kernel Documentation][4])

Example:

```bash
scripts/config --set-str INITRAMFS_SOURCE "$PWD/rootfs"
make -j"$(nproc)" bzImage
```

Now you can boot without `-initrd`:

```bash
qemu-system-x86_64 \
  -m 512 \
  -nographic \
  -kernel arch/x86/boot/bzImage \
  -append 'console=ttyS0 rdinit=/init ip=dhcp' \
  -nic user,model=virtio-net-pci,hostfwd=tcp::8080-:8080
```

That is the closest to your “single binary Go init + Linux kernel system” idea: one kernel image containing your userspace payload.

---

## 6) The three most common failure modes

**Boots but you see no output**
Usually missing `console=ttyS0`, missing `-nographic`, or missing `/dev/console` in the initramfs.

**HTTP server starts but host curl hangs**
Usually the guest never got networking. Make sure you used `ip=dhcp` and the kernel has `CONFIG_IP_PNP=y` and `CONFIG_IP_PNP_DHCP=y`. The kernel parameter docs point `ip=` to the IP autoconfiguration docs, which describe `ip=dhcp` as the common form. ([Kernel.org][5])

**QEMU says init exited / kernel panic**
PID 1 must not return. In this design, your Go HTTP server blocks forever, so that is fine. The initramfs `/init` process is not expected to return to the kernel. ([Linux Kernel Documentation][1])

---

## 7) Smallest practical workflow

For a weekend PoC, I would do it in exactly this order:

1. build `init`
2. pack `initramfs.cpio.gz`
3. try host kernel in QEMU
4. if networking fails, build tiny custom kernel
5. once stable, embed the initramfs into the kernel

That gets you from zero to “Go binary is PID 1 and serves hello world in QEMU” with the fewest moving parts.

I can also give you a ready-to-copy `Makefile` that builds the Go init, packs the initramfs, builds the kernel, and runs QEMU with one command.

[1]: https://docs.kernel.org/filesystems/ramfs-rootfs-initramfs.html "Ramfs, rootfs and initramfs — The Linux Kernel  documentation"
[2]: https://pkg.go.dev/net/http "http package - net/http - Go Packages"
[3]: https://docs.kernel.org/driver-api/early-userspace/buffer-format.html?utm_source=chatgpt.com "initramfs buffer format"
[4]: https://docs.kernel.org/driver-api/early-userspace/early_userspace_support.html?utm_source=chatgpt.com "Early userspace support"
[5]: https://www.kernel.org/doc/html/v6.8/admin-guide/kernel-parameters.html "The kernel's command-line parameters — The Linux Kernel  documentation"

