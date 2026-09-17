# Enter Scheduler

A small native 64-bit Windows utility that schedules a keyboard sequence for a selected top-level window.

The application was originally built on 6 August 2026 as `Enter_Scheduler_3D.exe`. The source in this repository is a reconstructed, maintainable version based on the final executable's verified behaviour and recovered symbols/strings.

## Current sequence

At the selected first-execution time, Enter Scheduler can now run this sequence:

1. Wait for a configurable **pre-text delay** in milliseconds.
2. Activate and verify the selected target window.
3. Type the configured **Unicode text** (the text may also be empty).
4. Wait for a configurable **text-to-Enter delay** in milliseconds.
5. Re-activate and verify the same target window.
6. Send **Enter**.
7. Optionally repeat the whole sequence after a configurable loop interval.

The loop interval has separate fields for **hours, minutes, seconds, and milliseconds**. When all four loop fields are zero, the sequence runs once. When a loop is configured, the interval is counted after a completed Enter before the next sequence begins, so executions never overlap.

`Δοκιμή ακολουθίας` executes the complete delay → text → delay → Enter sequence immediately and exactly once, ignoring the loop setting.

## Safety behaviour

- Lists visible top-level windows and lets you lock one as the target.
- Restores and activates the selected target before typing and again before Enter.
- Verifies that the intended target actually became the foreground window.
- If activation fails, it does **not** type text or send Enter elsewhere.
- Detects if the target window closes while a schedule or loop is active.
- Locks configuration controls while a run is active, so one loop keeps a stable configuration.
- `Ακύρωση` stops the active schedule/loop.
- Optional minimization after scheduling.
- Diagnostic crash log at `%TEMP%\Enter_Scheduler_error.txt`.

Milliseconds are accepted by the UI and deadlines are tracked at millisecond resolution. Actual delivery timing is still subject to normal Windows scheduling/timer latency.

## Build

Requirements: Windows x64 and Go 1.23+.

```powershell
./build.ps1
```

The distribution is written to:

```text
dist/
  Enter_Scheduler.exe
  assets/
    enter_scheduler.ico
```

The program loads the recovered icon from the adjacent `assets` folder at runtime. If that file is absent, the program still runs using the standard Windows application icon.

The GitHub Actions workflow builds the Windows x64 distribution on every push and pull request and publishes it as an Actions artifact.

## Project history

The recovered original executable reports Go `1.23.2`, `GOOS=windows`, `GOARCH=amd64`, `CGO_ENABLED=0`, and build path `_/tmp/enter_scheduler_v5`. Recovered function names include `refreshWindowList`, `activateTarget`, `sendKeyboardEnter`, `executeEnter`, `startSchedule`, `cancelSchedule`, and `updateCountdown`.

SHA-256 of the recovered original `Enter_Scheduler_3D.exe`:

```text
c4c45f8ee89aad0bf374266b0ba72607705dbc3aef3dcde1e14eba077ab824c7
```

## License

MIT.
