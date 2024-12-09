# TODO

- [ ] Only one postcards.css per group/directory
- [x] Figure out USD format
  - [x] Add colour to postcard edges
  - [x] Calendar flip cards
  - [x] No flip cards
  - [ ] Four point edges
  - [ ] Figure out how to do multi-edge vertexes
  - NB. Currently back of card is front of card in terms of points (esp. relevant for Calendar flip); not flip of card
- [ ] Hold file size & other info? `fs.Info{}` from `.Stat()`
- [ ] Look at using [tinyUSDZ](https://github.com/lighttransport/tinyusdz) to create USDZ files directly & without all the manual fussing
  - NB. This USDZ/USDC writer seems to be incomplete at the moment
- [ ] Get this CLI tool building automatically
- [ ] Creating a USD(Z) from an image that doesn't have resolution data (eg front/back portrait fixtures) seems to nil pointer fail. #bug
- [ ] Decode USD & USDZ #usd
- [ ] Read XMP data from jpg, png `web` format

## Done

- [x] Auto-transparency
- [x] Add thickness & paper colour to Metadata
- [x] Align YAML & JSON formats
  - [x] YAML output for front-dimensions
- [x] Force `-only` cards to be FlipNone
- [x] Swap to Annotations for locales (to allow XMP to be HTML-free)
- [x] Secret areas
- [x] Paper edge colour #usd
- [x] Move to using JPEGli for smaller filesizes
- [x] Add XMP to web JPG & PNG output #xmp
- [x] XMP decoder #xmp
- [x] Throw error on invalid flip
- [x] Compile without CGO
- [x] Read XMP data from WebP `web` format
