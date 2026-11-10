---
# dotpostcard-per8
title: USB bindings for Linux
status: todo
type: feature
created_at: 2026-11-10T16:43:41Z
updated_at: 2026-11-10T16:43:41Z
---

`fastfoto-postcards` only drives the scanner on macOS: `internal/fastfoto/usb_other.go` is a stub that returns an error everywhere else. Implement the `Transport` interface in `internal/fastfoto/session.go` for Linux so the tool can scan there, then add linux to its `goos` in `.goreleaser.yaml`.

Keep it pure Go with `CGO_ENABLED=0`: usbfs (`/dev/bus/usb/...`) through ioctls from `golang.org/x/sys/unix` should cover claiming interface 0 and bulk transfers on endpoints 0x01 and 0x82.

Things macOS taught us:
- Go preempts threads with SIGURG, which aborted synchronous IOKit transfers. Expect the Linux equivalent to be `EINTR` from a blocking ioctl.
- A bulk OUT sent immediately after a bulk IN can be ignored by the firmware, hence the write gap in `session.go`. It has reportedly been a real problem on Linux, so test without assuming the Mac's timing carries over.
- `Drain` must read until a timeout.

Needs thought:
- Non-root access, most likely a udev rule, and how people are told about it.
- Needs testing on a real FF-680W.

## Todo
- [ ] Implement `Transport` over usbfs
- [ ] Decide how users get device permissions
- [ ] Scan a real stack on Linux
- [ ] Add linux to the goreleaser build
