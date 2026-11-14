---
# dotpostcard-najh
title: Undated postcards get a new record key when re-uploaded after a download
status: completed
type: bug
created_at: 2026-11-14T09:10:15Z
updated_at: 2026-11-14T14:28:35Z
---

A postcard record's key is a TID made from the day the card was sent plus the image's hash (`atproto.RecordKey`). An undated card has no sent day, so the key uses a fallback day:
- the upload day, for `-f atproto`;
- the day the card was added to the collection, for `collection publish`.

The fallback day doesn't survive a round trip. Download an undated card and upload it again, and it gets a new key. The same postcard is then in the repo twice. Dated cards keep their key.

The clearest case is `collection create -f atproto` run on a later day:
- It builds a fresh temporary collection each run, so every card's `added_at` is that day.
- Every undated card is then uploaded again under a new key.

Downloading with `postcards at://… -f web` re-encodes the image. That changes the hash, and so the key, even for a dated card, which is a separate problem.

Options:
- **Carry the original day across.** An undated record's TID timestamp falls inside its fallback day, so the day can be read back from its rkey. Fetch could store that day as the card's `added_at`, which needs `pkg/collection` to accept one. `RecordKey` would then reproduce the original key exactly.
- **Match by image CID before uploading,** using `ListRecords`. This catches every round trip, including earlier `-f atproto` uploads, but lists the whole repo on each publish.
- **Remember each card's at-uri** in the collection when it's fetched or published, and reuse it.

## Decision

JP chose **match by image CID before uploading** on 14 November. Publish lists the repo's postcard records once and reuses any whose image is the same as a card's stored bytes, whatever its key. That covers running `collection create -f atproto` again, rebuilding a collection from its address and publishing it, and cards uploaded earlier with `-f atproto`.

The client leaves blob CIDs to the server, so matching needs one of: the stored bytes' CID computed locally (CIDv1, raw, SHA-256), or each listed record's blob CID decoded to its digest and compared with the card's `file_sha256`.

Duplicates: if several records share an image, publish prefers the one the collection record already points at, else the lowest key. The match is byte-for-byte.

## Todo
- [x] Pick an approach
- [x] Publish reuses an existing postcard record whose image matches a card's stored bytes, whatever its key
- [x] Running `collection create -f atproto` again on a later day writes no new postcard records
- [x] A card uploaded earlier with `-f atproto` is reused, not uploaded again
- [x] `--dry-run` reports those cards as reused
- [x] `docs/formats/atproto.md` no longer says an earlier `-f atproto` upload is not recognised
