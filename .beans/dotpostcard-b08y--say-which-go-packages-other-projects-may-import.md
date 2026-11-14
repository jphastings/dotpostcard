---
# dotpostcard-b08y
title: Say which Go packages other projects may import
status: completed
type: task
priority: low
created_at: 2026-11-14T14:28:35Z
updated_at: 2026-11-14T14:52:33Z
---

Unsent will import this module from its indexer and its image deriver:

- `types`;
- `formats`, for `formats.DecodeOptions`, which decoding needs;
- `formats/atproto`, for the record types, `ParseATURI`, `FetchCollection`, `ListRecords` and the anonymous client, without the image toolchain;
- `formats/web`, to decode a web-format image into upright front and back sides.

Nothing says which of these are meant to stay stable. A v0 module may change freely, but a short note would stop an internal tidy-up from breaking a consumer unannounced.

## Todo
- [x] A short "Using dotpostcard as a library" section in the README or docs, naming the supported packages and entry points
- [x] An example test that decodes a web-format image into its two upright sides
- [x] Changes to those entry points are marked as breaking in the changelog
