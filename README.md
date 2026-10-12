# cliamp-plugin-dc-rpc

Shows what you are listening to in [cliamp](https://cliamp.stream) as a Discord Rich Presence card.

It has two parts:

- **Lua plugin** `dc-rpc.lua`: publishes playback state (track, artist, album, position, cover URL) over cliamp's IPC socket.
- **Daemon** `cliamp-rpcd`: subscribes to that state and updates your local Discord client. It exits on its own if cliamp stops responding.

## Requirements

- cliamp with plugin support.
- Discord desktop app running on the same machine.
- Go 1.22 or newer, to build the daemon. There are no prebuilt binaries yet.

## Installation

1. Install the plugin and trust it:

   ```sh
   cliamp plugins install 4irF1ux/cliamp-plugin-dc-rpc
   ```

2. Build the daemon and put it in a folder on your `PATH`:

   ```sh
   git clone https://github.com/4irF1ux/cliamp-plugin-dc-rpc
   cd cliamp-plugin-dc-rpc
   go build -o cliamp-rpcd ./cmd/cliamp-rpcd
   ```

   On Windows the binary is `cliamp-rpcd.exe`.

3. Start the daemon. It is not started by the plugin yet, so run it by hand:

   ```sh
   cliamp-rpcd
   ```

4. Restart cliamp and play something. The card appears in Discord.

## Configuration

Optional. The defaults are shown below.

```toml
[plugins.dc-rpc]
enabled = true      # false turns the plugin off and removes the card
show_stream = true  # false hides title and artist for streams (radio, etc.)
```

`enabled` and `show_stream` are read when the plugin loads. Restart cliamp after changing them.

### Daemon flags

| Flag | Default | Purpose |
| --- | --- | --- |
| `-client-id` | project application ID | Discord application used for the card. Override only if you use your own Discord app. |
| `-sock` | cliamp's config directory, `cliamp.sock` | Path to cliamp's IPC socket. Set it if cliamp uses a non-default config directory. |

The daemon finds the socket from `CLIAMP_CONFIG_DIR`, then `XDG_CONFIG_HOME`, then `~/.config/cliamp`, then `%APPDATA%\cliamp` on Windows.

## What the card shows

- **Playing a local file or stream with `show_stream = true`:** title, artist, album, and elapsed time.
- **Cover art:** the public `https` cover URL of the track. Local embedded art is not sent, because the Discord card cannot load files from your machine.
- **No cover URL:** the `cliamp` image asset of the Discord application.
- **Stream with `show_stream = false`:** no card.
- **Paused or stopped:** the card is cleared.

## Uninstall

```sh
cliamp plugins remove dc-rpc
```

This removes the plugin only. Stop the daemon and delete `cliamp-rpcd` yourself.

## How the daemon behaves

- Connects to cliamp's socket and subscribes to playback state.
- Updates are debounced: the last state is sent after 1 second without new changes.
- If cliamp stops responding for more than a minute, the daemon clears the card and exits.
- If Discord is closed or restarts, the daemon retries every 5 seconds.

## Notes

- The default cover fallback is the `cliamp` asset of the project's Discord application. If you use your own application (your own `-client-id`), upload an image named `cliamp` to it, or the card shows no image.
- Discord caches images. After changing an asset, the new image can take a while to appear.

## Limitations

- Only Windows has been tested. Linux and macOS have not.
- Embedded cover art in local files is not shown; only public `https` URLs are.
- The daemon talks to Discord over the local IPC pipe; the connection has no timeout, so a hung Discord client can block it until it is restarted.
- Cost measured on Windows with one run per scenario: about +0.5 percentage points of CPU while a track plays, which may be noise. The daemon itself stayed near 0% CPU and about 12 MB RAM.
