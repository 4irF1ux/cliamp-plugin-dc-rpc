local p = plugin.register({
    name = "discord-rpc",
    type = "hook",
    version = "0.1.0",
    description = "Publishes playback snapshots on cliamp.sock for the Discord RPC daemon",
})

-- p:config returns strings or nil. Defaults: enabled on, show_stream on.
local enabled = p:config("enabled") ~= "false"
local show_stream = p:config("show_stream") ~= "false"

local last_key = nil   -- identity of the last published snapshot, position excluded
local last_snap = nil -- most recent snapshot, kept so seeks can republish with a new position
local force = false    -- set by track.change so a repeated track still republishes

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
    emit({ status = "stopped" })
end)
