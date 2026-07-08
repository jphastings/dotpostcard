# Web postcard format

The most versatile of the formats, the Web format holds both sides of the postcard within one image (the front above the back). The image contains [XMP](xmp.md) metadata that fully describes the postcard, so this format is also suitable for archive purposes.

If the postcard is 'heteroriented' (the back and front have different orientations) then the lower half will be rotated 90º clockwise (for left-hand flip postcards), or 90º anti-clockwise (for right-hand flip postcards), so that a Web format postcard is always the same width as the front of the postcard, and twice the height of the front.

> [!TIP]
> By default this format uses lossy JPEGli compression and resizing to make a small but high quality image. If your input images use transparency, or if you request an 'archival' quality postcard, then it will produce a WebP format image that can support what you've requested.

## Example

The following three output formats will create a Web format postcard that makes use of the HTML & CSS files to show the postcard in a visually appealing way.

```sh
$ postcards -f web,css,html pyramids-front.jpg
⚙︎ Converting 1 postcard into 3 different formats…
pyramids-front.jpg (Component files) → (Web) pyramids.postcard.jpg
pyramids-front.jpg (Component files) → (CSS) postcards.css
pyramids-front.jpg (Component files) → (HTML) pyramids.html
```

## Single-extension variant (`postcard`)

The `postcard` format writes the exact same bytes as `web`, but names the file `{name}.postcard`, without a trailing image-format extension.

This exists because macOS QuickLook can't be made to show a custom preview for a file whose extension maps to a system-recognised image UTI (like `public.jpeg` or `public.webp`) — the system's own image preview always wins over a third-party QuickLook extension. Dropping the extension lets a `.postcard` file get its own, dedicated QuickLook preview.

Because the filename no longer carries the codec extension, this format doesn't support the `css`/`html` support files (which need to reference the image by its full, extended filename); requesting them alongside `postcard` is an error. Decoding is unaffected: the image codec is still detected by sniffing the file's content, exactly as with `web`.

```sh
$ postcards -f postcard pyramids-front.jpg
⚙︎ Converting 1 postcard into 1 different format…
pyramids-front.jpg (Component files) → (Web) pyramids.postcard
```
