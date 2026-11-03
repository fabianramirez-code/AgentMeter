// AgentMeter desktop widget for Übersicht: the same cards as the Rainmeter
// skin, fed by the shared ~/.agentmeter/run.sh (unix/install.sh puts it
// there). ▲ / ▼ cycle the enabled providers; the choice is remembered.
//
// Position, size and opacity: edit POSITION, SCALE and WINDOW_ALPHA below.
// Providers: put e.g. "-providers claude,codex" in ~/.agentmeter/flags.

export const command = `"$HOME/.agentmeter/run.sh"`;
export const refreshFrequency = 60 * 1000;

const POSITION = { top: 40, right: 40 };
const SCALE = 1.0;
const WINDOW_ALPHA = 0.9;

const PROVIDERS = [
	{ id: "claude", name: "Claude", color: "rgb(232,102,74)" },
	{ id: "codex", name: "Codex", color: "rgb(46,191,165)" },
	{ id: "gemini", name: "Gemini", color: "rgb(74,144,226)" },
];
// Usage traffic light, thresholds inclusive (same as WarnPct/CritPct in the skin).
const WARN_PCT = 60;
const CRIT_PCT = 85;
const COLOR_OK = "rgb(205,205,212)";
const COLOR_WARN = "rgb(242,201,76)";
const COLOR_CRIT = "rgb(232,80,70)";
// Card dot: green while the provider CLI process is running, gray otherwise.
const COLOR_ACTIVE = "rgb(76,201,120)";
const COLOR_IDLE = "rgb(110,110,118)";

const STORAGE_KEY = "agentmeter.activeProvider";

export const className = `
	top: ${POSITION.top}px;
	right: ${POSITION.right}px;
	zoom: ${SCALE};
	font-family: -apple-system, "SF Pro Text", sans-serif;
	-webkit-font-smoothing: antialiased;
`;

export const initialState = { data: null, error: "", active: loadActive() };

export const updateState = (event, prev) => {
	switch (event.type) {
		case "UB/COMMAND_RAN":
			if (event.error) return { ...prev, error: String(event.error) };
			try {
				return { ...prev, data: JSON.parse(event.output), error: "" };
			} catch (e) {
				return { ...prev, error: "data.json is not valid JSON" };
			}
		case "SELECT":
			saveActive(event.active);
			return { ...prev, active: event.active };
		default:
			return prev;
	}
};

export const render = ({ data, error, active }, dispatch) => {
	const enabled = data ? PROVIDERS.filter((p) => data[`${p.id}_status`] && data[`${p.id}_status`] !== "off") : [];
	const index = Math.max(0, enabled.findIndex((p) => p.id === active));
	const provider = enabled[index];
	const cycle = (step) => {
		if (enabled.length < 2) return;
		const next = enabled[(index + step + enabled.length) % enabled.length];
		dispatch({ type: "SELECT", active: next.id });
	};

	return (
		<div style={styles.panel}>
			<div style={styles.header}>
				<span>
					AgentMeter / <span style={{ color: provider ? provider.color : COLOR_OK }}>{provider ? provider.name : "—"}</span>
				</span>
				<span>
					<span style={styles.nav} onClick={() => cycle(-1)}>▲</span>
					<span style={styles.nav} onClick={() => cycle(1)}>▼</span>
				</span>
			</div>
			<div style={styles.separator} />
			{error ? (
				<Message text={error} />
			) : !data ? (
				<Message text="Loading…" />
			) : !provider ? (
				<Message text="No provider enabled" />
			) : (
				<Cards data={data} id={provider.id} />
			)}
		</div>
	);
};

const Cards = ({ data, id }) => {
	const cards = ["c1", "c2", "c3"].filter((c) => data[`${id}_${c}_label`]);
	if (cards.length === 0) return <Message text={data[`${id}_msg`] || data[`${id}_status`]} />;
	const running = data[`${id}_active`] === "1";
	return cards.map((c) => (
		<Card
			key={c}
			label={data[`${id}_${c}_label`]}
			pct={Number(data[`${id}_${c}_pct`]) || 0}
			reset={data[`${id}_${c}_reset`]}
			running={running}
		/>
	));
};

const Card = ({ label, pct, reset, running }) => {
	const color = pct >= CRIT_PCT ? COLOR_CRIT : pct >= WARN_PCT ? COLOR_WARN : COLOR_OK;
	const dot = running ? COLOR_ACTIVE : COLOR_IDLE;
	return (
		<div style={{ ...styles.card, borderColor: color }}>
			<div style={styles.cardTop}>
				<span style={styles.label}>{label}</span>
				<span style={{ ...styles.dot, background: dot, boxShadow: `0 0 6px ${dot}` }} />
			</div>
			<div style={{ ...styles.pct, color }}>{pct}%</div>
			<div style={styles.track}>
				<div style={{ ...styles.fill, width: `${Math.min(Math.max(pct, 0), 100)}%`, background: color }} />
			</div>
			<div style={styles.reset}>{reset}</div>
		</div>
	);
};

const Message = ({ text }) => <div style={{ ...styles.card, ...styles.reset }}>{text}</div>;

function loadActive() {
	try {
		return localStorage.getItem(STORAGE_KEY) || "claude";
	} catch (e) {
		return "claude";
	}
}

function saveActive(id) {
	try {
		localStorage.setItem(STORAGE_KEY, id);
	} catch (e) {
		// not persisted; the widget still works
	}
}

const styles = {
	panel: {
		width: 320,
		boxSizing: "border-box",
		padding: "10px 16px 14px",
		borderRadius: 14,
		background: `rgba(22,22,27,${WINDOW_ALPHA})`,
		border: `1px solid rgba(40,40,46,${WINDOW_ALPHA})`,
		color: "rgb(190,190,196)",
		WebkitBackdropFilter: "blur(20px)",
	},
	header: { display: "flex", justifyContent: "space-between", fontSize: 15, fontWeight: 700 },
	nav: { color: "rgb(170,170,178)", marginLeft: 10, cursor: "pointer", userSelect: "none" },
	separator: { height: 1, background: "rgb(40,40,46)", margin: "8px 0 4px" },
	card: {
		marginTop: 8,
		padding: "8px 14px 10px",
		borderRadius: 10,
		border: "1.5px solid rgb(40,40,46)",
		background: `rgba(26,26,32,${WINDOW_ALPHA})`,
	},
	cardTop: { display: "flex", justifyContent: "space-between", alignItems: "center" },
	label: { fontSize: 13, color: "rgb(180,180,188)" },
	dot: { width: 8, height: 8, borderRadius: "50%" },
	pct: { fontSize: 28, fontWeight: 700, margin: "2px 0 4px" },
	track: { height: 6, borderRadius: 3, background: "rgb(45,45,52)", overflow: "hidden" },
	fill: { height: "100%", borderRadius: 3 },
	reset: { fontSize: 12, color: "rgb(150,150,158)", marginTop: 6 },
};
