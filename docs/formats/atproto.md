# ATProto format

Uploads postcards to, and downloads them from, an [atproto](https://atproto.com) PDS (the same network Bluesky runs on), as records of collection `org.dotpostcard.postcard`. This isn't a file format like the others on this page — it doesn't produce local files, and it isn't a registered codec — but it's requested and behaves the same way from the CLI.

## Uploading

Request it with `-f atproto`, alongside or instead of any other format:

```sh
$ postcards -f atproto,web --at-user alice.example --at-password xxxx-xxxx-xxxx-xxxx pyramids-front.jpg
⚙︎ Converting 1 postcard into 1 different format…
pyramids-front.jpg (Component files) → (Web) pyramids.postcard.jpg
pyramids-front.jpg (Component files) → (ATProto) at://did:plc:.../org.dotpostcard.postcard/3jzfcijpj2z2a (312ms)
```

Credentials come from `--at-user`/`--at-password`, or — in the same order [goat](https://github.com/bluesky-social/indigo/tree/main/cmd/goat) checks — the `GOAT_USERNAME`/`GOAT_PASSWORD`, `ATP_USERNAME`/`ATP_PASSWORD`, or `ATP_AUTH_USERNAME`/`ATP_AUTH_PASSWORD` environment variables. `--at-user` accepts a handle or a DID; the password is a PDS [app password](https://bsky.app/settings/app-passwords), not your main account password. The login happens once per run, before any card is processed.

`ATP_PDS_HOST` overrides where a handle/DID would otherwise resolve to, for both uploads and downloads. `ATP_PLC_HOST` likewise overrides the PLC directory (`https://plc.directory`) that a `did:plc` resolves through. Both are mainly useful for testing against a non-production network.

### The record key

The record's key (`rkey`) is a [TID](https://atproto.com/specs/tid): a timestamp (the day the postcard was sent, or the day it's uploaded if that's unknown — clamped forward to 1970-01-01 for anything sent before then, which is common for postcards) combined with bits taken from the uploaded image's own hash. It isn't the postcard's name — names aren't unique, sortable, or guaranteed to fit atproto's rkey syntax, and a TID is all three.

A consequence of hashing the image into the key: re-uploading a card whose image and metadata haven't changed computes the exact same key, so it's treated exactly like any other existing output (`--overwrite` to replace it, `--skip-existing` to leave it alone) — this check happens right after encoding, before anything is actually uploaded. Change the image or its metadata at all (including re-running with different options — the embedded XMP will differ) and re-uploading creates a new record at a new key, leaving the old one in place. An undated card also gets a different key if uploaded on a different day.

## Downloading

Give an `at://` record URI in place of a file, with any other output format:

```sh
$ postcards -f web at://alice.example/org.dotpostcard.postcard/3jzfcijpj2z2a
```

Downloads don't need to be logged in — the URI's authority (a handle or DID) is resolved independently, and its PDS is found the normal atproto way (via `.well-known/atproto-did`, a `_atproto.` DNS TXT record, or `plc.directory`/`did:web`).

Since the rkey is a TID, not a name, a downloaded card is named instead from its record's location: the location name, lower-cased and kebab-ed (`"St. John's, Newfoundland"` becomes `st-john-s-newfoundland`). If there's no usable location, the card is named after the first characters of the record's own CID (skipping the `bafyrei` prefix every record CID shares).

## The record wins

A postcard's image is a self-describing [web](web.md) format file: it carries its own metadata in [XMP](xmp.md), same as any other web-format postcard. When downloading, that embedded metadata is compared against the record's — if they disagree, a warning names the differing fields (eg. `record and image metadata differ: context, sides[1].transcription`) and **the record's values are used**. Only what the image alone can carry — its exact pixel dimensions, and whether it has transparency — is taken from the image regardless.

This should rarely happen in practice, since uploading always builds the record from the same metadata that ends up embedded in the image. It's there for records that get hand-edited, or edited through some other atproto client, without the image being re-uploaded to match.

## What's lossy

The lexicon (`org.dotpostcard.postcard`, see [lexicons/org/dotpostcard](../../lexicons/org/dotpostcard/postcard.json)) can't represent every field at full precision:

- Physical size is stored to the nearest millimetre, not the exact fraction this tool otherwise carries around.
- Secret region points are stored to 1/10,000th of the side's width/height.
- Postcard thickness is stored in whole micrometres.
- A secret's `prehidden` field defaults to `true` (most secrets are already obscured before upload) and is only written when `false`, so it can't distinguish "prehidden, and said so" from "prehidden by default" — both read back the same way.

Round-tripping through atproto and back will reproduce these fields' rounded values, not necessarily the exact originals.
