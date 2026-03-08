GO ?= go
BUILD_DIR ?= build
QEMU_BIN ?= qemu-system-x86_64
KERNEL_IMAGE ?= $(shell find /boot -maxdepth 1 -name 'vmlinuz-*' -readable 2>/dev/null | sort | tail -n 1)
QEMU_HOST_PORT ?= 8080
QEMU_GUEST_PORT ?= 8080
QEMU_SSH_HOST_PORT ?= 10022
QEMU_SSH_GUEST_PORT ?= 2222
QEMU_MEMORY ?= 512
QEMU_APPEND ?= console=ttyS0 rdinit=/init
QEMU_ENABLE_STORAGE ?= 1
QEMU_DATA_IMAGE ?= $(BUILD_DIR)/data.img
QEMU_DATA_SIZE ?= 64M
QEMU_ENABLE_SHARED_STATE ?= 1
QEMU_SHARED_STATE_HOST_PATH ?= $(BUILD_DIR)/shared-state
QEMU_SHARED_STATE_MOUNT_TAG ?= hostshare
QEMU_SHARED_STATE_FSDEV_ID ?= hostsharefs
MKFS_EXT4 ?= mkfs.ext4
QEMU_ENABLE_VIRTIO_RNG ?= 1
QEMU_RNG_OBJECT ?= rng-random,id=rng0,filename=/dev/urandom
QEMU_RNG_DEVICE ?= virtio-rng-pci,rng=rng0
INITRAMFS_ENABLE_VIRTIO_RNG_MODULE ?= 1
INITRAMFS_VIRTIO_RNG_MODULE_SRC ?= /lib/modules/$(shell uname -r)/kernel/drivers/char/hw_random/virtio-rng.ko.zst
INITRAMFS_ENABLE_9P_MODULES ?= 1
INITRAMFS_9P_MODULE_SRC ?= /lib/modules/$(shell uname -r)/kernel/fs/9p/9p.ko.zst
INITRAMFS_9PNET_MODULE_SRC ?= /lib/modules/$(shell uname -r)/kernel/net/9p/9pnet.ko.zst
INITRAMFS_9PNET_VIRTIO_MODULE_SRC ?= /lib/modules/$(shell uname -r)/kernel/net/9p/9pnet_virtio.ko.zst

INIT_BIN := $(BUILD_DIR)/init
INITRAMFS := $(BUILD_DIR)/initramfs.cpio.gz

.PHONY: build initramfs test run smoke clean data-image shared-state-dir

build: $(INIT_BIN)

initramfs: $(INITRAMFS)

test:
	$(GO) test ./...

run: $(INITRAMFS) $(if $(filter 1 true yes on,$(QEMU_ENABLE_STORAGE)),$(QEMU_DATA_IMAGE)) $(if $(filter 1 true yes on,$(QEMU_ENABLE_SHARED_STATE)),shared-state-dir)
	test -n "$(KERNEL_IMAGE)" || (echo "Set KERNEL_IMAGE to a readable kernel image" && false)
	$(QEMU_BIN) \
		-m $(QEMU_MEMORY) \
		-nographic \
		-kernel $(KERNEL_IMAGE) \
		-initrd $(INITRAMFS) \
		-append "$(QEMU_APPEND)" \
		$(if $(filter 1 true yes on,$(QEMU_ENABLE_STORAGE)),-drive file=$(QEMU_DATA_IMAGE),if=virtio,format=raw) \
		$(if $(filter 1 true yes on,$(QEMU_ENABLE_SHARED_STATE)),-virtfs "local,path=$(QEMU_SHARED_STATE_HOST_PATH),mount_tag=$(QEMU_SHARED_STATE_MOUNT_TAG),security_model=none,id=$(QEMU_SHARED_STATE_FSDEV_ID)" -device "virtio-9p-pci,fsdev=$(QEMU_SHARED_STATE_FSDEV_ID),mount_tag=$(QEMU_SHARED_STATE_MOUNT_TAG)") \
		$(if $(filter 1 true yes on,$(QEMU_ENABLE_VIRTIO_RNG)),-object "$(QEMU_RNG_OBJECT)" -device "$(QEMU_RNG_DEVICE)") \
		-nic user,model=virtio-net-pci,hostfwd=tcp::$(QEMU_HOST_PORT)-:$(QEMU_GUEST_PORT),hostfwd=tcp::$(QEMU_SSH_HOST_PORT)-:$(QEMU_SSH_GUEST_PORT)

smoke: $(INITRAMFS) $(if $(filter 1 true yes on,$(QEMU_ENABLE_STORAGE)),$(QEMU_DATA_IMAGE)) $(if $(filter 1 true yes on,$(QEMU_ENABLE_SHARED_STATE)),shared-state-dir)
	HOST_PORT=$(QEMU_HOST_PORT) \
	KERNEL_IMAGE=$(KERNEL_IMAGE) \
	SSH_HOST_PORT=$(QEMU_SSH_HOST_PORT) \
	SSH_GUEST_PORT=$(QEMU_SSH_GUEST_PORT) \
	QEMU_ENABLE_STORAGE=$(QEMU_ENABLE_STORAGE) \
	QEMU_DATA_IMAGE=$(QEMU_DATA_IMAGE) \
	QEMU_ENABLE_SHARED_STATE=$(QEMU_ENABLE_SHARED_STATE) \
	QEMU_SHARED_STATE_HOST_PATH=$(QEMU_SHARED_STATE_HOST_PATH) \
	QEMU_SHARED_STATE_MOUNT_TAG=$(QEMU_SHARED_STATE_MOUNT_TAG) \
	QEMU_SHARED_STATE_FSDEV_ID=$(QEMU_SHARED_STATE_FSDEV_ID) \
	QEMU_ENABLE_VIRTIO_RNG=$(QEMU_ENABLE_VIRTIO_RNG) \
	QEMU_RNG_OBJECT='$(QEMU_RNG_OBJECT)' \
	QEMU_RNG_DEVICE='$(QEMU_RNG_DEVICE)' \
	./scripts/qemu-smoke.sh

data-image: $(QEMU_DATA_IMAGE)

clean:
	rm -rf $(BUILD_DIR)

$(INIT_BIN): $(shell find cmd internal -type f -name '*.go')
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags='-s -w' -o $(INIT_BIN) ./cmd/init

$(INITRAMFS): $(INIT_BIN)
	$(GO) run ./cmd/mkinitramfs -init-bin $(INIT_BIN) -output $(INITRAMFS) \
		$(if $(filter 1 true yes on,$(INITRAMFS_ENABLE_VIRTIO_RNG_MODULE)),-module-map "/lib/modules/virtio_rng.ko=$(INITRAMFS_VIRTIO_RNG_MODULE_SRC)") \
		$(if $(filter 1 true yes on,$(INITRAMFS_ENABLE_9P_MODULES)),-module-map "/lib/modules/9p.ko=$(INITRAMFS_9P_MODULE_SRC)" -module-map "/lib/modules/9pnet.ko=$(INITRAMFS_9PNET_MODULE_SRC)" -module-map "/lib/modules/9pnet_virtio.ko=$(INITRAMFS_9PNET_VIRTIO_MODULE_SRC)")

$(QEMU_DATA_IMAGE):
	mkdir -p $(BUILD_DIR)
	truncate -s $(QEMU_DATA_SIZE) $(QEMU_DATA_IMAGE)
	$(MKFS_EXT4) -F -L GOINITDATA $(QEMU_DATA_IMAGE)

shared-state-dir:
	mkdir -p $(QEMU_SHARED_STATE_HOST_PATH)
