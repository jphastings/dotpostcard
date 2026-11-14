---
# dotpostcard-ep94
title: Publish the org.dotpostcard lexicons as atproto schema records
status: in-progress
type: feature
created_at: 2026-11-14T14:28:35Z
updated_at: 2026-11-14T14:52:33Z
---

`org.dotpostcard.postcard` and `org.dotpostcard.collection` only exist as JSON files in `lexicons/`. Other tools can't resolve them by NSID, so they can't validate or describe these records. JP decided on 14 November that they should be published.

Publishing a lexicon needs two things:
- a `com.atproto.lexicon.schema` record for each NSID, in one account's repo, keyed by the NSID and holding the lexicon JSON;
- a DNS TXT record at `_lexicon.dotpostcard.org` holding `did=<that account's DID>`.

Unsent (`me.byjp.unsent.*`) will publish its lexicons the same way, and its card record embeds a postcard record, so resolvable schemas help it directly.

Two parts are JP's: naming the account that holds the records, and adding the TXT record.

String limits (`maxLength`) were added to `postcard.json` and `collection.json` before publishing, because limits can't sensibly be tightened afterwards; the uploader refuses a card over them.

## JP's steps
1. Create the `did:web:dotpostcard.org` account.
2. Add the DNS record `_lexicon.dotpostcard.org TXT "did=did:web:dotpostcard.org"`.
3. Run `make publish-lexicons` with that account's credentials.
4. Check both NSIDs: `goat lex resolve org.dotpostcard.postcard` and `goat lex resolve org.dotpostcard.collection`.

## Todo
- [x] JP names the account: the dedicated `did:web:dotpostcard.org` account, which doesn't exist yet
- [x] A repeatable way to publish: a `make` target (`make publish-lexicons`) or `postcards` subcommand that writes each file in `lexicons/` as a schema record, and is safe to run again
- [ ] JP adds the `_lexicon.dotpostcard.org` TXT record
- [ ] Both NSIDs resolve through a lexicon resolver
- [x] `docs/formats/atproto.md` says where the published schemas live (true once JP's steps are done)
