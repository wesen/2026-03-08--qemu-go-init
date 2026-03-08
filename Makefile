GO ?= go
BUILD_DIR ?= build
QEMU_BIN ?= qemu-system-x86_64
KERNEL_IMAGE ?= $(shell find /boot -maxdepth 1 -name 'vmlinuz-*' -readable 2>/dev/null | sort | tail -n 1)
QEMU_HOST_PORT ?= 8080
QEMU_GUEST_PORT ?= 8080
QEMU_MEMORY ?= 512
QEMU_APPEND ?= console=ttyS0 rdinit=/init ip=dhcp

INIT_BIN := $(BUILD_DIR)/init
INITRAMFS := $(BUILD_DIR)/initramfs.cpio.gz

.PHONY: build initramfs test run smoke clean

build: $(INIT_BIN)

initramfs: $(INITRAMFS)

test:
	$(GO) test ./...

run: $(INITRAMFS)
	test -n "$(KERNEL_IMAGE)" || (echo "Set KERNEL_IMAGE to a readable kernel image" && false)
	$(QEMU_BIN) \
		-m $(QEMU_MEMORY) \
		-nographic \
		-kernel $(KERNEL_IMAGE) \
		-initrd $(INITRAMFS) \
		-append "$(QEMU_APPEND)" \
		-nic user,model=virtio-net-pci,hostfwd=tcp::$(QEMU_HOST_PORT)-:$(QEMU_GUEST_PORT)

smoke: $(INITRAMFS)
	HOST_PORT=$(QEMU_HOST_PORT) KERNEL_IMAGE=$(KERNEL_IMAGE) ./scripts/qemu-smoke.sh

clean:
	rm -rf $(BUILD_DIR)

$(INIT_BIN): $(shell find cmd internal -type f -name '*.go')
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags='-s -w' -o $(INIT_BIN) ./cmd/init

$(INITRAMFS): $(INIT_BIN)
	$(GO) run ./cmd/mkinitramfs -init-bin $(INIT_BIN) -output $(INITRAMFS)
