// task143-batchreactor frontend: native JS, no build step. Talks to the Go
// API over fetch. Covers the full read/write flow: create reactors/recipes,
// set the cleaning matrix, run a dry kinetics simulation, create+plan+start a
// campaign, advance batch lifecycle, view thermal-safety results.
"use strict";

const api = (m, p, b) =>
  fetch(p, { method: m, headers: { "Content-Type": "application/json" }, body: b ? JSON.stringify(b) : undefined }).then(r => r.ok ? r.json() : r.json().then(j => Promise.reject(j)));

let currentCampaign = null;

// --- tab navigation ---
document.querySelectorAll(".tabs button").forEach(btn => btn.addEventListener("click", () => {
  document.querySelectorAll(".tabs button").forEach(b => b.classList.remove("active"));
  document.querySelectorAll(".panel").forEach(p => p.classList.remove("active"));
  btn.classList.add("active");
  document.getElementById("tab-" + btn.dataset.tab).classList.add("active");
  if (btn.dataset.tab === "simulate" || btn.dataset.tab === "campaigns") refreshSelects();
}));

// --- reactors ---
document.getElementById("reactor-form").addEventListener("submit", async e => {
  e.preventDefault();
  const f = formObj(e.target);
  try { await api("POST", "/api/reactors", f); e.target.reset(); refreshReactors(); } catch (err) { alertErr(err); }
});
async function refreshReactors() {
  const { reactors } = await api("GET", "/api/reactors");
  const tb = document.querySelector("#reactor-table tbody");
  tb.innerHTML = "";
  (reactors || []).forEach(r => {
    const tr = document.createElement("tr");
    tr.innerHTML = `<td>${r.id}</td><td>${r.name}</td><td>${r.volume}</td><td>${r.heat_transfer_u}</td><td>${r.heat_transfer_area}</td><td>${r.max_operating_temp}</td><td class="status-${r.status}">${r.status}</td>`;
    tb.appendChild(tr);
  });
}

// --- recipes ---
document.getElementById("recipe-form").addEventListener("submit", async e => {
  e.preventDefault();
  const f = formObj(e.target);
  f.order = parseInt(f.order, 10);
  try { await api("POST", "/api/recipes", f); e.target.reset(); refreshRecipes(); } catch (err) { alertErr(err); }
});
async function refreshRecipes() {
  const { recipes } = await api("GET", "/api/recipes");
  const tb = document.querySelector("#recipe-table tbody");
  tb.innerHTML = "";
  (recipes || []).forEach(r => {
    const dTad = ((-r.delta_h_rx) * r.ca0 / (r.rho * r.cp)).toFixed(1);
    const tr = document.createElement("tr");
    tr.innerHTML = `<td>${r.id}</td><td>${r.name}</td><td>${r.product}</td><td>${r.k0}</td><td>${r.ea}</td><td>${r.order}</td><td>${dTad} K</td>`;
    tb.appendChild(tr);
  });
}

// --- cleaning matrix ---
document.getElementById("cleaning-form").addEventListener("submit", async e => {
  e.preventDefault();
  const f = formObj(e.target);
  f.severity = parseInt(f.severity, 10);
  try { await api("PUT", `/api/cleaning-matrix/${f.from_product}/${f.to_product}`, { severity: f.severity }); e.target.reset(); refreshCleaning(); } catch (err) { alertErr(err); }
});
async function refreshCleaning() {
  const { matrix } = await api("GET", "/api/cleaning-matrix");
  const tb = document.querySelector("#cleaning-table tbody");
  tb.innerHTML = "";
  const dur = { 0: 0, 1: 600, 2: 1800, 3: 3600 };
  (matrix || []).forEach(e => {
    const tr = document.createElement("tr");
    tr.innerHTML = `<td>${e.from_product}</td><td>${e.to_product}</td><td>${e.severity}</td><td>${dur[e.severity] || 0}</td>`;
    tb.appendChild(tr);
  });
}

