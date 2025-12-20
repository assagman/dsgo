# internal/ids

Internal ID helpers (not part of the public API).

## Overview

- `NewUUID` generates a UUIDv4 string using `crypto/rand`.
- `NewShortID` generates an 8-character hex ID.

## Notes

- Both helpers panic if random data cannot be read.
- Use only within DSGo internals.
