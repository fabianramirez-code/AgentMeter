-- AgentMeter skin control logic: provider cycling and the settings panel.
-- Kept in Lua (Rainmeter's own scripting plugin, not an external dependency)
-- because cycling through only the ENABLED providers and clamping the
-- transparency/size sliders both need small loops/conditionals that plain
-- Rainmeter bangs can't express cleanly.

local providers = { "Claude", "Codex", "Gemini" }
local providerVars = { "EnableClaude", "EnableCodex", "EnableGemini" }
local providerColorVars = { "ColorClaude", "ColorCodex", "ColorGemini" }

local function isEnabled(skin, idx)
    return tonumber(skin:GetVariable(providerVars[idx + 1])) == 1
end

-- The Card1/Card2 meters are generic: their MeasureName is
-- "Measure#ActiveProviderName#C1..." (DynamicVariables=1), so switching
-- providers is just setting ActiveProviderName/Color and re-evaluating
-- those meters -- no per-provider meter groups to show/hide.
-- ActiveIndex/Name are also written to the .ini so the chosen provider
-- survives the !Refresh that every settings change triggers.
local function showProvider(skin, idx)
    skin:Bang("!SetVariable ActiveIndex " .. idx)
    skin:Bang('!SetVariable ActiveProviderName "' .. providers[idx + 1] .. '"')
    skin:Bang('!SetVariable ActiveProviderColor "' .. skin:GetVariable(providerColorVars[idx + 1]) .. '"')
    skin:Bang("!WriteKeyValue Variables ActiveIndex " .. idx)
    skin:Bang('!WriteKeyValue Variables ActiveProviderName "' .. providers[idx + 1] .. '"')
    -- Switching providers starts the cards area back at the top. MeasureHasC3
    -- is updated synchronously so Card3/scrollbar visibility and MaxScroll
    -- match the new provider right away (ShowMeterGroup Cards re-shows them).
    skin:Bang("!SetVariable ScrollY 0")
    skin:Bang("!UpdateMeasure MeasureHasC3")
    skin:Bang("!UpdateMeterGroup Cards")
    skin:Bang("!Redraw")
end

-- direction: 1 scrolls down, -1 scrolls up. Step is 64 unscaled px, clamped
-- to [0, MaxScroll] (0 when the active provider has no third card).
function Scroll(direction)
    local skin = SKIN
    local maxScroll = tonumber(skin:GetVariable("MaxScroll")) or 0
    local cur = tonumber(skin:GetVariable("ScrollY")) or 0
    local newVal = cur + (direction * 64)
    if newVal < 0 then newVal = 0 end
    if newVal > maxScroll then newVal = maxScroll end
    skin:Bang("!SetVariable ScrollY " .. newVal)
    skin:Bang("!UpdateMeterGroup Cards")
    skin:Bang("!Redraw")
end

-- Clicking the scrollbar track/thumb jumps between top and bottom.
function ScrollToggle()
    local skin = SKIN
    local maxScroll = tonumber(skin:GetVariable("MaxScroll")) or 0
    local cur = tonumber(skin:GetVariable("ScrollY")) or 0
    local newVal = 0
    if cur < maxScroll then newVal = maxScroll end
    skin:Bang("!SetVariable ScrollY " .. newVal)
    skin:Bang("!UpdateMeterGroup Cards")
    skin:Bang("!Redraw")
end

local function buildEnabledList(skin)
    local list = {}
    for i = 0, 2 do
        if isEnabled(skin, i) then table.insert(list, providers[i + 1]:lower()) end
    end
    return table.concat(list, ",")
end

function Initialize()
    local skin = SKIN
    skin:Bang('!SetVariable EnabledProviders "' .. buildEnabledList(skin) .. '"')
    local idx = tonumber(skin:GetVariable("ActiveIndex")) or 0
    for _ = 1, 3 do
        if isEnabled(skin, idx) then break end
        idx = (idx + 1) % 3
    end
    showProvider(skin, idx)
    -- ShowSettings is persisted, so the panel stays open across the !Refresh
    -- done by every settings change until the user clicks Done.
    if tonumber(skin:GetVariable("ShowSettings")) == 1 then
        skin:Bang("!HideMeterGroup Cards")
        skin:Bang("!ShowMeterGroup Settings")
        skin:Bang("!Redraw")
    end
end

-- direction: 1 for next (up), -1 for previous (down)
function CycleProvider(direction)
    local skin = SKIN
    local idx = tonumber(skin:GetVariable("ActiveIndex")) or 0
    for _ = 1, 3 do
        idx = (idx + direction) % 3
        if idx < 0 then idx = idx + 3 end
        if isEnabled(skin, idx) then break end
    end
    showProvider(skin, idx)
end

function ToggleSettings()
    local skin = SKIN
    local cur = tonumber(skin:GetVariable("ShowSettings")) or 0
    local newVal = 1 - cur
    skin:Bang("!SetVariable ShowSettings " .. newVal)
    skin:Bang("!WriteKeyValue Variables ShowSettings " .. newVal)
    if newVal == 1 then
        skin:Bang("!HideMeterGroup Cards")
        skin:Bang("!ShowMeterGroup Settings")
    else
        skin:Bang("!HideMeterGroup Settings")
        skin:Bang("!ShowMeterGroup Cards")
        local idx = tonumber(skin:GetVariable("ActiveIndex")) or 0
        showProvider(skin, idx)
    end
    skin:Bang("!UpdateMeter *")
    skin:Bang("!Redraw")
end

-- varName: "EnableClaude" | "EnableCodex" | "EnableGemini"
function ToggleProvider(varName)
    local skin = SKIN
    local cur = tonumber(skin:GetVariable(varName)) or 0
    skin:Bang("!WriteKeyValue Variables " .. varName .. " " .. (1 - cur))
    skin:Bang("!Refresh")
end

-- Transparency is a 0-100 % user setting (0 = opaque, 100 = invisible
-- background); WindowAlpha (0-255) is derived from it for the color strings.
function ChangeTransparency(delta)
    local skin = SKIN
    local cur = tonumber(skin:GetVariable("Transparency")) or 10
    local newVal = math.max(0, math.min(100, cur + tonumber(delta)))
    local alpha = math.floor(255 * (100 - newVal) / 100 + 0.5)
    skin:Bang("!WriteKeyValue Variables Transparency " .. newVal)
    skin:Bang("!WriteKeyValue Variables WindowAlpha " .. alpha)
    skin:Bang("!Refresh")
end

function ChangeScale(delta)
    local skin = SKIN
    local cur = tonumber(skin:GetVariable("Scale")) or 1.0
    local newVal = cur + tonumber(delta)
    if newVal < 0.75 then newVal = 0.75 end
    if newVal > 2.0 then newVal = 2.0 end
    skin:Bang("!WriteKeyValue Variables Scale " .. string.format("%.2f", newVal))
    skin:Bang("!Refresh")
end
