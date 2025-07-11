## 0.12.0 (2025-03-08)

### Feat

- Allow specifying the country as part of the location

## 0.11.14 (2025-02-20)

### Perf

- :arrow_up: Bump versions

## 0.11.13 (2025-01-25)

### Fix

- Overwrite wasm_exec files if needed

## 0.11.12 (2025-01-25)

### Fix

- :green_heart: Update go.sum & README, to allow build to work

## 0.11.11 (2025-01-25)

### Fix

- :art: Fix lint issue with Github Action

## 0.11.10 (2025-01-25)

### Fix

- :green_heart: Copy correct wasm & service worker exec files

## 0.11.9 (2025-01-25)

### Fix

- :green_heart: Install TinyGo before build

## 0.11.8 (2025-01-25)

### Fix

- :green_heart: Install QTC binary before build

## 0.11.7 (2025-01-25)

### Fix

- :bug: Fix build bug, makes qtc available & generates after go mod

## 0.11.6 (2025-01-25)

### Refactor

- :zap: Swap templating library & compile WASM with TinyGo

## 0.11.5 (2025-01-06)

### Fix

- **postoffice**: Show useful errors on failure

## 0.11.4 (2025-01-06)

### Fix

- **postoffice**: :bug: Detect & give reasonable error when input image too large

## 0.11.3 (2025-01-06)

### Fix

- **postoffice**: Simplify HTML output & fix flip none output

## 0.11.2 (2025-01-05)

### Fix

- **postoffice**: :bug: Allow decimals on lat/long input

## 0.11.1 (2025-01-05)

### Fix

- **postoffice**: Switch to local wasm_exec.js and sw.js

## 0.11.0 (2024-12-19)

### Feat

- **postoffice**: Allow selection of secrets in PostOffice
- **postoffice**: Show front/back image when chosen

### Fix

- **postoffice**: Fix .only-empty to only show without items

## 0.10.5 (2024-12-18)

### Fix

- Update TODO.md

## 0.10.4 (2024-12-18)

### Fix

- Move experimental marker to the right

## 0.10.3 (2024-12-18)

### Fix

- Move Checkboxes to the right side

## 0.10.2 (2024-12-18)

### Fix

- **postoffice**: Stop loading after downloading a file

## 0.10.1 (2024-12-18)

### Fix

- **postoffice**: Hide output when restarting form

## 0.10.0 (2024-12-18)

### Feat

- **postoffice**: Add loading spinner while creating
- **postoffice**: Show only the appropriate flip choices
- **postoffice**: :art: Style the Postoffice

## 0.9.3 (2024-12-18)

### Fix

- Use Upload Pages Artifact action

## 0.9.2 (2024-12-18)

### Fix

- Deploy static postoffice to Github Pages

## 0.9.1 (2024-12-18)

### Fix

- **postoffice**: :bug: Forgot to merge extra git chunk

## 0.9.0 (2024-12-18)

### Feat

- **postoffice**: Allow noJS friendly web response

### Fix

- :bug: Prevent segfault extracting empty lat/long from XMP

## 0.8.4 (2024-12-18)

### Fix

- Swap back to WebP as default transparent/lossless format for web

## 0.8.3 (2024-12-18)

### Fix

- :bug: Fix WebP XMP padding byte issue
- :bug: Correct the CRC32 calculated for the iTXt PNG chunk

## 0.8.2 (2024-12-17)

### Fix

- Prevent segfault when no SentOn date

## 0.8.1 (2024-12-17)

### Fix

- Handle finding -only images as part of a component bundle

## 0.8.0 (2024-12-17)

### Feat

- :technologist: Add the postcards info command

### Fix

- Prevent empty fields from appearing in YAML
- :poop: Custom image decoder selection

## 0.7.9 (2024-12-17)

### Fix

- Prevent flip for single-sided cards

## 0.7.8 (2024-12-16)

### Fix

- :bug: Set the *right* HTML codes for annotations

## 0.7.7 (2024-12-16)

### Fix

- Correct HTML output for underline Annotation

## 0.7.6 (2024-12-16)

### Fix

- :bug: Correctly half the web decoder's FrontSize.PxHeight

## 0.7.5 (2024-12-16)

### Fix

- :bug: Correct TIDD width/height ordering on EXIF read

