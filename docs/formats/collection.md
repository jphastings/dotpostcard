# Collection format

A `*.postcards` file is a SQLite database holding a group of postcards together, for browsing and searching in the [postcard viewer app](https://github.com/jphastings/dotpostcard). Unlike the other formats on this page, collections aren't produced by the main `postcards` conversion command — they're built and maintained with the `postcards collection` subcommands.

Each card in a collection is stored as the raw bytes of its [web](web.md) format file (untouched, so it can be extracted byte-for-byte later), alongside a small JPEG thumbnail of its front and the fields extracted from its XMP metadata — sender/recipient names, sent date, location, descriptions and transcriptions, and so on — so the app can list, sort, and filter cards without decoding every image.

Each card has a `title`: its own [title](yaml.md) if its metadata has one, otherwise one derived from its name (`cnn-center` becomes "Cnn Center"). The name stays the card's unique key; the title is what `ls` and search results lead with.

Those extracted fields, titles included, are also indexed in a SQLite FTS5 virtual table, so free-text search across names, titles, people, places, descriptions, and transcriptions is fast even for large collections.

> [!TIP]
> Only web-format postcard files (`*.postcard`, `*.postcard.webp`, `*.postcard.jpg`, `*.postcard.jpeg`, `*.postcard.png`) can be added to a collection. Convert component bundles to the web format first with `postcards -f web`.

Collections carry a schema version (in both a `meta` table and `PRAGMA user_version`), so future versions of this tool can detect and migrate older collection files. Migrating an older file backfills each card's title and rebuilds the search index; this needs the collection opened read-write, and a read-only open of an outdated file is refused.

## Publishing

A collection can be published to an atproto PDS as a single record listing its cards, and built again from one: see [Collections](atproto.md#collections) in the ATProto format. `postcards collection publish` stores the record's at-uri in the file (`published_uri`) so later publishes update the same record; `postcards collection fetch` creates a new collection from a published one.

## Example

```sh
$ postcards collection create trip.postcards pyramids.postcard.webp
Created collection trip.postcards
pyramids
Added 1 card

$ postcards collection ls trip.postcards
Pyramids — Alice → Bob (2006-01-02)
1 card

$ postcards collection search trip.postcards pyramids
Pyramids: The word 'Front' in large blue letters
1 result

$ postcards collection publish trip.postcards --at-user alice.example
pyramids → (ATProto) at://did:plc:.../org.dotpostcard.postcard/3jzfcijpj2z2a
Published 1 card as at://did:plc:.../org.dotpostcard.collection/3lxyzabcdef2c
```
