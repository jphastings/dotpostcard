# XMP inject

This package provides _very dumb_ code for injecting XMP data into various image formats. They'll likely not play well with images created outside of the Go standard library image generation (ie. images without other metadata chunks).

The JPEG codec is the exception: it walks JPEG's marker segments properly (rather than assuming a fixed layout), and payloads too large for a single APP1 segment are split across Adobe's standard ExtendedXMP APP1 segments (XMP Specification Part 3), so it should play well with JPEGs from other tools too.

The test fixture images are generated with `exiftool` like this:

```sh
exiftool "-xmp<=../../internal/testhelpers/sample-meta.xmp" 1px-nometa.png
mv 1px-nometa.png 1px-xmp.png
mv 1px-nometa.png_original 1px-nometa.png
```
