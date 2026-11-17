# Compatibility

When changing any file format or record lexicon, aim for both:

- **Backward compatibility**: new code reads files and records written by old code.
- **Forward compatibility**: old code tolerates files and records written by new code, for example by only adding optional fields.

If a change can't be compatible, ship a migration step for that format in the same release. Migrations must be lossless: image data is left untouched and all metadata is preserved.

For [collections](formats/collection.md), append a step to the ladder in `pkg/collection/migrate.go` and update `schemaStatements` to match. Apps reach migration through `appcore.CollectionSchemaStatus` and `appcore.MigrateCollection`.

## Known gaps

atproto `org.dotpostcard.postcard` records written by v0.28.0 use a shape from before v0.29.0 that current versions can't read. There's no record migrator yet; add one under `formats/atproto` if those records ever need to load.
