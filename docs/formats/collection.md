# Collection format

A `*.postcards` file is a SQLite database holding a group of postcards together, for browsing and searching in the [postcard viewer app](https://github.com/jphastings/dotpostcard). Unlike the other formats on this page, collections aren't produced by the main `postcards` conversion command — they're built and maintained with the `postcards collection` subcommands.

Each card in a collection is stored as the raw bytes of its [web](web.md) format file (untouched, so it can be extracted byte-for-byte later), alongside a small JPEG thumbnail of its front and the fields extracted from its XMP metadata — sender/recipient names, sent date, location, descriptions and transcriptions, and so on — so the app can list, sort, and filter cards without decoding every image.

Those extracted fields are also indexed in a SQLite FTS5 virtual table, so free-text search across names, people, places, descriptions, and transcriptions is fast even for large collections.

> [!TIP]
> Only web-format postcard files (`*.postcard.webp`, `*.postcard.jpg`, `*.postcard.jpeg`, `*.postcard.png`) can be added to a collection. Convert component bundles to the web format first with `postcards -f web`.

Collections carry a schema version (in both a `meta` table and `PRAGMA user_version`), so future versions of this tool can detect and migrate older collection files.

## Example

```sh
$ postcards collection create trip.postcards pyramids.postcard.webp
Created collection trip.postcards
pyramids
Added 1 card

$ postcards collection ls trip.postcards
pyramids — Alice → Bob (2006-01-02)
1 card

$ postcards collection search trip.postcards pyramids
pyramids: The word 'Front' in large blue letters
1 result
```
