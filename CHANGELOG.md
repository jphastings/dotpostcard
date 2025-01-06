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