// --- simulate ---
document.getElementById("simulate-form").addEventListener("submit", async e => {
  e.preventDefault();
  const f = formObj(e.target);
  try {
    const res = await api("POST", "/api/kinetics/simulate", { recipe_id: f.recipe_id, reactor_id: f.reactor_id });
    const box = document.getElementById("simulate-result");
    box.innerHTML = `<div class="card"><h3>动力学与热安全结果</h3>
      <pre>${JSON.stringify(res, null, 2)}</pre>
      <div class="meta">Stoessel 等级 ${res.stoessel_class} / 判定 <span class="verdict-${res.verdict}">${res.verdict}</span> / ΔT_ad ${res.delta_t_ad.toFixed(1)} K / MTSR ${res.mtsr.toFixed(1)} K / TMR ${res.tmr_seconds.toFixed(0)} s / 峰值温度 ${res.peak_temp.toFixed(1)} K / 转化率 ${(res.conversion * 100).toFixed(2)}%</div></div>`;
  } catch (err) { alertErr(err); }
});

// --- campaigns ---
document.getElementById("campaign-form").addEventListener("submit", async e => {
  e.preventDefault();
  const f = formObj(e.target);
  try {
    const c = await api("POST", "/api/campaigns", { name: f.name });
    currentCampaign = c.id;
    document.getElementById("item-form").classList.remove("hidden");
    refreshCampaigns();
  } catch (err) { alertErr(err); }
});
document.getElementById("item-form").addEventListener("submit", async e => {
  e.preventDefault();
  const f = formObj(e.target);
  f.batch_count = parseInt(f.batch_count, 10);
  try { await api("POST", `/api/campaigns/${currentCampaign}/items`, f); refreshCampaignDetail(); } catch (err) { alertErr(err); }
});
document.getElementById("plan-btn").addEventListener("click", async () => {
  if (!currentCampaign) return alert("先选择活动");
  try { await api("POST", `/api/campaigns/${currentCampaign}/plan`); refreshCampaignDetail(); } catch (err) { alertErr(err); }
});
document.getElementById("start-btn").addEventListener("click", async () => {
  if (!currentCampaign) return alert("先选择活动");
  try { await api("POST", `/api/campaigns/${currentCampaign}/start`); refreshCampaignDetail(); } catch (err) { alertErr(err); }
});
document.getElementById("report-btn").addEventListener("click", refreshReport);

async function refreshCampaigns() {
  const { campaigns } = await api("GET", "/api/campaigns");
  const list = document.getElementById("campaign-list");
  list.innerHTML = "";
  (campaigns || []).forEach(c => {
    const card = document.createElement("div");
    card.className = "card";
    card.innerHTML = `<h3>${c.name}</h3><div class="meta">${c.id} · ${c.status}</div>`;
    card.addEventListener("click", () => { currentCampaign = c.id; document.getElementById("item-form").classList.remove("hidden"); refreshCampaignDetail(); });
    list.appendChild(card);
  });
}

async function refreshCampaignDetail() {
  if (!currentCampaign) return;
  try {
    const det = await api("GET", `/api/campaigns/${currentCampaign}`);
    document.getElementById("campaign-detail").textContent =
      `活动 ${det.campaign.name} (${det.campaign.status})\n` +
      (det.items || []).map(i => `  项 #${i.seq} 配方=${i.recipe_id} 反应釜=${i.reactor_id} 批数=${i.batch_count} (${i.status})`).join("\n");
    const bs = await api("GET", `/api/campaigns/${currentCampaign}/batches`);
    renderBatches(bs.batches || []);
  } catch (err) { alertErr(err); }
}

