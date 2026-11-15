---
# dotpostcard-mind
title: Annotate transcriptions and choose the card's locale
status: completed
type: feature
created_at: 2026-11-15T08:29:42Z
updated_at: 2026-11-15T08:29:42Z
---

Transcriptions can carry annotations (`em`, `strong`, `underline`, `locale`), but `--llm` only ever filled in the plain text. Clearly underlined, bold or italic writing, and writing in a language other than the card's, should be annotated.

Models can't count UTF-8 bytes reliably (emoji and accented letters are several bytes each), so the model doesn't give offsets: it gives each transcription as runs of text with their styles and language, and Go joins the runs and works out the byte offsets.

`fastfoto-postcards --locale en-GB` fixes the card's locale rather than letting the model pick the card's main language: the descriptions are then written in that language even when the card is written in another, and the message is annotated as being in its own language.

## Todo
- [x] Ask for transcriptions as styled runs, and turn them into annotated text with correct byte offsets
- [x] Write descriptions in the card's locale
- [x] Add `--locale` to fastfoto-postcards
