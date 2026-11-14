---
# dotpostcard-2p1j
title: Let other lexicons embed a postcard by reference
status: draft
type: feature
created_at: 2026-11-14T14:28:35Z
updated_at: 2026-11-14T14:52:33Z
---

**Don't start this unless Unsent's first spike fails.**

Unsent stores a full copy of a postcard, image blob included, inside its own `me.byjp.unsent.card` record. Its plan does that with an untyped (`unknown`) field holding a postcard record value. Its first spike tests whether a PDS keeps a blob that is referenced only from inside an untyped field.

If the spike fails, another lexicon needs something typed to point at: the postcard's fields as a named object definition (say `org.dotpostcard.postcard#postcard`) that `me.byjp.unsent.card` can `ref`. The JSON of existing postcard records must not change.

Lexicon may not allow a record's `record` to be a `ref`. If it doesn't, the object definition sits beside `main` with the same properties, and a test keeps the two identical.

## Todo
- [ ] The result of Unsent's spike 1 is recorded here
- [ ] If it failed: add the named object definition, with `main` using it or mirroring it
- [ ] Existing records still validate, unchanged
- [ ] Go types and `docs/formats/atproto.md` updated
