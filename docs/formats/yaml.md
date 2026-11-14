# Postcard metadata (YAML)

The native format for postcard metadata can be losslessly stored in [YAML](https://en.wikipedia.org/wiki/YAML) or [JSON](https://en.wikipedia.org/wiki/JSON) formats. When produced they will be called `{name}-meta.{ext}`.

A postcard's optional top-level `title` is its human-readable name. When it's absent, tools show a title derived from the file's name (`cnn-center` is shown as "Cnn Center") but never write that derived title back into the metadata.
