---
# dotpostcard-6i5b
title: Check collection publishing against a real PDS
status: todo
type: task
created_at: 2026-11-14T14:28:35Z
updated_at: 2026-11-14T14:28:35Z
---

`postcards collection create -f atproto` (publishing straight from files) and `collection create` from an `at://` collection URI are tested against the in-repo fake PDS. Two things only a real PDS can show:

- **Blobs survive.** A blob is kept only while a record references it. Check that the images of published postcards are still served a day after publishing.
- **Record size.** JP expects a collection to stay under 500 cards, which is about 75 KB of strongRefs, so one record should be enough and collections aren't split. What isn't known is how large a record a PDS accepts, or what its refusal looks like. A failed collection write now names the collection's size ahead of the PDS's own error.

## Todo
- [ ] Publish a three-card collection to a test account with `collection create -f atproto`, then rebuild it from its `at://` address
- [ ] A day later, every blob is still served
- [ ] Publish a 500-card collection (repeated or synthetic cards are fine) and note whether it is accepted
- [x] If a PDS refuses a large collection record, the error names the collection's size and says collections aren't split (applies to any failed collection write, as the shape of a real refusal isn't known)

## How to run it

Use a throwaway account. `postcards` reads `GOAT_USERNAME` and `GOAT_PASSWORD` (an app password). `$CARDS` is a directory of three web-format cards.

```sh
export GOAT_USERNAME=test.example.com GOAT_PASSWORD=xxxx-xxxx-xxxx-xxxx
CARDS=./three-cards
```

1. Three-card round trip. Dry run first, then publish, then rebuild from the printed `at://` address:

```sh
postcards collection create -f atproto --title "PDS check" --dry-run "$CARDS"
postcards collection create -f atproto --title "PDS check" "$CARDS"
# note the "Published 3 cards as at://…" address
AT=at://did:plc:…/org.dotpostcard.collection/pds-check
mkdir -p /tmp/pds-check && postcards collection create --title "PDS check" --out-dir /tmp/pds-check "$AT"
postcards collection ls /tmp/pds-check/pds-check.postcards
```

Compare that listing with the originals (`postcards collection create --title x "$CARDS" --out-dir /tmp/orig` then `collection ls` on it).

2. Size test. Identical images publish as a single postcard record, so the collection record still carries 500 strongRefs (the size being tested) with only one blob uploaded. Cards are deduplicated on name plus content, so 500 differently-named copies are stored as 500 cards.

```sh
mkdir -p /tmp/big && FILE=$(ls "$CARDS"/*.postcard.* | head -1)
for i in $(seq -w 1 500); do cp "$FILE" "/tmp/big/card-$i.postcard.${FILE##*.}"; done
postcards collection create -f atproto --title "PDS size check" --dry-run /tmp/big
postcards collection create -f atproto --title "PDS size check" /tmp/big
```

3. A day later, rebuild the three-card collection again. It downloads every blob, so success means they survived:

```sh
mkdir -p /tmp/pds-check-later && postcards collection create --title "PDS check" --out-dir /tmp/pds-check-later "$AT"
```

## Results

- 500-card collection accepted?
- Record size (KB):
- Error text if refused:
- Blobs still served after a day?
