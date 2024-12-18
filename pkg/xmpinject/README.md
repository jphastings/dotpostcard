# XMP inject

This package provides _very dumb_ code for injecting XMP data into various image formats. They'll likely not play well with images created outside of the Go standard library image generation (ie. images without other metadata chunks).

The test fixture images are generated with `exiftool` like this:

```sh
exiftool "-xmp<=../../internal/testhelpers/sample-meta.xmp" 1px-nometa.png
mv 1px-nometa.png 1px-xmp.png
mv 1px-nometa.png_original 1px-nometa.png
```
