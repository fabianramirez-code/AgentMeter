# AgentMeter for Waybar

`unix/install.sh` does not edit your Waybar config (it is JSONC and often
hand-tuned). Add the module yourself:

`~/.config/waybar/config.jsonc`

```jsonc
"modules-right": ["custom/agentmeter", /* ... */],

"custom/agentmeter": {
    "exec": "$HOME/.agentmeter/run.sh waybar",
    "return-type": "json",
    "interval": 60,
    "format": "󰊚 {}",
    "tooltip": true,
    "on-click": "xdg-open $HOME/.agentmeter/agentmeter.log"
}
```

`~/.config/waybar/style.css` (classes follow the same traffic light as the
other front ends: below 60 % `ok`, from 60 % `warn`, from 85 % `crit`):

```css
#custom-agentmeter.warn  { color: #F2C94C; }
#custom-agentmeter.crit  { color: #E85046; }
#custom-agentmeter.error { color: #E85046; }
```

Then reload Waybar (`killall -SIGUSR2 waybar`).
