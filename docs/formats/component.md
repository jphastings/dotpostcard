# Postcard components

This format describes having images for each side of your postcard. If you're scanning postcards and preparing them for use elsewhere with this tool then you'll be starting with this.

If you convert _to_ this format then you'll receive one image file for each side of the postcard. If you convert _from_ this format then you'll need one photo for each side you want to include _and_ a [metadata](yaml.md) file. The filenames _will all have_ (for output) or _must all have_ (for input) a common structure: `{name}-{type}.{ext}`. Eg:

| Side files                                              | `{name}`    | Web file                 |
|---------------------------------------------------------|-------------|--------------------------|
| `mine-front.jpg`<br>`mine-back.png`<br>`mine-meta.yaml` | `postcard`  | `mine.postcard.webp`     |
| `a-pc-front.png`<br>`a-pc-back.png`<br>`a-pc-meta.json` | `a-pc`      | `a-pc.postcard.webp`     |
| `one-sided-only.jpg`<br>`one-sided-meta.yaml`           | `one-sided` | `one-sided.postcard.jpg` |

## Notes

- Metadata can be provided in YAML, JSON or XMP format, as desired — YAML is the easiest to write by hand, use `postcards init mine-front.jpg` to create a `mine-meta.yaml` file with examples & comments to help you.
- The front & back can be in any supported image format, but their _physical dimensions_ must be close to each other
- Hyphens can be used in the `{name}` part, so long as the suffix is present (eg. `-front`)
- Transparency is preserved in input images (representing the postcard's shape). If present the mask should probably be the same (but suitably flipped) between the front and the back of the postcard (because they're two sides of the same object). This isn't enforced, but may cause issues with the [3D output formats](usdz.md).
- A metadata file must be present, and meet the minimum requirements as defined in the [metadata definition](yaml.md)
