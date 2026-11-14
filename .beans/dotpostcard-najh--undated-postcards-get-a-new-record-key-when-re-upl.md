---
# dotpostcard-najh
title: Undated postcards get a new record key when re-uploaded after a download
status: todo
type: bug
created_at: 2026-11-14T09:10:15Z
updated_at: 2026-11-14T09:10:15Z
---

A postcard record's key is a TID made from the day the card was sent plus the image's hash (`atproto.RecordKey`). An undated card has no sent day, so the key uses a fallback day:
- the upload day, for `-f atproto`;
- the day the card was added to the collection, for `collection publish`.

The fallback day doesn't survive a round trip. Download an undated card and upload it again, and it gets a new key. The same postcard is then in the repo twice. Dated cards keep their key.

The clearest case is `collection fetch` followed by `collection publish` to the same account (PR #26):
- Fetch stores each image exactly as published, so the hash matches.
- But every fetched card's `added_at` is the day of the fetch, so every undated card is uploaded again under a new key.

Downloading with `postcards at://… -f web` re-encodes the image. That changes the hash, and so the key, even for a dated card, which is a separate problem.

Options:
- **Carry the original day across.** An undated record's TID timestamp falls inside its fallback day, so the day can be read back from its rkey. Fetch could store that day as the card's `added_at`, which needs `pkg/collection` to accept one. `RecordKey` would then reproduce the original key exactly.
- **Match by image CID before uploading,** using `ListRecords`. This catches every round trip, including earlier `-f atproto` uploads, but lists the whole repo on each publish.
- **Remember each card's at-uri** in the collection when it's fetched or published, and reuse it.

## Todo
- [ ] Pick an approach
- [ ] Fetching a collection and publishing it to the same account writes no new postcard records
