GO ?= go
BUILD_DIR ?= build
QEMU_BIN ?= qemu-system-x86_64
comma := ,
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
PINOCCHIO_HOST_CONFIG_DIR ?= $(HOME)/.config/pinocchio
PINOCCHIO_HOST_CONFIG_FILE ?= $(if $(wildcard $(HOME)/.pinocchio/config.yaml),$(HOME)/.pinocchio/config.yaml,$(PINOCCHIO_HOST_CONFIG_DIR)/config.yaml)
PINOCCHIO_HOST_PROFILES_FILE ?= $(PINOCCHIO_HOST_CONFIG_DIR)/profiles.yaml
QEMU_SHARED_PINOCCHIO_DIR ?= $(QEMU_SHARED_STATE_HOST_PATH)/pinocchio
MKFS_EXT4 ?= mkfs.ext4
QEMU_ENABLE_VIRTIO_RNG ?= 1
QEMU_RNG_OBJECT ?= rng-random,id=rng0,filename=/dev/urandom
QEMU_RNG_DEVICE ?= virtio-rng-pci,rng=rng0
INITRAMFS_ENABLE_VIRTIO_RNG_MODULE ?= 1
INITRAMFS_VIRTIO_RNG_MODULE_SRC ?= /lib/modules/$(shell uname -r)/kernel/drivers/char/hw_random/virtio-rng.ko.zst
INITRAMFS_ENABLE_9P_MODULES ?= 1
INITRAMFS_NETFS_MODULE_SRC ?= /lib/modules/$(shell uname -r)/kernel/fs/netfs/netfs.ko.zst
INITRAMFS_9P_MODULE_SRC ?= /lib/modules/$(shell uname -r)/kernel/fs/9p/9p.ko.zst
INITRAMFS_9PNET_MODULE_SRC ?= /lib/modules/$(shell uname -r)/kernel/net/9p/9pnet.ko.zst
INITRAMFS_9PNET_VIRTIO_MODULE_SRC ?= /lib/modules/$(shell uname -r)/kernel/net/9p/9pnet_virtio.ko.zst
INITRAMFS_ENABLE_CA_CERTS ?= 1
INITRAMFS_CA_CERT_BUNDLE_SRC ?= /etc/ssl/certs/ca-certificates.crt

INIT_BIN := $(BUILD_DIR)/init
INITRAMFS := $(BUILD_DIR)/initramfs.cpio.gz
QEMU_RUN_STORAGE_ARGS := -drive file=$(QEMU_DATA_IMAGE),if=virtio,format=raw
QEMU_RUN_SHARED_STATE_ARGS := -virtfs local$(comma)path=$(QEMU_SHARED_STATE_HOST_PATH)$(comma)mount_tag=$(QEMU_SHARED_STATE_MOUNT_TAG)$(comma)security_model=none$(comma)id=$(QEMU_SHARED_STATE_FSDEV_ID) -device virtio-9p-pci$(comma)fsdev=$(QEMU_SHARED_STATE_FSDEV_ID)$(comma)mount_tag=$(QEMU_SHARED_STATE_MOUNT_TAG)
QEMU_RUN_RNG_ARGS := -object "$(QEMU_RNG_OBJECT)" -device "$(QEMU_RNG_DEVICE)"

.PHONY: build initramfs test run smoke clean data-image shared-state-dir pinocchio-shared-config

build: $(INIT_BIN)

initramfs: $(INITRAMFS)

test:
	$(GO) test ./...

run: $(INITRAMFS) $(if $(filter 1 true yes on,$(QEMU_ENABLE_STORAGE)),$(QEMU_DATA_IMAGE)) $(if $(filter 1 true yes on,$(QEMU_ENABLE_SHARED_STATE)),shared-state-dir pinocchio-shared-config)
	test -n "$(KERNEL_IMAGE)" || (echo "Set KERNEL_IMAGE to a readable kernel image" && false)
	$(QEMU_BIN) \
		-m $(QEMU_MEMORY) \
		-nographic \
		-kernel $(KERNEL_IMAGE) \
		-initrd $(INITRAMFS) \
		-append "$(QEMU_APPEND)" \
		$(if $(filter 1 true yes on,$(QEMU_ENABLE_STORAGE)),$(QEMU_RUN_STORAGE_ARGS)) \
		$(if $(filter 1 true yes on,$(QEMU_ENABLE_SHARED_STATE)),$(QEMU_RUN_SHARED_STATE_ARGS)) \
		$(if $(filter 1 true yes on,$(QEMU_ENABLE_VIRTIO_RNG)),$(QEMU_RUN_RNG_ARGS)) \
		-nic user,model=virtio-net-pci,hostfwd=tcp::$(QEMU_HOST_PORT)-:$(QEMU_GUEST_PORT),hostfwd=tcp::$(QEMU_SSH_HOST_PORT)-:$(QEMU_SSH_GUEST_PORT)

smoke: $(INITRAMFS) $(if $(filter 1 true yes on,$(QEMU_ENABLE_STORAGE)),$(QEMU_DATA_IMAGE)) $(if $(filter 1 true yes on,$(QEMU_ENABLE_SHARED_STATE)),shared-state-dir pinocchio-shared-config)
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
		$(if $(filter 1 true yes on,$(INITRAMFS_ENABLE_CA_CERTS)),-file-map "/etc/ssl/certs/ca-certificates.crt=$(INITRAMFS_CA_CERT_BUNDLE_SRC)") \
		$(if $(filter 1 true yes on,$(INITRAMFS_ENABLE_VIRTIO_RNG_MODULE)),-module-map "/lib/modules/virtio_rng.ko=$(INITRAMFS_VIRTIO_RNG_MODULE_SRC)") \
		$(if $(filter 1 true yes on,$(INITRAMFS_ENABLE_9P_MODULES)),-module-map "/lib/modules/netfs.ko=$(INITRAMFS_NETFS_MODULE_SRC)" -module-map "/lib/modules/9p.ko=$(INITRAMFS_9P_MODULE_SRC)" -module-map "/lib/modules/9pnet.ko=$(INITRAMFS_9PNET_MODULE_SRC)" -module-map "/lib/modules/9pnet_virtio.ko=$(INITRAMFS_9PNET_VIRTIO_MODULE_SRC)")

$(QEMU_DATA_IMAGE):
	mkdir -p $(BUILD_DIR)
	truncate -s $(QEMU_DATA_SIZE) $(QEMU_DATA_IMAGE)
	$(MKFS_EXT4) -F -L GOINITDATA $(QEMU_DATA_IMAGE)

shared-state-dir:
	mkdir -p $(QEMU_SHARED_STATE_HOST_PATH)

pinocchio-shared-config: shared-state-dir
	mkdir -p $(QEMU_SHARED_PINOCCHIO_DIR)
	rm -f "$(QEMU_SHARED_PINOCCHIO_DIR)/config.yaml" "$(QEMU_SHARED_PINOCCHIO_DIR)/profiles.yaml"
	if [ -f "$(PINOCCHIO_HOST_CONFIG_FILE)" ]; then cp "$(PINOCCHIO_HOST_CONFIG_FILE)" "$(QEMU_SHARED_PINOCCHIO_DIR)/config.yaml"; fi
	if [ -f "$(PINOCCHIO_HOST_PROFILES_FILE)" ]; then cp "$(PINOCCHIO_HOST_PROFILES_FILE)" "$(QEMU_SHARED_PINOCCHIO_DIR)/profiles.yaml"; fi
