"use strict";

/* kristal-debug-tools GUI frontend — vanilla JS, zero build. */

const I18N = {
  zh: {
    tasks: "运行项列表（高级）", launch: "启动游戏", notice: "说明", runs: "运行记录",
    project: "项目信息", system: "系统", libraries: "依赖库",
    run: "运行", refresh: "刷新",
    copyUrl: "复制地址", copied: "已复制",
    loading: "加载中…",
    fLang: "语言", fEncounter: "遭遇", fWave: "波次", fWaveForce: "强制波次",
    fTp: "初始 TP", fMercy: "初始 mercy", fExtra: "额外参数",
    love: "love", engine: "引擎", mod: "模组", just: "just",
    loveMissing: "未找到 love（请安装 LÖVE 并加入 PATH）",
    noEngine: "未找到 Kristal 引擎",
    noMod: "未找到 mod",
    justNone: "不可用",
    justSystem: "系统", justEmbedded: "内置", justCache: "缓存",
    started: "开始", empty: "（空）",
    noticeText: "游戏与任务会打开在独立终端窗口（可交互），输出不再显示在这里。",
    launchOk: "游戏已在新终端窗口中启动",
    taskStarted: "任务已在新终端窗口中启动",
    taskFail: "启动失败", justUnavailable: "just 不可用，无法列出任务",
    noTasks: "没有可运行的任务",
    spawned: "已在新终端启动",
  },
  en: {
    tasks: "RUN LIST (ADVANCED)", launch: "LAUNCH GAME", notice: "NOTICE", runs: "RUNS",
    project: "PROJECT", system: "system", libraries: "libraries",
    run: "RUN", refresh: "REFRESH",
    copyUrl: "COPY URL", copied: "COPIED",
    loading: "loading…",
    fLang: "LANGUAGE", fEncounter: "ENCOUNTER", fWave: "WAVE", fWaveForce: "WAVE FORCE",
    fTp: "TP", fMercy: "MERCY", fExtra: "EXTRA ARGS",
    love: "love", engine: "engine", mod: "mod", just: "just",
    loveMissing: "love not found (install LÖVE and add it to PATH)",
    noEngine: "Kristal engine not found",
    noMod: "mod not found",
    justNone: "unavailable",
    justSystem: "system", justEmbedded: "embedded", justCache: "cache",
    started: "started", empty: "(empty)",
    noticeText: "Game and tasks open in a separate interactive terminal window — output no longer appears here.",
    launchOk: "game launched in a new terminal window",
    taskStarted: "task started in a new terminal window",
    taskFail: "start failed", justUnavailable: "just unavailable — no tasks to list",
    noTasks: "no runnable tasks",
    spawned: "spawned in new terminal",
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

/* Scale the whole UI with CSS zoom (WebKit-native; scales fonts, borders and
 * paddings together, so the layout stays proportional). devicePixelRatio
 * alone is not enough: a 192-DPI panel with desktop scale 1 reports dpr 1,
 * so we default to >= 1.25 (= Shadow's 20px body text) and let the user
 * adjust with the A− / A+ buttons (persisted). */
const MIN_SCALE = 0.75, MAX_SCALE = 3, BASE_SCALE = 1.25;

function dprDefaultScale() {
  const dpr = window.devicePixelRatio || 1;
  return Math.min(1.6, Math.max(BASE_SCALE, 0.6 + dpr * 0.5));
}

function currentScale() {
  let s = parseFloat(localStorage.getItem("kdt-scale"));
  if (!(s >= MIN_SCALE && s <= MAX_SCALE)) {
    s = dprDefaultScale();
  }
  return s;
}

function applyDpiScale() {
  const s = currentScale();
  document.documentElement.style.zoom = s.toFixed(3);
  const label = document.getElementById("scale-label");
  if (label) label.textContent = Math.round(s * 100) + "%";
}

function setScale(s) {
  s = Math.min(MAX_SCALE, Math.max(MIN_SCALE, s));
  localStorage.setItem("kdt-scale", String(s));
  applyDpiScale();
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

/* --- status bar --- */

async function loadStatus() {
  const bar = document.getElementById("statusbar");
  let s;
  try {
    s = await api("/api/status");
  } catch (err) {
    bar.innerHTML = `<span class="bad">status: ${escapeHtml(err.message)}</span>`;
    return;
  }
  const just = s.just.mode === "none"
    ? `<span class="bad">${t("just")}: ${t("justNone")}</span>`
    : `<span>${t("just")}: ${t("just" + s.just.mode[0].toUpperCase() + s.just.mode.slice(1))} ${s.just.version || ""}</span>`;
  const love = s.love.found
    ? `<span>${t("love")}: ${escapeHtml(s.love.path)}</span>`
    : `<span class="bad">${t("loveMissing")}</span>`;
  const engine = s.engineRoot
    ? `<span>${t("engine")}: ${escapeHtml(s.engineRoot)}</span>`
    : `<span class="bad">${t("noEngine")}</span>`;
  const mod = s.modRoot
    ? `<span>${t("mod")}: ${escapeHtml(s.modRoot)}</span>`
    : `<span class="bad">${t("noMod")}</span>`;
  const os = `<span>${t("system")}: ${escapeHtml(s.os)} ${escapeHtml(s.arch)}</span>`;
  bar.innerHTML = love + engine + mod + just + os;
  renderProject(s);
}

/* --- project info --- */

function renderProject(s) {
  const panel = document.getElementById("project-panel");
  const box = document.getElementById("project-info");
  if (!s.project || !s.project.id) {
    panel.hidden = true;
    return;
  }
  panel.hidden = false;
  let html = `<div class="proj-name">${escapeHtml(s.project.name || s.project.id)}</div>`;
  if (s.project.subtitle) {
    html += `<div class="proj-sub">${escapeHtml(s.project.subtitle)}</div>`;
  }
  if (s.libraries && s.libraries.length) {
    html += `<div class="proj-libs">${t("libraries")}:</div><ul class="proj-list">`;
    for (const lib of s.libraries) {
      html += `<li><span class="lib-id">${escapeHtml(lib.id)}</span> <span class="lib-ver">${escapeHtml(lib.version || "")}</span></li>`;
    }
    html += `</ul>`;
  }
  box.innerHTML = html;
}

/* --- transient flash message --- */

let flashTimer = null;
function showFlash(msg, isErr) {
  const el = document.getElementById("flash");
  el.textContent = msg;
  el.className = "flash" + (isErr ? " err" : "");
  clearTimeout(flashTimer);
  flashTimer = setTimeout(() => { el.textContent = ""; el.className = "flash"; }, 4000);
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
  try {
    await postJSON("/api/runs", { task: task.name, args });
    showFlash(t("taskStarted"));
    loadRuns();
  } catch (err) {
    showFlash(t("taskFail") + ": " + err.message, true);
  }
}

/* --- launch game --- */

async function launchGame() {
  const val = (id) => document.getElementById(id).value.trim();
  const passthrough = val("la-extra").split(/\s+/).filter(Boolean);
  try {
    await postJSON("/api/game/launch", {
      lang: val("la-lang"),
      encounter: val("la-encounter"),
      wave: val("la-wave"),
      waveForce: val("la-waveforce"),
      tp: val("la-tp"),
      mercy: val("la-mercy"),
      passthrough,
    });
    showFlash(t("launchOk"));
    loadRuns();
  } catch (err) {
    showFlash(err.message, true);
  }
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
    meta.textContent = t("spawned");
    row.append(label, cmd, meta);
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

applyDpiScale();
window.addEventListener("resize", applyDpiScale);
if (window.matchMedia) {
  window.matchMedia(`(resolution: ${(window.devicePixelRatio || 1).toFixed(2)}dppx)`)
    .addEventListener("change", applyDpiScale);
}

document.getElementById("scale-down").onclick = () => setScale(currentScale() / 1.15);
document.getElementById("scale-up").onclick = () => setScale(currentScale() * 1.15);
document.getElementById("scale-label").onclick = () => setScale(dprDefaultScale());

applyI18n();
loadStatus();
loadTasks();
loadRuns();
