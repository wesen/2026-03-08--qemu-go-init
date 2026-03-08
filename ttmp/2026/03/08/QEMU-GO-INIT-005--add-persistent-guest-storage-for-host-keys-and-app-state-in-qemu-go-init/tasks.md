# Tasks

## TODO

- [x] Create the ticket workspace and baseline document set
- [x] Analyze the current initramfs-only boot model and identify the persistence gap
- [x] Write a detailed persistence architecture and implementation plan
- [x] Write an intern-oriented implementation guide with commands, pseudocode, and file-level guidance
- [x] Record the analysis work in the ticket diary
- [x] Add `internal/storage` with config loading, device detection, ext4 mount, and state-directory preparation
- [x] Add host-side data-image creation and QEMU attachment with create-once / reuse-on-reboot behavior
- [x] Mount storage during boot, surface storage status in the API/UI, and route Wish state onto the mounted volume
- [x] Extend QEMU smoke coverage to reboot against the same image and verify the SSH host key persists
- [x] Refresh the ticket diary, changelog, validation, and published bundle after implementation
