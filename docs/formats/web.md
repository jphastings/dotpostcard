# Web postcard format

The most versatile of the formats, the Web format holds both sides of the postcard within one image (the front above the back). The image contains [XMP](xmp.md) metadata that fully describes the postcard, so this format is also suitable for archive purposes.

If the postcard is 'heteroriented' (the back and front have different orientations) then the lower half will be rotated 90º clockwise (for left-hand flip postcards), or 90º anti-clockwise (for right-hand flip postcards), so that a Web format postcard is always the same width as the front of the postcard, and twice the height of the front.

By default this format uses lossy JPEGli compression and resizing to make a small but high quality image. If your input images use transparency, or if you request an 'archival' quality postcard, then it will produce a WebP format image that can support what you've requested.

## Example

The following three output formats will create a Web format postcard that makes use of the HTML & CSS files to show the postcard in a visually appealing way.

```sh
$ postcards -f web,css,html pyramids-front.jpg
⚙︎ Converting 1 postcard into 3 different formats…
pyramids-front.jpg (Component files) → (Web) pyramids.postcard.jpg
pyramids-front.jpg (Component files) → (CSS) postcards.css
pyramids-front.jpg (Component files) → (HTML) pyramids.html
```
