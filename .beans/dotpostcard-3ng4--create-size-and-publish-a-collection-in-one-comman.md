---
# dotpostcard-3ng4
title: Create, size and publish a collection in one command
status: completed
type: feature
priority: normal
created_at: 2026-11-14T20:14:13Z
updated_at: 2026-11-14T20:27:20Z
---

`postcards collection create` only makes a local file from finished web-format cards, so creating a collection, choosing its quality and publishing it to atproto takes several commands, and quality can't be chosen at all.

Make `create` read like the base `postcards` command:

```
postcards collection create [-f postcards,atproto] [-Q quality] --title "Blanks" <sources…>
postcards collection add <collection.postcards> [-Q quality] <sources…>
```

- `-f` defaults to `postcards` (writes `<kebab(title)>.postcards`); `-f atproto` publishes; both together do both.
- Sources: web-format card files, `.postcards` files, directories of those, and `at://` postcard or collection URIs.
- Without `-Q` a card's bytes are stored untouched; with `-Q` it is re-encoded at that quality.
- The published collection record is keyed by its kebab-cased title, so creating again with the same title updates it. A file that already stores a published address keeps using it.
- `collection fetch` is removed: `create` with an `at://` collection URI covers it.
