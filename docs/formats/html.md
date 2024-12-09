# HTML

Used in conjunction with the [css](css.md) and [web](web.md) formats, this output format creates a sample `{name}.html` file with an HTML fragment that's suitable to display the postcard being converted.

The HTML fragment produced has two sections separated by `\n\n` (the only `\n\n` in the file). The first of these sections can be discarded if you're injecting the HTML into an already established postcards page.

In the unlikely even that the CSS being produced by the [css](css.md) format changes in a backwards incompatible way (which will only happen after a breaking change release of this tool) then any HTML previously produced by this format may also need to be updated. Details on how to manually make the needed changes will be in the release notes for this tool.

## Example

The following three output formats will create an HTML file that makes use of the CSS & web files to show the postcard in a visually appealing way.

```sh
$ postcards -f html,css,web pyramids-front.jpg
⚙︎ Converting 1 postcard into 3 different formats…
pyramids-front.jpg (Component files) → (HTML) pyramids.html
pyramids-front.jpg (Component files) → (CSS) postcards.css
pyramids-front.jpg (Component files) → (Web) pyramids.postcard.jpg
```
