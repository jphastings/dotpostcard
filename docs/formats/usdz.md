# Universal Scene Description (zip) format

[USDZ](https://en.wikipedia.org/wiki/Universal_Scene_Description) is a 3D modelling format used extensively by Pixar and Apple. Postcards are created with the correct physical dimensions for augmented reality usage.

For postcards with only one side stored in the file the produced model will (by default) have the same image on both sides.

Alongside the 3D model data the zip file that is the produced USDZ holds the [web](web.md) format postcard in JPEG form. This is used directly as the texture file for the 3D model — it can be extracted manually if needed, but this tool will do this while performing consistency checks.

> [!NOTE]
> The USD contains Apple ARKit-specific extensions that flip the postcard (along the correct axis) when it is tapped. Those these extensions won't work on other platforms, they won't cause any problems either.

> [!WARNING]
> Postcards with transparent borders (like those created with the `-B` flag) don't work well with the USDZ format (yet). The transparency is ignored, and the postcard will look a little odd, particularly if it isn't rectangular.
>
> Eventually I'd like to alter the 3D model's geometry to match the outermost opaque pixels of a transparent-border postcard, but that's a complex operation I've not yet got around to.

## Requirements

USDZ creation requires that the `usdzip` tool is installed on yours system (see [OpenUSD](https://openusd.org/) for information). If you don't have access to that tool, you can still export in the `usd` format, which produces both the unzipped (and text-formatted) version of USD and the necessary texture file.

## Example

```sh
$ postcards -f usdz pyramids-front.jpg
⚙︎ Converting 1 postcard into 1 different format…
pyramids-front.jpg (Component files) → (USDZ 3D model) pyramids.postcard.usdz
```