## 0.7.4 (2024-12-16)

### Fix

- **border-detector**: :bug: Fix incorrect scaling threshold value

## 0.7.3 (2024-12-16)

### Fix

- Align chunks with even offsets

## 0.7.2 (2024-12-16)

### Perf

- :arrow_up: Upgrade dependencies

## 0.7.1 (2024-12-16)

### Fix

- Improve transparency algorithm
- Allow ignoring transparency and removing border

### Refactor

- :art: Break border removal into own file

## 0.7.0 (2024-12-14)

### Feat

- :lipstick: Add Underline annotation style

## 0.6.9 (2024-12-14)

### Fix

- Work around Goreleaser brew.ids issue

## 0.6.8 (2024-12-14)

### Fix

- Allow different binary counts

## 0.6.7 (2024-12-14)

### Fix

- Ensure binaries have the same count

## 0.6.6 (2024-12-14)

### Fix

- Remove unneeded rm command in build

## 0.6.5 (2024-12-14)

### Fix

- :construction_worker: Ensure the build can complete, even with WASM in the www/postoffice dir

## 0.6.4 (2024-12-14)

### Fix

- Update TODO & force a build

## 0.6.3 (2024-12-14)

### Fix

- 👷‍♂️ Correct order of builds in Github Actions

## 0.6.2 (2024-12-14)

### Fix

- 👷‍♂️ Fix "Unknown flag --id"

## 0.6.1 (2024-12-14)

### Fix

- :construction_worker: Don;t push service worker to homebrew

## 0.6.0 (2024-12-14)

### Feat

- :construction: Add web-based postcard maker, including WASM
- :construction: Support WASM as compilation target

### Fix

- Add inputs for all postcard metadata

## 0.5.0 (2024-12-13)

### Feat

- :children_crossing: Allow USDZ creation without OpenUSD

## 0.4.0 (2024-12-11)

### Feat

- :necktie: Allow XMP as component input

### Fix

- :white_check_mark: Add YAML tests and fix discovered issue

## 0.3.0 (2024-12-10)

### Feat

- Allow ignoring transparency

### Fix

- :adhesive_bandage: Collect the physical & pixel size from XMP

## 0.2.4 (2024-12-10)

### Fix

- :construction_worker: Bump github actions & ship to correct tap dir

## 0.2.3 (2024-12-10)

### Fix

- :construction_worker: Pass correct token to GoReleaser Homebrew

## 0.2.2 (2024-12-10)

### Fix

- :construction_worker: Fix Goreleaser workflow

## 0.2.1 (2024-12-10)

### Fix

- :memo: Updating TODO and triggering first build

## 0.2.0 (2024-12-10)

## v0.14.15 (2025-07-12)

### Fix

- go mod tidy

## v0.14.14 (2025-07-12)

### Fix

