"use strict";

/* kristal-debug-tools GUI frontend — vanilla JS, zero build. */

const I18N = {
  zh: {
    tasks: "任务", launch: "启动游戏", terminal: "输出", runs: "运行记录",
    run: "运行", refresh: "刷新", cancel: "取消",
    copyUrl: "复制地址", copied: "已复制",
    loading: "加载中…",
    fLang: "语言", fEncounter: "遭遇", fWave: "波次", fWaveForce: "强制波次",
    fTp: "初始 TP", fMercy: "初始宽容", fExtra: "额外参数（透传）",
    love: "love", engine: "引擎", mod: "模组", just: "just",
    loveMissing: "未找到 love（请安装 LÖVE 并加入 PATH）",
    noEngine: "未找到 Kristal 引擎",
    noMod: "未找到 mod",
    justNone: "不可用",
    justSystem: "系统", justEmbedded: "内置", justCache: "缓存",
    running: "运行中", exited: "退出码", started: "开始", duration: "耗时",
    empty: "（空）",
    termPlaceholder: "—",
    launchRunning: "正在启动游戏…", launchOk: "已启动，输出见右侧终端",
    taskFail: "运行失败", justUnavailable: "just 不可用，无法列出任务",
    noTasks: "没有可运行的任务",
  },
  en: {
    tasks: "TASKS", launch: "LAUNCH GAME", terminal: "OUTPUT", runs: "RUNS",
    run: "RUN", refresh: "REFRESH", cancel: "CANCEL",
    copyUrl: "COPY URL", copied: "COPIED",
    loading: "loading…",
    fLang: "LANGUAGE", fEncounter: "ENCOUNTER", fWave: "WAVE", fWaveForce: "WAVE FORCE",
    fTp: "TP", fMercy: "MERCY", fExtra: "EXTRA ARGS (PASSTHROUGH)",
    love: "love", engine: "engine", mod: "mod", just: "just",
    loveMissing: "love not found (install LÖVE and add it to PATH)",
    noEngine: "Kristal engine not found",
    noMod: "mod not found",
    justNone: "unavailable",
    justSystem: "system", justEmbedded: "embedded", justCache: "cache",
    running: "running", exited: "exit", started: "started", duration: "took",
    empty: "(empty)",
    termPlaceholder: "—",
    launchRunning: "launching…", launchOk: "launched — output on the right",
    taskFail: "run failed", justUnavailable: "just unavailable — no tasks to list",
    noTasks: "no runnable tasks",
  },
};

let lang = localStorage.getItem("kdt-lang") ||
  (navigator.language && navigator.language.toLowerCase().startsWith("zh") ? "zh" : "en");

const t = (key) => (I18N[lang] && I18N[lang][key]) || I18N.en[key] || key;

function applyI18n() {
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    el.textContent = t(el.dataset.i18n);
  });
  const el = document.getElementById("lang-toggle");
  if (el) el.textContent = lang === "zh" ? "EN" : "中文";
  document.title = "Kristal Debug Tools";
}

/* --- API helpers --- */

async function api(path, opts) {
  const r = await fetch(path, opts);
  let j = {};
  try { j = await r.json(); } catch (_) { /* non-JSON error body */ }
  if (!r.ok) throw new Error(j.error || r.statusText || ("HTTP " + r.status));
  return j;
}

const postJSON = (path, body) => api(path, {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body),
});

/* --- state --- */

let currentRun = null; // { id, es }
let statusData = null;

/* --- status bar --- */

async function loadStatus() {
  const bar = document.getElementById("statusbar");
  try {
    statusData = await api("/api/status");
  } catch (err) {
    bar.innerHTML = `<span class="bad">status: ${escapeHtml(err.message)}</span>`;
    return;
  }
  const s = statusData;
  const just = s.just.mode === "none"
    ? `<span class="bad">${t("just")}: ${t("justNone")}</span>`
    : `<span class="${s.just.mode === "system" ? "ok" : "warn"}">${t("just")}: ${t("just" + s.just.mode[0].toUpperCase() + s.just.mode.slice(1))} ${s.just.version || ""}</span>`;
  const love = s.love.found
    ? `<span class="ok">${t("love")}: ${escapeHtml(s.love.path)}</span>`
    : `<span class="bad">${t("loveMissing")}</span>`;
  const engine = s.engineRoot
    ? `<span class="ok">${t("engine")}: ${escapeHtml(s.engineRoot)}</span>`
    : `<span class="bad">${t("noEngine")}</span>`;
  const mod = s.modRoot
    ? `<span class="ok">${t("mod")}: ${escapeHtml(s.modRoot)}</span>`
    : `<span class="bad">${t("noMod")}</span>`;
  bar.innerHTML = love + engine + mod + just;
}

