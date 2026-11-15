---
# dotpostcard-4dly
title: Flip-lift ignores the card's resting lift
status: completed
type: bug
created_at: 2026-11-15T11:19:15Z
updated_at: 2026-11-15T11:19:15Z
---

A flipping card rises the full half-width it sweeps below its axis, as if it started on the page. It already rests `--lift` above it, so it rises `--lift` too far, and `--flip-lift` no longer means a multiple of the rise needed to clear the page.

- [x] Rise by `--flip-lift` × max(0, reach − lift): `1` just clears the page mid-flip
