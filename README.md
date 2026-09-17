# Enter Scheduler

A small native 64-bit Windows utility that schedules a single **Enter** keystroke for a selected top-level window.

The application was originally built on 6 August 2026 as `Enter_Scheduler_3D.exe`. The source in this repository is a reconstructed, maintainable version based on the final executable's verified behaviour and recovered symbols/strings.

## Features

- Lists visible top-level windows and lets you lock one as the target.
- Selects hour and minute and schedules the next matching local time.
- Live countdown.
- `Δοκιμή Enter` sends an immediate test Enter to the selected window.
- Restores and activates the selected target before sending Enter.
- Verifies that the intended target actually became the foreground window; it does **not** send Enter elsewhere if activation is blocked.
- Detects if the target window closes while a schedule is active.
- Optional minimization after scheduling.
- Diagnostic crash log at `%TEMP%\Enter_Scheduler_error.txt`.
- Native Win32 GUI; no PowerShell or .NET runtime dependency.
- The recovered original application icon is included in `assets/enter_scheduler.ico` and embedded in normal builds.

## Build

Requirements: Windows x64 and Go 1.23+.

```powershell
./build.ps1
```

The script writes:

```text
dist/Enter_Scheduler.exe
```

The GitHub Actions workflow also builds the Windows x64 executable on every push and pull request and publishes it as an Actions artifact.

## Project history

The recovered original executable reports Go `1.23.2`, `GOOS=windows`, `GOARCH=amd64`, `CGO_ENABLED=0`, and build path `_/tmp/enter_scheduler_v5`. Recovered function names include `refreshWindowList`, `activateTarget`, `sendKeyboardEnter`, `executeEnter`, `startSchedule`, `cancelSchedule`, and `updateCountdown`.

SHA-256 of the recovered original `Enter_Scheduler_3D.exe`:

```text
c4c45f8ee89aad0bf374266b0ba72607705dbc3aef3dcde1e14eba077ab824c7
```

The historical executable and screenshot are retained separately in the recovery package used to reconstruct this repository; neither is required for building from source.

## License

MIT.
