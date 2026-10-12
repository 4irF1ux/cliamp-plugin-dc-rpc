local p = plugin.register({
    name = "dc-rpc",
    type = "hook",
    version = "0.1.0",
    description = "Publishes playback snapshots on cliamp.sock and runs the Discord RPC daemon",
    permissions = { "exec" },
})

-- p:config returns strings or nil. Defaults: enabled on, show_stream on.
local enabled = p:config("enabled") ~= "false"
local show_stream = p:config("show_stream") ~= "false"

local last_key = nil   -- identity of the last published snapshot, position excluded
local last_snap = nil -- most recent snapshot, kept so seeks can republish with a new position
local force = false    -- set by track.change so a repeated track still republishes

-- The daemon lives only while this plugin does: cliamp.exec kills it when cliamp
-- quits, and it exits by itself a minute after cliamp stops answering.
local DAEMON = "cliamp-rpcd"
local RELAUNCH_DELAY = 5 -- seconds before restarting a daemon that exited
local quitting = false

-- cliamp's socket lives in its config dir. The daemon runs with a minimal
-- environment that sets HOME, so it would pick ~/.config/cliamp on Windows,
-- where cliamp does not listen. Compute the dir the way cliamp does (same
-- order as cliamp's appdir) and always pass the socket path explicitly.
local function join_path(base, name)
    local last = base:sub(-1)
    if last == "\\" or last == "/" then
        return base .. name
    end
    local sep = base:find("\\", 1, true) and "\\" or "/"
    return base .. sep .. name
end

local function config_dir()
    local dir = os.getenv("CLIAMP_CONFIG_DIR")
    if dir and dir ~= "" then return dir end
    local xdg = os.getenv("XDG_CONFIG_HOME")
    if xdg and xdg ~= "" then return join_path(xdg, "cliamp") end
    local home = os.getenv("HOME")
    if home and home ~= "" then return join_path(join_path(home, ".config"), "cliamp") end
    local appdata = os.getenv("APPDATA")
    if appdata and appdata ~= "" then return join_path(appdata, "cliamp") end
    local profile = os.getenv("USERPROFILE")
    if profile and profile ~= "" then return join_path(join_path(profile, ".config"), "cliamp") end
    return nil
end

local function daemon_args()
    local dir = config_dir()
    if dir == nil then
        return {}
    end
    local sock = join_path(dir, "cliamp.sock")
    cliamp.log.info("cliamp socket: " .. sock)
    return { "-sock", sock }
end

local launch
launch = function()
    if quitting then
        return
    end
    local id, err = cliamp.exec.run(DAEMON, daemon_args(), {
        on_exit = function(code)
            if not quitting then
                cliamp.log.warn("cliamp-rpcd exited (code " .. tostring(code) .. "), restarting")
                cliamp.timer.after(RELAUNCH_DELAY, launch)
            end
        end,
    })
    if id == nil then
        cliamp.log.error("cannot start cliamp-rpcd: " .. tostring(err) .. " (add cliamp-rpcd to [plugins] allowed_binaries and put it on PATH)")
    end
end

if enabled then
    launch()
end

local function emit(snap)
    if not enabled then
        snap = { status = "stopped" }
    end
    p:publish("playback", snap, { retain = true })
end

-- Discord can only fetch public https images. cliamp's embedded-art paths are
-- file:// URLs on this machine, so they never leave the plugin.
local function public_https(url)
    if type(url) == "string" and url:sub(1, 8) == "https://" then
        return url
    end
    return nil
end

local function on_state(e)
    local stream = e.stream == true
    local hide = stream and not show_stream
    local snap = {
        status = e.status,
        title = (not hide) and e.title or nil,
        artist = (not hide) and e.artist or nil,
        album = (not hide) and e.album or nil,
        image = (not hide) and public_https(e.album_art_url) or nil,
        duration = e.duration,
        position = e.position,
    }
    last_snap = snap

    local key = table.concat({
        tostring(enabled),
        tostring(snap.status),
        tostring(snap.title),
        tostring(snap.artist),
        tostring(snap.album),
        tostring(snap.duration),
        tostring(snap.image),
    }, "\0")

    if force or key ~= last_key then
        last_key = key
        force = false
        emit(snap)
    end
end

p:on("playback.state", on_state)

p:on("track.change", function()
    force = true
end)

p:on("player.seek", function(e)
    if last_snap == nil then
        return
    end
    last_snap.position = e.position
    emit(last_snap)
end)

p:on("app.quit", function()
    quitting = true
    emit({ status = "stopped" })
end)
