# USDZ format

The [Universal Scene Description format](https://en.wikipedia.org/wiki/Universal_Scene_Description) is a 3D modelling format that this tool can produce using postcard images and metadata, and convert from. When produced these files will be called `{name}.usdz`.

For postcards with only one side stored in the file the produced model will (by default) have the same image on both sides.

Alongside the 3D model data the zip file that is the produced USDZ holds the [web](./web.md) format postcard in JPEG form. This is used directly as the texture file for the 3D model — it can be extracted manually if needed, but this tool will do this while performing consistency checks.

## Requirements

USDZ creation requires that the `usdzip` tool is installed on yours system (see [OpenUSD](https://openusd.org/) for information). If you don't have access to that tool, you can still export in the `usd` format, which produces both the unzipped (and text-formatted) version of USD and the necessary texture file.
