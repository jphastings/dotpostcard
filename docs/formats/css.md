# CSS

This output format produces the same `postcards.css` file every time, which can be used in conjunction with the [html](html.md) format to display [web.md] format postcards beautifully on the web.

> [!WARNING]
> The CSS is modern, and makes use of [nesting](https://caniuse.com/css-nesting), so you may need to do some extra work if your target audience uses older browsers.

## Example

The following three output formats will create an HTML file that uses the CSS file to show the postcard in a visually appealing way.

```sh
$ postcards -f css,html,web pyramids-front.jpg
⚙︎ Converting 1 postcard into 3 different formats…
pyramids-front.jpg (Component files) → (CSS) postcards.css
pyramids-front.jpg (Component files) → (HTML) pyramids.html
pyramids-front.jpg (Component files) → (Web) pyramids.postcard.jpg
```