/* --- tasks --- */

async function loadTasks() {
  const list = document.getElementById("task-list");
  try {
    const l = await api("/api/tasks");
    renderTasks(l);
  } catch (err) {
    list.innerHTML = `<p class="hint err">${escapeHtml(err.message)}</p>`;
  }
}

function renderTasks(l) {
  const list = document.getElementById("task-list");
  list.innerHTML = "";
  if (l.source === "builtin") {
    list.innerHTML = `<p class="hint err">${t("justUnavailable")}${l.note ? " — " + escapeHtml(l.note) : ""}</p>`;
    return;
  }
  if (!l.tasks.length) {
    list.innerHTML = `<p class="hint">${t("noTasks")}</p>`;
    return;
  }
  for (const task of l.tasks) {
    const row = document.createElement("div");
    row.className = "task-row";

    const name = document.createElement("span");
    name.className = "task-name";
    name.textContent = task.name;
    if (task.aliases && task.aliases.length) {
      const al = document.createElement("span");
      al.className = "alias";
      al.textContent = " [" + task.aliases.join(", ") + "]";
      name.appendChild(al);
    }
    row.appendChild(name);

    const params = document.createElement("div");
    params.className = "task-params";
    for (const p of task.params || []) {
      params.appendChild(renderParam(task, p));
    }
    row.appendChild(params);

    if (task.doc) {
      const doc = document.createElement("span");
      doc.className = "task-doc";
      doc.textContent = task.doc;
      row.appendChild(doc);
    }

    const btn = document.createElement("button");
    btn.className = "btn small";
    btn.textContent = "▶ " + t("run");
    btn.onclick = () => runTask(task, collectTaskArgs(task));
    row.appendChild(btn);

    list.appendChild(row);
  }
}

function renderParam(task, p) {
  const wrap = document.createElement("label");
  if (p.kind === "flag") {
    const cb = document.createElement("input");
    cb.type = "checkbox";
    cb.id = `p-${task.name}-${p.name}`;
    const val = document.createElement("input");
    val.type = "text";
    val.id = `pv-${task.name}-${p.name}`;
    val.placeholder = "--" + p.name + "=value";
    val.className = "flag-input";
    cb.onchange = () => val.classList.toggle("on", cb.checked);
    wrap.appendChild(cb);
    wrap.appendChild(document.createTextNode("--" + p.name));
    wrap.appendChild(val);
    return wrap;
  }
  const input = document.createElement("input");
  input.type = "text";
  input.id = `p-${task.name}-${p.name}`;
  input.placeholder = p.kind === "many" ? "a, b, c" : p.kind === "star" ? "arg1 arg2" : "";
  wrap.appendChild(document.createTextNode(p.kind === "many" ? "[" + p.name + "]" : p.name + ":"));
  wrap.appendChild(input);
  return wrap;
}

function collectTaskArgs(task) {
  const args = [];
  for (const p of task.params || []) {
    const ctrl = document.getElementById(`p-${task.name}-${p.name}`);
    if (!ctrl) continue;
    if (p.kind === "flag") {
      const val = document.getElementById(`pv-${task.name}-${p.name}`);
      if (ctrl.checked) {
        args.push(val && val.value ? `--${p.name}=${val.value}` : `--${p.name}`);
      }
    } else if (p.kind === "star") {
      args.push(...ctrl.value.trim().split(/\s+/).filter(Boolean));
    } else if (p.kind === "many") {
      args.push(...ctrl.value.split(",").map((s) => s.trim()).filter(Boolean));
    } else if (ctrl.value.trim()) {
      args.push(ctrl.value.trim());
    }
  }
  return args;
}

async function runTask(task, args) {
  clearTerminal();
  setRunning(false);
  try {
    const { id } = await postJSON("/api/runs", { task: task.name, args });
    openRun(id);
  } catch (err) {
    appendTerm(t("taskFail") + ": " + err.message, "err");
  }
}

/* --- launch game --- */

