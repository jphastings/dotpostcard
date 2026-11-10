---
# dotpostcard-xg0v
title: USB bindings for Windows
status: todo
type: feature
created_at: 2026-11-10T16:43:41Z
updated_at: 2026-11-10T16:43:41Z
---

`fastfoto-postcards` only drives the scanner on macOS: `internal/fastfoto/usb_other.go` is a stub that returns an error everywhere else. Implement the `Transport` interface in `internal/fastfoto/session.go` for Windows so the tool can scan there, then add windows to its `goos` in `.goreleaser.yaml`.

Keep it pure Go with `CGO_ENABLED=0`: WinUSB via `winusb.dll` and SetupAPI through `golang.org/x/sys/windows` should cover interface 0 and bulk transfers on endpoints 0x01 and 0x82.

The main open question is the driver. WinUSB can only open a device bound to the WinUSB driver, and Epson's own driver normally claims the FF-680W. Find out whether it can coexist, or whether people would have to replace the driver, and so lose Epson's software, before committing to this approach. If replacing is the only option, it may not be worth shipping.

Things macOS taught us:
- A session that ends badly leaves replies queued and a session open, so `resync` must still run on connect.
- `Drain` must read until a timeout.

Needs testing on a real FF-680W.

## Todo
- [ ] Find out how WinUSB can reach the scanner alongside Epson's driver
- [ ] Implement `Transport` over WinUSB
- [ ] Scan a real stack on Windows
- [ ] Add windows to the goreleaser build
