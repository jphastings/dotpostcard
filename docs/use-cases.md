# Postcards

When I'm archiving images of a postcard
  I want to store the front, back, and context/metadata (physical size, how it flips) together
    So that the component parts aren't accidentally lost/separated.

  I want to be able to mark areas as private and tastefully hide the information within them
    So that I can hide my address when sharing or displaying.

  I want to be able to use a command line tool
    So I can archive many postcards quickly and in an automated way.

  I want to be able to use a web-based tool to archive a postcard
    So that I can do it without needing anything installed.

  I want to be able to store text transcriptions of the writing on the postcard
    So I can search through a large postcard collection easily.

When I am displaying an archived postcard
  I want it to be easy and pretty to show on the web
    So that I get around to actually displaying it.

  I want to be able to convert my postcard into a 3D format (eg. USDZ)
    So that the physicality of the postcard is preserved.

  I want to be able to extract metadata in easy to process formats (eg. JSON, YAML)
    So that I can create static sites that show off the metadata as well as the front/back.

## Command line tool

Example uses:

```bash
$ postcards --help
Usage: postcards [--output list,of,formats] [flags] postcard1-front.jpg [postcard2.yaml]

$ tree
.
├── postcard1-front.jpg
├── postcard1-back.jpg
├── postcard2.webp
├── postcard3.usdz
├── postcard4-only.png
└── postcard4-meta.yaml

# To build a postcard from scratch put a 'postcard1-front.{jpg,png,web}' and
# a matching 'postcard1-back.{web,png,jpg}' in the same directory together and
# request a metadata format (json or yaml) to have a template created for you
$ postcards --output yaml postcard1-front.png
postcard1: skipped, as metadata is missing
↪ Wrote new metadata file to postcard1-meta.yaml
ℹ Edit this text file with information about your postcard

$ tree
.
├── postcard1-front.jpg
├── postcard1-back.jpg
├── postcard1-meta.yaml
├── postcard2.webp
├── postcard3.usdz
├── postcard4-only.png
└── postcard4-meta.yaml

# Create combined 'web' postcards from constituent parts, and display easily on the web
$ postcards --output web,json,css postcard1-front.jpg
postcard1: 10cm x 15cm (136 dpi)
↪ Wrote web postcard file to postcard1.webp
↪ Wrote metadata file to postcard1.json
↪ Wrote standard postcard CSS file to postcards.css
ℹ This CSS expects your postcard HTML to be an image wrapped in a postcard div:
  <div class="postcard"><img src="your-postcard.webp" /></div>

# Formats can be converted between losslessly
$ postcards --output web,yaml postcard3.usdz
postcard3: 12cm x 12cm (136 dpi)
↪ Wrote web postcard file to postcard3.webp
↪ Wrote metadata file to postcard3.yaml

$ postcards --output 3d.json postcard2.webp
postcard2: 14.8cm x 10.5cm (136 dpi)
↪ Wrote 3D postcard file to postcard2.usdz
↪ Wrote metadata file to postcard3.json

# Whole directories can be processed at once
$ postcards --output json *
postcard1: 10.5cm x 14.8cm (136 dpi)
↪ Wrote metadata file to postcard1.json
postcard2: 14.8cm x 10.5cm (136 dpi)
↪ Wrote metadata file to postcard2.json
postcard3: 12cm x 12cm (136 dpi)
↪ Wrote metadata file to postcard3.json
postcard4: 17.7cm x 12.7cm (136 dpi)
↪ Wrote metadata file to postcard4.json
```

### Flags

| Flag           | Example     | Purpose                                               | Default                                           |
|----------------|-------------|-------------------------------------------------------|---------------------------------------------------|
| -o, --output   | -o web,json | Choose the output formats to create                   | Empty; no conversion, just info                   |
| -A, --archival | -A          | Uses lossless compression & does not downscale images | Off; fits within 2048x2048px & compresses lossily |

## Formats

| Format    | Filename                   | Purpose                                                              | Convertible?             |
|-----------|----------------------------|----------------------------------------------------------------------|--------------------------|
| web       | name.webp[^1]              | A stacked front/back webp image with embedded XMP metadata           | Yes, no loss             |
| json,yaml | name-meta.{json,yaml}      | A simple JSON/YAML file describing the metadata about a postcard     | Yes, needs `sides`       |
| sides     | name-{front,back}.webp[^1] | The separate front and back images of the postcard                   | Yes, needs `json`/`yaml` |
| usdz,3d   | name.usdz                  | A USDZ 3D model of the postcard (to scale)                           | Yes, no loss             |
| css       | postcards.css              | Outputs the (unchanging) CSS needed to display a postcard on the web | n/a                      |

[^1]: These file formats are always output in the WebP format, but any of JPEG, PNG, WebP can be imported with their usual extensions, or (in the case of the 'web' output format) with the `.postcard` extension.