async function launchGame() {
  const val = (id) => document.getElementById(id).value.trim();
  const passthrough = val("la-extra").split(/\s+/).filter(Boolean);
  clearTerminal();
  setRunning(false);
  const hint = document.getElementById("launch-hint");
  hint.textContent = "";
  hint.className = "hint";
  try {
    const { id } = await postJSON("/api/game/launch", {
      lang: val("la-lang"),
      encounter: val("la-encounter"),
      wave: val("la-wave"),
      waveForce: val("la-waveforce"),
      tp: val("la-tp"),
      mercy: val("la-mercy"),
      passthrough,
    });
    hint.textContent = t("launchOk");
    hint.className = "hint ok";
    openRun(id);
  } catch (err) {
    hint.textContent = err.message;
    hint.className = "hint err";
    appendTerm(err.message, "err");
  }
}

/* --- terminal + SSE --- */

const term = document.getElementById("terminal");

function clearTerminal() {
  term.textContent = "";
  term.removeAttribute("data-empty");
}

function appendTerm(text, cls) {
  const span = document.createElement("span");
  if (cls) span.className = cls;
  span.textContent = text;
  term.appendChild(span);
  term.scrollTop = term.scrollHeight;
}

function setRunning(on) {
  const btn = document.getElementById("cancel-btn");
  btn.hidden = !on;
  if (on && currentRun) btn.disabled = false;
}

function openRun(id) {
  if (currentRun && currentRun.es) currentRun.es.close();
  currentRun = { id, es: null };
  setRunning(true);
  const es = new EventSource(`/api/runs/${id}/stream`);
  currentRun.es = es;
  es.onmessage = (e) => {
    let ev;
    try { ev = JSON.parse(e.data); } catch (_) { return; }
    if (ev.type === "exit") {
      appendTerm("\n[" + t("exited") + " " + ev.code + "]", ev.code === 0 ? "excline" : "excline bad");
      es.close();
      setRunning(false);
      loadRuns();
    } else if (ev.type === "error") {
      appendTerm(ev.message || "", "err");
    } else {
      appendTerm(ev.text, ev.stream === "stderr" ? "err" : "");
    }
  };
  es.onerror = () => {
    /* The server closes the stream after the exit event (we already closed);
       transient errors auto-reconnect. */
  };
}

async function cancelRun() {
  if (!currentRun) return;
  const { id } = currentRun;
  try { await postJSON(`/api/runs/${id}/cancel`, {}); } catch (_) { /* best effort */ }
}

/* --- runs log --- */

async function loadRuns() {
  const box = document.getElementById("runs-log");
  let data;
  try { data = await api("/api/runs"); } catch (_) { return; }
  box.innerHTML = "";
  if (!data.runs || !data.runs.length) {
    box.innerHTML = `<span class="hint">${t("empty")}</span>`;
    return;
  }
  for (const r of data.runs.slice().reverse()) {
    const row = document.createElement("div");
    row.className = "run-entry";
    const label = document.createElement("span");
    label.className = "r-label";
    label.textContent = r.label;
    const cmd = document.createElement("span");
    cmd.className = "r-cmd";
    cmd.textContent = r.command;
    cmd.title = r.command;
    const meta = document.createElement("span");
    meta.className = "r-meta";
    meta.textContent = (r.duration || t("running")) + " ";
    const code = document.createElement("span");
    code.className = "r-code " + (r.code < 0 ? "run" : r.code === 0 ? "ok" : "bad");
    code.textContent = r.code < 0 ? t("running") : t("exited") + " " + r.code;
    row.append(label, cmd, meta, code);
    box.appendChild(row);
  }
}

/* --- misc --- */

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  }[c]));
}

/* --- wire up --- */

document.getElementById("refresh-tasks").onclick = loadTasks;
document.getElementById("cancel-btn").onclick = cancelRun;
document.getElementById("launch-btn").onclick = launchGame;

document.getElementById("lang-toggle").onclick = () => {
  lang = lang === "zh" ? "en" : "zh";
  localStorage.setItem("kdt-lang", lang);
  applyI18n();
  loadStatus();
  loadTasks();
  loadRuns();
};

document.getElementById("copy-url").onclick = async () => {
  try {
    await navigator.clipboard.writeText(location.href);
    const btn = document.getElementById("copy-url");
    const old = btn.textContent;
    btn.textContent = t("copied");
    setTimeout(() => { btn.textContent = old; }, 1500);
  } catch (_) { /* clipboard unavailable in webview */ }
};

applyI18n();
loadStatus();
loadTasks();
loadRuns();
