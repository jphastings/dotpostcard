# Postcards

A tool for creating archival and postable scans of postcards, and their metadata. The `postcards` CLI tool can create & convert between a number of formats suitable for the web and 3D environments. The `postbox` server can offer the same service over the internet.

See them in use at:
- [shutup.jp](https://shutup.jp)

## Install

If you have [Homebrew](https://brew.sh) installed:

```sh
brew install jphastings/tools/postcards
```

You can also download compiled binaries from [Github releases](https://github.com/jphastings/dotpostcard/releases/) for Windows, Linux, macOS — each of which is provided for arm64 and amd64 architectures.

WASIp1 compatible WASM executables are also provided with a reduced feature set (notably the more efficient image encoders, WebP and JPEGli, are absent).

## Usage

To make your first postcard file you should scan both sides of your postcard, its "components".

- Use a scanner (for accurate resolution info)
- Use a black background (as black as possible, to minimise shadows)
- Save your scan as a PNG (For quality, and because Go doesn't support macOS' TIFF encoding)

> [!TIP]
> There is an experimental web-based postcard creator at [create.dotpostcard.org](https://create.dotpostcard.org), which you may find easier.

Pick a name for your postcard, eg. `pyramid`, and name your scans `{name}-front.png` and `{name}-back.png` (or `{name}-only.png` if you're only producing a single sided card).

Create a template metadata file & edit it to show the details of your postcard:

```sh
$ postcards init pyramid
⚙︎ Generating 1 postcard metadata file…
Template (Metadata) → (Metadata) pyramid-meta.yaml

$ vi pyramid-meta.yaml

$ ls
pyramid-back.png
pyramid-front.png
pyramid-meta.yaml
```

Now you can generate any other postcard format from this "component" format:

```sh
$ postcards -f usdz,web,html,css pyramid-meta.yaml
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
