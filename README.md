# cliamp-plugin-dc-rpc

Shows what you are listening to in [cliamp](https://cliamp.stream) as a Discord Rich Presence card.

It has two parts:

- **Lua plugin** `dc-rpc.lua`: publishes playback state (track, artist, album, position, cover URL) over cliamp's IPC socket, and starts the daemon.
- **Daemon** `cliamp-rpcd`: subscribes to that state and updates your local Discord client. It runs only while cliamp runs: cliamp stops it on quit, and it exits by itself a minute after cliamp stops answering.

## Requirements

- cliamp with plugin support.
- Discord desktop app running on the same machine.
- Go 1.22 or newer, to build the daemon. There are no prebuilt binaries yet.

## Installation

1. Copy `dc-rpc.lua` into cliamp's `plugins` folder and trust it:

   ```sh
   cliamp plugins trust dc-rpc
   ```

2. Build the daemon and put it in a folder on your `PATH`:

   ```sh
   git clone https://github.com/4irF1ux/cliamp-plugin-dc-rpc
   cd cliamp-plugin-dc-rpc
   go build -o cliamp-rpcd ./cmd/cliamp-rpcd
   ```

   On Windows the binary is `cliamp-rpcd.exe`.

3. Allow cliamp to run the daemon. Add it to `allowed_binaries` under `[plugins]` in cliamp's `config.toml`. The default list is kept; this adds to it:

   ```toml
   [plugins]
   allowed_binaries = "cliamp-rpcd"
   ```

4. Restart cliamp and play something. The daemon starts with cliamp and the card appears in Discord.

If the daemon cannot start, cliamp's log says why (usually step 2 or 3 is missing).

## Configuration

Optional. The defaults are shown below.

```toml
[plugins.dc-rpc]
enabled = true      # false turns the plugin off, stops the daemon and removes the card
show_stream = true  # false hides title and artist for streams (radio, etc.)
```

`enabled` and `show_stream` are read when the plugin loads. Restart cliamp after changing them.

### Daemon flags

The plugin starts the daemon with no flags, except when `CLIAMP_CONFIG_DIR` is set: then it passes `-sock` pointing at `cliamp.sock` in that directory.

| Flag | Default | Purpose |
| --- | --- | --- |
| `-client-id` | project application ID | Discord application used for the card. Override only if you use your own Discord app. |
| `-sock` | cliamp's config directory, `cliamp.sock` | Path to cliamp's IPC socket. |

Without `CLIAMP_CONFIG_DIR`, the daemon looks for the socket in `$XDG_CONFIG_HOME/cliamp`, `~/.config/cliamp`, or `%APPDATA%\cliamp` on Windows, in that order.

## What the card shows

- **Playing a local file or stream with `show_stream = true`:** title, artist, album, and elapsed time.
- **Cover art:** the public `https` cover URL of the track. Local embedded art is not sent, because Discord cannot load files from your machine.
- **No cover URL:** the `cliamp` image asset of the Discord application.
- **Stream with `show_stream = false`:** no card.
- **Paused or stopped:** the card is cleared.

## Uninstall

```sh
cliamp plugins remove dc-rpc
```

This removes the plugin. Remove `cliamp-rpcd` from your `PATH` and the `allowed_binaries` entry yourself.

## How the daemon behaves

- Connects to cliamp's socket and subscribes to playback state.
- Updates are debounced: the last state is sent after 1 second without new changes.
- If cliamp stops responding for more than a minute, the daemon clears the card and exits.
- If Discord is closed or restarts, the daemon retries every 5 seconds.
- cliamp stops each daemon process after 30 minutes (its exec timeout). The plugin starts a new one 5 seconds later.

## Notes

- The default cover fallback is the `cliamp` asset of the project's Discord application. If you use your own application (your own `-client-id`), upload an image named `cliamp` to it, or the card shows no image.
- Discord caches images. After changing an asset, the new image can take a while to appear.

## Limitations

- Only Windows has been tested. Linux and macOS have not.
- Embedded cover art in local files is not shown; only public `https` URLs are.
- The daemon talks to Discord over the local IPC pipe; the connection has no timeout, so a hung Discord client can block it until it is restarted.
- Cost measured on Windows with one run per scenario: about +0.5 percentage points of CPU while a track plays, which may be noise. The daemon itself stayed near 0% CPU and about 12 MB RAM.
