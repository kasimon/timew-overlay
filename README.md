# timew-overlay

A small desktop overlay for [Timewarrior](https://timewarrior.net/) that shows
the current tracking status, day total, and provides controls to start/stop
the clock.

## Features

- Live tracking status with play/stop toggle button
- Current tag and elapsed time display
- Day total summary
- Tag entry field — type a tag and press Enter to start a new tagged timeblock
- Window title shows day total and active/stopped state
- Refreshes every second

## Requirements

- [Timewarrior](https://timewarrior.net/) (`timew`) installed and on `$PATH`
- [Nix](https://nixos.org/) (for build dependencies) or Go + C compiler + X11/GL dev libraries

## Building

With Nix:

```sh
nix-shell --run "go build -o timew-overlay ."
```

## Running

```sh
nix-shell --run ./timew-overlay
```

Or after building, if all runtime libraries are available:

```sh
./timew-overlay
```
