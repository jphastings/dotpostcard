---
# dotpostcard-dnt1
title: Test and tune transcription of handwritten messages
status: todo
type: feature
created_at: 2026-11-11T09:20:24Z
updated_at: 2026-11-11T09:20:24Z
---

`fastfoto-postcards --llm` already asks for a transcription of any message the sender added after the card was made (handwritten or typed), and writes it to `front.transcription` / `back.transcription` in the metadata. None of the cards it was tested on had a message, so transcription is untested.

Build a small eval of real cards with messages: varied handwriting, faint pencil, messages in other languages, text running around the edges or over the picture. Judge how closely the transcriptions match.

Things to find out:
- Whether 1200 px per side, the size used today, is enough for small or faint handwriting. If it isn't, the back could be sent larger than the front, or a second, closer look could be taken at message areas.
- Whether the "only text the sender added" rule holds: printed captions, copyright lines and postmarks must stay out of the transcription.
- Whether `transcription` annotations, which today hold only the plain text, are worth filling in, e.g. line breaks or illegible words.

## Todo
- [ ] Collect and scan a set of cards with messages
- [ ] Run them through --llm and judge the transcriptions
- [ ] Adjust image size or prompt if needed