function renderBatches(batches) {
  const list = document.getElementById("batch-list");
  list.innerHTML = "";
  if (!batches.length) { list.textContent = "无批次"; return; }
  batches.forEach(b => {
    const card = document.createElement("div");
    card.className = "card";
    const conv = b.conversion ? (b.conversion * 100).toFixed(1) + "%" : "—";
    card.innerHTML = `<h3>批次 ${b.id}</h3><div class="meta">反应釜 ${b.reactor_id} · 配方 ${b.recipe_id} · 序 ${b.seq} · 状态 <span class="status-${b.status}">${b.status}</span> · 转化率 ${conv} · 峰值 ${b.peak_temp ? b.peak_temp.toFixed(1) : "—"} K · 判定 ${b.safety_verdict || "—"}${b.fault_reason ? " · " + b.fault_reason : ""}</div>`;
    const adv = document.createElement("div");
    adv.className = "actions";
    if (isTerminal(b.status)) {
      const tag = document.createElement("span");
      tag.className = "meta terminal-note";
      tag.textContent = "已到终态，无后续操作";
      adv.appendChild(tag);
    } else {
      ["charging", "reacting", "cooling", "discharging", "cleaning", "done"].forEach(st => {
        const btn = document.createElement("button");
        btn.textContent = "→" + st;
        btn.addEventListener("click", async () => {
          try { await api("POST", `/api/batches/${b.id}/advance`, { target: st }); refreshCampaignDetail(); } catch (err) { alertErr(err); }
        });
        adv.appendChild(btn);
      });
      const ab = document.createElement("button");
      ab.textContent = "中止";
      ab.addEventListener("click", async () => { try { await api("POST", `/api/batches/${b.id}/abort`); refreshCampaignDetail(); } catch (err) { alertErr(err); } });
      adv.appendChild(ab);
    }
    card.appendChild(adv);
    list.appendChild(card);
  });
}

// isTerminal mirrors model.BatchStatus.IsTerminal so the running view hides
// transition buttons for batches that have reached an end-state (done /
// faulted / aborted): offering advance or abort actions on a terminal batch
// would surface an inappropriate next action for a batch that can no longer
// transition.
function isTerminal(status) { return status === "done" || status === "faulted" || status === "aborted"; }

async function refreshReport() {
  if (!currentCampaign) return alert("先选择活动");
  try {
    const rep = await api("GET", `/api/campaigns/${currentCampaign}/full-report`);
    document.getElementById("report-box").textContent =
      `活动 ${rep.campaign.name} (${rep.campaign.status})\n` +
      `项数 ${(rep.items || []).length} 批次数 ${(rep.batches || []).length}\n` +
      (rep.batches || []).map(b => `  ${b.id} reactor=${b.reactor_id} seq=${b.seq} ${b.status} conv=${(b.conversion * 100).toFixed(1)}% peak=${b.peak_temp ? b.peak_temp.toFixed(1) : "-"}K verdict=${b.safety_verdict || "-"}`).join("\n");
  } catch (err) { alertErr(err); }
}

// --- shared helpers ---
function formObj(form) {
  const o = {};
  new FormData(form).forEach((v, k) => { o[k] = v; });
  // cast numerics
  for (const k in o) if (/^-?\d+(\.\d+)?$/.test(o[k])) o[k] = parseFloat(o[k]);
  return o;
}
function alertErr(err) { alert(typeof err === "string" ? err : (err.error || JSON.stringify(err))); }
async function refreshSelects() {
  const [{ reactors }, { recipes }] = await Promise.all([api("GET", "/api/reactors"), api("GET", "/api/recipes")]);
  const fill = (sel, items, id, name) => { sel.innerHTML = items.map(i => `<option value="${i[id]}">${i[name]}</option>`).join(""); };
  const sr = document.querySelector('#simulate-form select[name="recipe_id"]');
  const srr = document.querySelector('#simulate-form select[name="reactor_id"]');
  if (sr) fill(sr, recipes || [], "id", "name");
  if (srr) fill(srr, reactors || [], "id", "name");
  const ir = document.querySelector('#item-form select[name="recipe_id"]');
  const irr = document.querySelector('#item-form select[name="reactor_id"]');
  if (ir) fill(ir, recipes || [], "id", "name");
  if (irr) fill(irr, reactors || [], "id", "name");
}

// init
refreshReactors(); refreshRecipes(); refreshCleaning(); refreshCampaigns();
