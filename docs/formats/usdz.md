# USDZ format

The [Universal Scene Description format](https://en.wikipedia.org/wiki/Universal_Scene_Description) is a 3D modelling format that this tool can produce using postcard images and metadata, and convert from. When produced these files will be called `{name}.usdz`.

For postcards with only one side stored in the file the produced model will (by default) have the same image on both sides.

Alongside the 3D model data the zip file that is the produced USDZ holds the [web](./web.md) format postcard. This is used directly as the texture file for the 3D model — it can be extracted manually if needed, but this tool will do this while performing consistency checks.
