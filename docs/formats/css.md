# CSS

This output format produces the same `postcards.css` file every time, which can be used in conjunction with the [html](html.md) format to display [web.md] format postcards beautifully on the web.

> [!WARNING]
> The CSS is modern, and makes use of [nesting](https://caniuse.com/css-nesting) and [`@property`](https://caniuse.com/mdn-css_at-rules_property), so you may need to do some extra work if your target audience uses older browsers.

## Customising the lift and shadow

Each postcard casts its shadow from a light onto the page. These custom properties control it. Set them on `.postcard`, or on any element containing postcards. All of them can be transitioned or animated.

| Property | What it controls |
|---|---|
| `--lift` | How far the card rests above the page |
| `--flip-lift` | How high the card rises halfway through a flip, as a multiple of the rise that keeps it clear of the page: `1` (the minimum) just clears it |
| `--light-angle` | Where the light comes from: `0deg` is the top, clockwise, as in `linear-gradient` |
| `--light-elevation` | How high the light is: `90deg` is straight above |
| `--shadow-softness` | How much the shadow blurs per pixel of height |
| `--shadow-color` | The shadow's colour and strength |
| `--square-hand-flips` | `1` (default) gives left- and right-hand cards a square box, because flipping turns them a quarter-turn on the page; `0` sizes the box to the front, so the flipped card overflows it |

The defaults are the `@property` rules at the top of `postcards.css`.

```css
/* Postcards that hover high under a light to their right, rising further when hovered */
body { --lift: 24px; --light-angle: 90deg; }
.postcard { transition: --flip 1s ease-in-out, --lift 0.3s; }
.postcard:hover { --lift: 48px; }
```

`postcards.css` animates flipping with `transition: --flip 1s ease-in-out`. If you give `.postcard` a `transition` of your own, include `--flip` in it, or the cards will snap over instead of turning.

## Example

The following three output formats will create an HTML file that uses the CSS file to show the postcard in a visually appealing way.

```sh
$ postcards -f css,html,web pyramids-front.jpg
⚙︎ Converting 1 postcard into 3 different formats…
pyramids-front.jpg (Component files) → (CSS) postcards.css
pyramids-front.jpg (Component files) → (HTML) pyramids.html
pyramids-front.jpg (Component files) → (Web) pyramids.postcard.jpg
```
