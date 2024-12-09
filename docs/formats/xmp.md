# XMP Metadata

Postcard metadata can be stored in [XMP format](https://en.wikipedia.org/wiki/Extensible_Metadata_Platform). Usually this is stored inside the images produced with the [web](web.md) output format.

> [!WARNING]
> Some features of metadata (particularly the location of secrets) assumes that the image being described is laid out like a Web format postcard — the front above the back, and the back rotated as in the Web format.

> [!TIP]
> When output directly with `-f xmp` a file `{name}-meta.xmp` will be produced — you may need to rename it to match the name of the postcard file (`{name}.postcard.xmp`) for other tools to recognise the association.

## Structure

The following XMP fields are used to store Postcard metadata. Some of them don't match _perfectly_ semantically (eg. the time the postcard was sent being represented by the tag usually used for when a photo was taken, or the GPS coordinates for the location the postcard references) — but they're close enough for human use. Postcard-specific machine uses should convert them according to the following guide:

Note that the TIFF schema fields (relating to image size) are omitted for XMP output directly (with `postcards -f xmp`), as image size information is only relevant when attached to a specific image.

| XMP field                   | Schema         | Postcard metadata                       | Use                                                                                                                                          |
|-----------------------------|----------------|-----------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------|
| ImageWidth                  | TIFF           | (from source images)                    | The pixel width of the combined image.                                                                                                       |
| ImageLength                 | TIFF           | (from source images)                    | The pixel height of the combined image                                                                                                       |
| XResolution                 | TIFF           | (from source images)                    | The cm width of the combined image (always the width of the front of the postcard)                                                           |
| YResolution                 | TIFF           | (from source images)                    | The cm height of the combined image (always twice the height of the front of the postcard)                                                   |
| ResolutionUnit              | TIFF           | (from source images)                    | Always "3", the indicator for centimetres                                                                                                    |
| Description                 | DC             | -                                       | Always "Both sides of a postcard, stored in the '.postcard' format (https://dotpostcard.org)"                                                |
| DateTimeOriginal            | Exif           | sentOn                                  | The date the postcard was sent                                                                                                               |
| GPSAreaInformation          | Exif           | location.name                           | The name of the location the postcard references                                                                                             |
| GPSLatitude                 | Exif           | location.latitude                       | The latitude of that location                                                                                                                |
| GPSLongitude                | Exif           | location.longitude                      | The longitude of that location                                                                                                               |
| AltTextAccessibility        | IPTC4 XMP Core | front.description, back.transcription   | Generated text suitable to be used as alt text for the postcard                                                                              |
| Transcript                  | IPTC4 XMP Ext  | back.transcription, front.transcription | The transcript of any writing on the the postcard. A § character will divide the back and the front (in that order), if needed               |
| ImageRegionName             | IPTC4 XMP Ext  | -                                       | Always "Private information" for secrets                                                                                                     |
| ImageRegionBoundaryVertices | IPTC4 XMP Ext  | front.secrets, back.secrets             | The (normalized) x, y positions of the edges of the secret region                                                                            |
| ImageRegionBoundaryUnit     | IPTC4 XMP Ext  | -                                       | Always "relative" (the vertex values are normalized to the width and height of the **image**, not the side)                                  |
| Flip                        | Postcard       | flip                                    | Which way the postcard should flip (book, calendar, left-hand, right-hand). **This field should be the one used to detect a postcard image** |
| Sender                      | Postcard       | sender                                  | The name (and possibly URL) of the sender of the postcard                                                                                    |
| Recipient                   | Postcard       | recipient                               | The name (and possibly URL) of the recipient of the postcard                                                                                 |
| Context                     | Postcard       | context.description                     | Any context provided about the postcard. Always has `xml:lang` attribute, which is the `locale` of the metadata for the postcard.            |
| ContextAuthor               | Postcard       | context.author                          | The name (and possibly URL) of the author of the context                                                                                     |
| ThicknessMM                 | Postcard       | physical.thickness_mm                   | The thickness of the postcard, if different to the standard 0.4mm                                                                            |
| DescriptionFront            | Postcard       | front.description                       | An alt-text style description of the front of the postcard                                                                                   |
| DescriptionBack             | Postcard       | back.description                        | An alt-text style description of the back of the postcard                                                                                    |
| TranscriptionFront          | Postcard       | front.transcription                     | The JSON blob representing the transcription of the front of the postcard (with annotations)                                                 |
| TranscriptionBack           | Postcard       | back.transcription                      | The JSON blob representing the transcription of the back of the postcard (with annotations)                                                  |
