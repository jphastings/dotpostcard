---
# dotpostcard-1a7m
title: Write a new collection in place, shown as in progress
status: completed
type: feature
priority: normal
created_at: 2026-11-14T20:54:03Z
updated_at: 2026-11-15T04:54:27Z
---

`postcards collection create` builds the collection in a hidden folder and renames it into place at the end, so nothing appears at the final name during a long run.

- A new file is written directly at its final name, and removed if the run fails or is cancelled.
- `--overwrite` of an existing file still builds aside and renames, so the old file survives a failed run.
- On macOS the file is marked as in progress while it's built, so Finder greys it out with a progress bar. Other systems have no file metadata for this, so they get no marker.