- allow creating non-lossless WebP images (#3)

## v0.14.13 (2025-07-11)

### Fix

- tidy go mod

## v0.14.12 (2025-07-11)

### Fix

- fix failing tests after card colour
- hacky detection of transparent postcards decoded from web versions
- fix: speed up secrets processing

## v0.14.11 (2025-04-11)

### Fix

- **postoffice**: :bug: Ensure USD only gets texture, not HTML/CSS

## v0.14.10 (2025-04-10)

### Fix

- **postoffice**: Attempt to fix webserver path issues

## v0.14.9 (2025-04-09)

### Fix

- Add CardColor into XMP data

## v0.14.8 (2025-04-09)

### Fix

- :heavy_minus_sign: Remove dependency on deprecated package

## v0.14.7 (2025-04-09)

### Fix

- Handle USDZ alignment paddings less than 4 bytes long

## v0.14.6 (2025-04-09)

### Fix

- Create fully valid USDZ
- Correctly offset USDZ file data

### Refactor

- Better error messages on localhost
- **postoffice**: Allow for other endpoints within wasm

## v0.14.5 (2025-04-06)

### Fix

- Switches the HTML and CSS "codecs" to be support files of the web output (which they are), so that they can be affected by the choices within it (eg. the image format being used).

## v0.14.4 (2025-04-06)

### Fix

- :recycle: Prevent panic when encode opts are nil

## v0.14.3 (2025-04-04)

### Fix

- **postoffice**: :lipstick: Remove HTML/CSS boxes for USD download

## v0.14.2 (2025-04-04)

### Fix

- Treat edges of image as transparent
- Support card colour and thickness in 3D models

## v0.14.1 (2025-04-04)

### Fix

- Fix dependencies

## v0.14.0 (2025-04-04)

### Feat

- :sparkles: 3D models that represent postcard transparency

### Refactor

- :fire: Remove old template format

## v0.13.2 (2025-03-08)

### Fix

- :construction_worker: Inject and retrieve countrycode from XMP

## v0.13.1 (2025-03-08)

### Fix

- :construction_worker: Fix go.mod

## v0.13.0 (2025-03-08)

### Feat

- Allow specifying the country as part of the location
- **postoffice**: Allow selection of secrets in PostOffice
- **postoffice**: Show front/back image when chosen
- **postoffice**: Add loading spinner while creating
- **postoffice**: Show only the appropriate flip choices
- **postoffice**: :art: Style the Postoffice
- **postoffice**: Allow noJS friendly web response
- :technologist: Add the postcards info command
- :lipstick: Add Underline annotation style
- :construction: Add web-based postcard maker, including WASM
- :construction: Support WASM as compilation target
- :children_crossing: Allow USDZ creation without OpenUSD
- :necktie: Allow XMP as component input
- Allow ignoring transparency

### Fix

- :technologist: Use v prefix for git tags
- Overwrite wasm_exec files if needed
- :green_heart: Update go.sum & README, to allow build to work
- :art: Fix lint issue with Github Action
- :green_heart: Copy correct wasm & service worker exec files
- :green_heart: Install TinyGo before build
- :green_heart: Install QTC binary before build
- :bug: Fix build bug, makes qtc available & generates after go mod
- **postoffice**: Show useful errors on failure
- **postoffice**: :bug: Detect & give reasonable error when input image too large
- **postoffice**: Simplify HTML output & fix flip none output
- **postoffice**: :bug: Allow decimals on lat/long input
- **postoffice**: Switch to local wasm_exec.js and sw.js
- **postoffice**: Fix .only-empty to only show without items
- Update TODO.md
- Move experimental marker to the right
- Move Checkboxes to the right side
- **postoffice**: Stop loading after downloading a file
- **postoffice**: Hide output when restarting form
- Use Upload Pages Artifact action
- Deploy static postoffice to Github Pages
- **postoffice**: :bug: Forgot to merge extra git chunk
- :bug: Prevent segfault extracting empty lat/long from XMP
- Swap back to WebP as default transparent/lossless format for web
- :bug: Fix WebP XMP padding byte issue
- :bug: Correct the CRC32 calculated for the iTXt PNG chunk
- Prevent segfault when no SentOn date
- Handle finding -only images as part of a component bundle
- Prevent empty fields from appearing in YAML
- :poop: Custom image decoder selection
- Prevent flip for single-sided cards
- :bug: Set the *right* HTML codes for annotations
- Correct HTML output for underline Annotation
- :bug: Correctly half the web decoder's FrontSize.PxHeight
- :bug: Correct TIDD width/height ordering on EXIF read
- **border-detector**: :bug: Fix incorrect scaling threshold value
- Align chunks with even offsets
- Improve transparency algorithm
- Allow ignoring transparency and removing border
- Work around Goreleaser brew.ids issue
- Allow different binary counts
- Ensure binaries have the same count
- Remove unneeded rm command in build
- :construction_worker: Ensure the build can complete, even with WASM in the www/postoffice dir
- Update TODO & force a build
- 👷‍♂️ Correct order of builds in Github Actions
- 👷‍♂️ Fix "Unknown flag --id"
- :construction_worker: Don;t push service worker to homebrew
- Add inputs for all postcard metadata
- :white_check_mark: Add YAML tests and fix discovered issue
- :adhesive_bandage: Collect the physical & pixel size from XMP
- :construction_worker: Bump github actions & ship to correct tap dir
- :construction_worker: Pass correct token to GoReleaser Homebrew
- :construction_worker: Fix Goreleaser workflow
- :memo: Updating TODO and triggering first build

### Refactor

- :zap: Swap templating library & compile WASM with TinyGo
- :art: Break border removal into own file

### Perf

- :arrow_up: Bump versions
- :arrow_up: Upgrade dependencies
