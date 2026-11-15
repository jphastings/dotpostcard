---
# dotpostcard-9oti
title: Cast postcard shadows from a light
status: completed
type: feature
priority: normal
created_at: 2026-11-15T08:18:44Z
updated_at: 2026-11-15T09:28:31Z
---

Today's postcard shadow is a grey blurred rectangle that ignores the card's shape and rotates in perspective like a second card. Nothing about it can be configured or animated.

## Todo
- [x] Animatable knobs via `@property`: lift, flip-lift, light angle and elevation, softness, colour
- [x] The card rises just enough mid-flip to stay clear of the page (`--flip-lift` is a multiple of that)
- [x] Left- and right-hand cards get a square box, so a container holds both sides (`--square-hand-flips`)
- [x] `flip-none` cards don't rise when clicked
- [x] The shadow is the card's projection along the light, blurred in page space
- [x] `flip-none` images fill their card, sit above their shadow, and rest at the same height as other cards
- [x] Document the knobs in `docs/formats/css.md`
- [x] Tune the default values by eye in a playground
