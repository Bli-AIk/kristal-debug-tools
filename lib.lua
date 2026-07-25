local lib = {}

local LIB_ID = "kristal-debug-tools"

local function config(key)
    return Kristal.getLibConfig(LIB_ID, key)
end

local function get_arg(name)
    local args = Kristal and Kristal.Args and Kristal.Args[name]
    if type(args) ~= "table" then
        return nil, false
    end
    return args[1], true
end

local function get_first_arg(names)
    for _, name in ipairs(names) do
        local value, present = get_arg(name)
        if present then
            return value, true
        end
    end
    return nil, false
end

local function parse_wave_selector(value)
    local number = tonumber(value)
    if number then
        if number ~= math.floor(number) or number < 1 then
            return nil
        end
        return number
    end

    if type(value) == "string" and value ~= "" then
        return value
    end
end

local function parse_non_negative(value)
    local number = tonumber(value)
    if not number or number < 0 then
        return nil
    end
    return number
end

local function parse_mercy(value)
    local number = tonumber(value)
    if not number or number < 0 or number > 100 then
        return nil
    end
    return number
end

local function config_default_encounter()
    local encounter = config("default_encounter")
    if type(encounter) ~= "string" or encounter == "" then
        return nil
    end
    return encounter
end

local function is_enabled()
    if config("enabled") == false then
        return false
    end
    if config("only_dev") == false then
        return true
    end
    return not Kristal.isDevMode or Kristal.isDevMode()
end

local function option_key(name)
    if name == "wave-force" then
        return "wave_force"
    elseif name == "initial-tp" then
        return "initial_tp"
    elseif name == "initial-mercy" then
        return "initial_mercy"
    end
    return name
end

function lib:load_options()
    if self.options_loaded then
        return
    end
    self.options_loaded = true

    self.options = {
        enabled = is_enabled(),
        encounter = nil,
        encounter_requested = false,
        direct_entry = false,
        wave = nil,
        wave_force = nil,
        initial_tp = parse_non_negative(config("initial_tp")),
        initial_mercy = parse_mercy(config("initial_mercy")),
    }

    if not self.options.enabled then
        return
    end

    local encounter, has_encounter = get_arg("encounter")
    local wave, has_wave = get_arg("wave")
    local wave_force, has_wave_force = get_arg("wave-force")
    local initial_tp, has_initial_tp = get_first_arg({"tp", "initial-tp"})
    local initial_mercy, has_initial_mercy = get_first_arg({"mercy", "initial-mercy"})

    self.options.encounter_requested = has_encounter
    self.options.direct_entry = has_encounter or has_wave or has_wave_force
        or has_initial_tp or has_initial_mercy

    if has_encounter then
        self.options.encounter = encounter
        if not self.options.encounter or self.options.encounter == "" then
            self.options.encounter = config_default_encounter()
        end
    elseif self.options.direct_entry then
        self.options.encounter = config_default_encounter()
    end

    if has_wave then
        self.options.wave = parse_wave_selector(wave)
        if not self.options.wave then
            print("[kristal-debug-tools] Ignoring invalid --wave value: " .. tostring(wave))
        end
    end

    if has_wave_force then
        self.options.wave_force = parse_wave_selector(wave_force)
        if not self.options.wave_force then
            print("[kristal-debug-tools] Ignoring invalid --wave-force value: " .. tostring(wave_force))
        end
    end

    if has_initial_tp then
        local parsed = parse_non_negative(initial_tp)
        if parsed == nil then
            print("[kristal-debug-tools] Ignoring invalid --tp value: " .. tostring(initial_tp))
        else
            self.options.initial_tp = parsed
        end
    end

    if has_initial_mercy then
        local parsed = parse_mercy(initial_mercy)
        if parsed == nil then
            print("[kristal-debug-tools] Ignoring invalid --mercy value: " .. tostring(initial_mercy))
        else
            self.options.initial_mercy = parsed
        end
    end

    if self.options.direct_entry and not self.options.encounter then
        print("[kristal-debug-tools] Battle options require --encounter or config.kristal-debug-tools.default_encounter")
        self.options.direct_entry = false
    end
end

function lib:getOption(name)
    self:load_options()
    return self.options[option_key(name)]
end

function lib:getOptions()
    self:load_options()
    return self.options
end

function lib:isEnabled()
    self:load_options()
    return self.options.enabled == true
end

function lib:installModOptionHook()
    if self.mod_option_hooked or not self.options.enabled or not HookSystem or not Kristal then
        return
    end
    self.mod_option_hooked = true

    HookSystem.hook(Kristal, "getModOption", function(orig, key, ...)
        local options = self.options
        if options.direct_entry and options.encounter and key == "encounter" then
            return options.encounter
        end
        if options.direct_entry and options.encounter and key == "map" then
            return nil
        end
        return orig(key, ...)
    end)
end

local function resolve_wave(enemy, selector)
    if not enemy or not selector or type(enemy.waves) ~= "table" then
        return nil
    end

    if type(selector) == "number" then
        return enemy.waves[selector]
    end

    for _, wave in ipairs(enemy.waves) do
        if wave == selector then
            return wave
        end
        if type(wave) == "table" and wave.id == selector then
            return wave
        end
    end
end

function lib:warnMissingWave(selector)
    self.wave_warnings = self.wave_warnings or {}
    local key = type(selector) .. ":" .. tostring(selector)
    if self.wave_warnings[key] then
        return
    end
    self.wave_warnings[key] = true
    print("[kristal-debug-tools] No matching wave for selector: " .. tostring(selector))
end

function lib:installBattleHooks()
    if self.battle_hooks_installed or not self.options.enabled or not HookSystem then
        return
    end

    self.battle_hooks_installed = true
    self.applied_battles = setmetatable({}, {__mode = "k"})
    self.started_wave_enemies = setmetatable({}, {__mode = "k"})

    if Battle then
        HookSystem.hook(Battle, "onIntroState", function(orig, battle, ...)
            if not self.applied_battles[battle] then
                self.applied_battles[battle] = true

                if self.options.initial_tp ~= nil and Game and Game.setTension then
                    Game:setTension(self.options.initial_tp)
                end

                if self.options.initial_mercy ~= nil then
                    for _, enemy in ipairs(battle.enemies or {}) do
                        enemy.mercy = self.options.initial_mercy
                    end
                end
            end

            return orig(battle, ...)
        end)
    end

    if EnemyBattler then
        HookSystem.hook(EnemyBattler, "selectWave", function(orig, enemy, ...)
            local selector = self.options.wave_force
            local is_forced = selector ~= nil

            if not selector and not self.started_wave_enemies[enemy] then
                selector = self.options.wave
            end

            if selector then
                if not is_forced then
                    self.started_wave_enemies[enemy] = true
                end

                local wave = resolve_wave(enemy, selector)
                if wave then
                    enemy.selected_wave = wave
                    return wave
                end
                self:warnMissingWave(selector)
            end

            return orig(enemy, ...)
        end)
    end
end

function lib:init()
    self:load_options()
    if not self.options.enabled then
        return
    end
    self:installModOptionHook()
    self:installBattleHooks()
end

function lib:unload()
    self.options_loaded = false
    self.options = nil
    self.applied_battles = nil
    self.started_wave_enemies = nil
    self.wave_warnings = nil
end

function lib:cleanup()
    self:unload()
end

if Registry and Registry.registerGlobal then
    Registry.registerGlobal("KristalDebugTools", lib)
end

return lib
