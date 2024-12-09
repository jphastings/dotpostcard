# Postcards

A tool for creating archival and postable scans of postcards, and their metadata. The `postcards` CLI tool can create & convert between a number of formats suitable for the web and 3D environments. The `postbox` server can offer the same service over the internet.

See them in use at:
- [shutup.jp](https://shutup.jp)

## Install

```sh
$ go install github.com/jphastings/dotpostcard/cmd/postcards@latest
```

## Usage

To make your first postcard file you should scan both sides of your postcard, its "comonents".

> [!NOTE]
> JPG, PNG and TIFF formats are all supported, but JPG may lose quality, and some scanning software's TIFF format isn't compatible with this software.

> [!TIP]
> Taking a photo will work, but a scanner won't produce reflections or add perspective to your postcard, and can provide accurate information about its physical size — which you'd have to add manually otherwise.

Pick a suitable short name for your postcard, eg. `pyramid`.

> [!TIP]
> Using only a-z, 0-9, hyphens, and underscores is smart as this will prevent issues moving your postcard between computers and ecosystems.

Name your two scans `{name}-front.{ext}` and `{name}-back.{ext}` (or `{name}-only.{ext}` if you're only producing a single sided card).

```sh
$ ls
pyramid-front.png
pyramid-back.png
```

Create a template metadata file & fill it out with the details of your postcard:

```sh
$ postcards init pyramid
⚙︎ Generating 1 postcard metadata file…
Template (Metadata) → (Metadata) pyramid-meta.yaml
```

Now generate any other postcard format from this "component" format:

```sh
$ postcards -f web,usdz,html,css pyramid-meta.yaml
⚙︎ Converting 1 postcard into 4 different formats…
pyramid-meta.yaml (Component files) → (USDZ 3D model) pyramid.postcard.usdz
pyramid-meta.yaml (Component files) → (Web) pyramid.postcard.jpg
pyramid-meta.yaml (Component files) → (HTML) pyramid.html
pyramid-meta.yaml (Component files) → (CSS) postcards.css
```

Here we've produced:

- A [USDZ 3D model](docs/formats/usdz.md) of the postcard, suitable for iOS augmented reality
- A [Web image](docs/formats/web.md) of the postcard, that can be archived (as it holds all the postcard information), and can be used with…
- Some [HTML](docs/formats/html.md) suitable for displaying the postcard Web image on a website
- The [CSS](docs/formats/css.md) needed to display the HTML format in a pretty way

> [!IMPORTANT]
> Most formats can be converted between each other (web, usdz, component). If this is likely use the `--archival` flag to turn on lossless conversion so quality remains high until the final conversion.

## Under construction

This repo is in rapid development (and migration from a previous iteration at [dotpostcard/postcards-go](https://github.com/dotpostcard/postcards-go)) — please bear with me!
