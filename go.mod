module github.com/jphastings/dotpostcard

go 1.22

toolchain go1.23.2

require (
	git.sr.ht/~sbinet/gg v0.6.0
	github.com/chai2010/webp v1.1.1
	github.com/charmbracelet/glamour v0.8.0
	github.com/dsoprea/go-exif/v3 v3.0.1
	github.com/dsoprea/go-jpeg-image-structure v0.0.0-20221012074422-4f3f7e934102
	github.com/dsoprea/go-png-image-structure v0.0.0-20210512210324-29b889a6093d
	github.com/dsoprea/go-tiff-image-structure v0.0.0-20221003165014-8ecc4f52edca
	github.com/ernyoke/imger v1.0.0
	github.com/gen2brain/jpegli v0.3.3
	github.com/spf13/cobra v1.8.1
	github.com/stretchr/testify v1.9.0
	github.com/sunshineplan/tiff v0.0.0-20220128141034-29b9d69bd906
	github.com/trimmer-io/go-xmp v1.0.0
	golang.org/x/image v0.22.0
	gopkg.in/yaml.v3 v3.0.1
)

// Use a more recent version of the chai2010/webp library that I've vetted
replace github.com/chai2010/webp => github.com/chirino/webp v0.0.0-20240906184250-8b3bed1ecc92

require (
	github.com/alecthomas/chroma/v2 v2.14.0 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/campoy/embedmd v1.0.0 // indirect
	github.com/charmbracelet/lipgloss v0.12.1 // indirect
	github.com/charmbracelet/x/ansi v0.1.4 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dlclark/regexp2 v1.11.0 // indirect
	github.com/dsoprea/go-exif/v2 v2.0.0-20230826092837-6579e82b732d // indirect
	github.com/dsoprea/go-iptc v0.0.0-20200610044640-bc9ca208b413 // indirect
	github.com/dsoprea/go-logging v0.0.0-20200710184922-b02d349568dd // indirect
	github.com/dsoprea/go-photoshop-info-format v0.0.0-20200610045659-121dd752914d // indirect
	github.com/dsoprea/go-utility v0.0.0-20221003172846-a3e1774ef349 // indirect
	github.com/dsoprea/go-utility/v2 v2.0.0-20221003172846-a3e1774ef349 // indirect
	github.com/go-errors/errors v1.5.1 // indirect
	github.com/go-xmlfmt/xmlfmt v1.1.3 // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/golang/geo v0.0.0-20230421003525-6adc56603217 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/hhrutter/lzw v1.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/microcosm-cc/bluemonday v1.0.27 // indirect
	github.com/muesli/reflow v0.3.0 // indirect
	github.com/muesli/termenv v0.15.3-0.20240618155329-98d742f6907a // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/tetratelabs/wazero v1.8.1 // indirect
	github.com/yuin/goldmark v1.7.4 // indirect
	github.com/yuin/goldmark-emoji v1.0.3 // indirect
	golang.org/x/net v0.31.0 // indirect
	golang.org/x/sys v0.27.0 // indirect
	golang.org/x/term v0.26.0 // indirect
	golang.org/x/text v0.20.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
