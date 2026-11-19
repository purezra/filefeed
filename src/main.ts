import { Service, type SearchQuery, type Config, type Preset, type TypeGroup, type FileItem } from "./mygo";

// ------------------------------------------------------------------ 常量
const TYPE_GROUPS: Record<string, string[]> = {
  doc: ["md", "txt", "doc", "docx", "pdf", "rtf", "odt", "pages"],
  code: ["go", "ts", "tsx", "js", "jsx", "py", "rs", "java", "c", "cpp", "h", "cs", "rb", "php", "sh", "sql", "json", "jsonl", "yaml", "yml", "toml", "xml"],
  image: ["png", "jpg", "jpeg", "gif", "webp", "bmp", "svg", "ico", "tif", "tiff", "heic", "avif"],
  video: ["mp4", "mkv", "mov", "avi", "webm", "flv", "wmv", "m4v", "ts", "mpg", "mpeg"],
  audio: ["mp3", "wav", "flac", "aac", "m4a", "ogg", "opus", "wma", "aiff"],
  archive: ["zip", "7z", "rar", "tar", "gz", "bz2", "xz", "zst"],
};
const MIN_CARD_WIDTH = 440;

// ------------------------------------------------------------------ 状态
const state = {
  exts: [] as string[],
  keyword: "",
  timeMode: "past" as "past" | "range",
  pastValue: 24,
  pastUnit: "hour",
  from: "",
  to: "",
  timeField: "created" as "created" | "modified",
  sortBy: "time",
  limit: 500,
  exclude: [] as string[],
  config: null as Config | null,
  busy: false,
};

/** 当前选中的文件路径 */
const sel = new Set<string>();
/** shift 连选的锚点 */
let anchor: { type: string; index: number } | null = null;
const resultPaths = new Set<string>();
const resultGroups = new Map<string, TypeGroup>();
const foldedTypes = new Set<string>();
const listOffsets = new Map<string, number>();
const grid = document.getElementById("grid")!;
const waterfall = document.getElementById("waterfall")!;
const formatNav = document.getElementById("formatNav")!;
let searchPending = false;
let renderedQuery = "";
let renderedGroups = "";
let refreshTimer: number | undefined;

// ------------------------------------------------------------------ 小工具
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const $ = (sel2: string): any => document.querySelector(sel2);

let toastTimer: number | undefined;
function toast(msg: string) {
  const el = $("#toast");
  el.textContent = msg;
  el.classList.add("on");
  window.clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => el.classList.remove("on"), 1800);
}

function setStatus(t: string) {
  $("#status").textContent = t;
}

function fmtTime(sec: number) {
  if (!sec) return "";
  const d = new Date(sec * 1000);
  const p = (n: number) => String(n).padStart(2, "0");
  const now = new Date();
  const sameYear = d.getFullYear() === now.getFullYear();
  const date = sameYear
    ? `${p(d.getMonth() + 1)}-${p(d.getDate())}`
    : `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
  return `${date} ${p(d.getHours())}:${p(d.getMinutes())}`;
}

function nowLocalInput(offsetMs = 0) {
  const d = new Date(Date.now() + offsetMs);
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
}

async function copyText(t: string) {
  try {
    await navigator.clipboard.writeText(t);
    return true;
  } catch {
    const ta = document.createElement("textarea");
    ta.value = t;
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.select();
    let ok = false;
    try { ok = document.execCommand("copy"); } catch { /* ignore */ }
    document.body.removeChild(ta);
    return ok;
  }
}

function el<K extends keyof HTMLElementTagNameMap>(tag: K, cls?: string, text?: string) {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}

// ------------------------------------------------------------------ 选择
function updateSelBar() {
  const bar = $("#selBar");
  const n = sel.size;
  $("#selCount").textContent = String(n);
  bar.classList.toggle("hide", n === 0);
}

function togglePath(path: string, on?: boolean) {
  const want = on === undefined ? !sel.has(path) : on;
  if (want) sel.add(path);
  else sel.delete(path);
}

function applyRowSel() {
  document.querySelectorAll<HTMLElement>(".row").forEach((r) => {
    const p = r.dataset.path || "";
    const isSel = sel.has(p);
    r.classList.toggle("sel", isSel);
    const chk = r.querySelector<HTMLInputElement>(".chk");
    if (chk && chk.checked !== isSel) chk.checked = isSel;
  });
  // 卡片头部的全选状态
  waterfall.querySelectorAll<HTMLInputElement>(".cardChk").forEach((chk) => {
    const items = resultGroups.get(chk.dataset.type!)!.items;
    let picked = 0;
    for (const item of items) if (sel.has(item.path)) picked++;
    chk.checked = items.length > 0 && picked === items.length;
    chk.indeterminate = picked > 0 && picked < items.length;
  });
  updateSelBar();
}

// ------------------------------------------------------------------ 类型 chips
function renderChips() {
  document.querySelectorAll<HTMLButtonElement>("[data-group]:not([data-group=clear])").forEach((button) => {
    const exts = TYPE_GROUPS[button.dataset.group!]!;
    const active = exts.length === state.exts.length && exts.every((ext) => state.exts.includes(ext));
    button.setAttribute("aria-pressed", String(active));
  });
  const box = $("#typeChips");
  box.innerHTML = "";
  if (!state.exts.length) {
    box.appendChild(el("span", "muted small", "未选类型（点右侧「编辑」添加）"));
    return;
  }
  for (const ext of state.exts) {
    const c = el("span", "chip on", ext);
    c.title = "点击移除";
    c.onclick = () => {
      state.exts = state.exts.filter((x) => x !== ext);
      renderChips();
      void runSearch();
    };
    box.appendChild(c);
  }
}

// ------------------------------------------------------------------ 查询
function buildQuery(): SearchQuery {
  return {
    exts: state.exts,
    keyword: state.keyword,
    mode: state.timeMode,
    pastValue: state.pastValue,
    pastUnit: state.pastUnit,
    from: state.timeMode === "range" ? state.from : "",
    to: state.timeMode === "range" ? state.to : "",
    timeField: state.timeField,
    exclude: state.exclude,
    limit: state.limit,
    sortBy: state.sortBy,
  };
}

async function runSearch() {
  if (state.busy) {
    searchPending = true;
    return;
  }
  if (!state.exts.length) {
    renderedQuery = "";
    renderedGroups = "";
    renderGroups([], { empty: "先选一个文件类型，或点击快捷分类" });
    setStatus("未选类型");
    $("#footStat").textContent = "—";
    $("#footRange").textContent = "";
    return;
  }
  const query = buildQuery();
  state.busy = true;
  setStatus("搜索中…");
  try {
    const res = await Service.search(query);
    if (searchPending) return;
    if (!res.ok) {
      setStatus("出错");
      toast(res.error || "搜索失败");
      return;
    }
    const groups = res.groups ?? [];
    const queryKey = JSON.stringify(query);
    const groupKey = JSON.stringify(groups);
    const keepScroll = queryKey === renderedQuery;
    if (!keepScroll || groupKey !== renderedGroups) {
      renderGroups(groups, { keepScroll });
      renderedQuery = queryKey;
      renderedGroups = groupKey;
    }
    setStatus(`命中 ${res.total} 条 · ${res.elapsedMs} ms`);
    $("#footStat").textContent = `命中 ${res.total} 条 · ${groups.length} 种类型 · 用时 ${res.elapsedMs} ms`;
    const lo = res.lo ? fmtTime(res.lo) : "不限";
    const hi = res.hi ? fmtTime(res.hi) : "现在";
    $("#footRange").textContent = `${query.timeField === "created" ? "创建" : "修改"}时间：${lo} → ${hi}`;
  } catch (e) {
    if (!searchPending) {
      setStatus("出错");
      toast(String(e));
    }
  } finally {
    state.busy = false;
    if (searchPending) {
      searchPending = false;
      void runSearch();
    }
  }
}

function scheduleAutoRefresh(seconds: number) {
  window.clearInterval(refreshTimer);
  $("#btn-refresh").title = `立即刷新（Ctrl+R） · 自动每 ${seconds} 秒`;
  refreshTimer = window.setInterval(() => {
    if (!document.hidden && !state.busy && state.exts.length) void runSearch();
  }, seconds * 1000);
}

// ------------------------------------------------------------------ 渲染
function layoutCards() {
  const cards = Array.from(waterfall.querySelectorAll<HTMLElement>(".card"));
  const gap = parseFloat(getComputedStyle(waterfall).columnGap);
  const width = waterfall.clientWidth;
  const columns = Math.min(cards.length, Math.max(1, Math.floor((width + gap) / (MIN_CARD_WIDTH + gap))));
  waterfall.classList.toggle("masonry", columns > 1);
  if (columns <= 1) {
    waterfall.style.height = "";
    return;
  }

  const cardWidth = (width - (columns - 1) * gap) / columns;
  waterfall.style.setProperty("--card-width", cardWidth + "px");
  const heights = cards.map((card) => card.getBoundingClientRect().height);
  const bottoms: number[] = Array(columns).fill(0);
  cards.forEach((card, index) => {
    const top = Math.min(...bottoms);
    const column = bottoms.indexOf(top);
    card.style.left = column * (cardWidth + gap) + "px";
    card.style.top = top + "px";
    bottoms[column] = top + heights[index]! + gap;
  });
  waterfall.style.height = Math.max(...bottoms) - gap + "px";
}

function renderGroups(groups: TypeGroup[], { empty = "这个条件下没有匹配的文件", keepScroll = false } = {}) {
  const scrollTop = keepScroll ? grid.scrollTop : 0;
  if (keepScroll) {
    waterfall.querySelectorAll<HTMLElement>(".card:not(.folded) .list").forEach((list) => {
      listOffsets.set(list.dataset.type!, list.scrollTop);
    });
  } else {
    listOffsets.clear();
  }
  const cards = document.createDocumentFragment();
  const links = document.createDocumentFragment();
  const multiple = groups.length > 1;

  resultPaths.clear();
  resultGroups.clear();
  anchor = null;
  if (!groups.length) cards.appendChild(el("div", "empty", empty));

  groups.forEach((group, index) => {
    resultGroups.set(group.type, group);
    for (const item of group.items) resultPaths.add(item.path);
    const { card, fold } = renderCard(group, index);
    cards.appendChild(card);

    if (multiple) {
      const format = group.type.toUpperCase();
      const link = el("button", "format-link");
      link.type = "button";
      link.title = "跳转到 " + format;
      link.append(el("span", "format-name", format), el("span", "", String(group.count)));
      link.onclick = () => {
        card.scrollIntoView({ block: "start", inline: "nearest" });
        fold.focus({ preventScroll: true });
      };
      links.appendChild(link);
    }
  });

  for (const path of sel) if (!resultPaths.has(path)) sel.delete(path);
  formatNav.replaceChildren(links);
  formatNav.classList.toggle("hide", !multiple);
  waterfall.replaceChildren(cards);
  waterfall.classList.toggle("single", groups.length === 1);
  applyRowSel();
  layoutCards();
  waterfall.querySelectorAll<HTMLElement>(".card:not(.folded) .list").forEach((list) => {
    list.scrollTop = listOffsets.get(list.dataset.type!) ?? 0;
  });
  grid.scrollTop = scrollTop;
}

function renderCard(g: TypeGroup, index: number) {
  const format = g.type.toUpperCase();
  const card = el("section", "card");
  card.dataset.type = g.type;

  const head = el("div", "card-head");
  const title = el("h2", "ext", format);
  title.id = "format-" + index;
  card.setAttribute("aria-labelledby", title.id);
  const cardChk = el("input", "chk cardChk");
  cardChk.type = "checkbox";
  cardChk.dataset.type = g.type;
  cardChk.title = "全选 " + format + " 已加载的 " + g.items.length + " 个文件";
  cardChk.setAttribute("aria-label", cardChk.title);
  cardChk.onchange = () => {
    for (const it of g.items) togglePath(it.path, cardChk.checked);
    applyRowSel();
  };
  head.append(cardChk, title, el("span", "cnt", g.count + " 个"));
  if (g.items.length < g.count) {
    head.appendChild(el("span", "more", "上限 " + g.items.length));
  }

  const fold = el("button", "foldbtn");
  fold.type = "button";
  const content = el("div");
  content.id = "files-" + index;
  fold.setAttribute("aria-controls", content.id);
  const list = el("div", "list");
  list.dataset.type = g.type;
  list.tabIndex = 0;
  list.setAttribute("role", "group");
  list.setAttribute("aria-label", format + " 文件列表");

  const updateFold = () => {
    const folded = foldedTypes.has(g.type);
    fold.textContent = folded ? "展开" : "收起";
    fold.setAttribute("aria-expanded", String(!folded));
    fold.setAttribute("aria-label", fold.textContent + " " + format + " 格式");
    content.hidden = folded;
    card.classList.toggle("folded", folded);
  };
  fold.onclick = () => {
    if (!foldedTypes.delete(g.type)) {
      foldedTypes.add(g.type);
      listOffsets.set(g.type, list.scrollTop);
    }
    updateFold();
    layoutCards();
    if (!content.hidden) list.scrollTop = listOffsets.get(g.type) ?? 0;
  };
  updateFold();
  head.appendChild(fold);

  g.items.forEach((item, i) => list.appendChild(renderFileRow(g, item, i)));
  content.appendChild(list);
  card.append(head, content);
  return { card, fold };
}

function renderFileRow(g: TypeGroup, it: FileItem, idx: number) {
  const row = el("div", "row");
  row.dataset.path = it.path;

  const chk = el("input", "chk") as HTMLInputElement;
  chk.type = "checkbox";
  chk.setAttribute("aria-label", "选择 " + it.name);
  chk.checked = sel.has(it.path);
  chk.onclick = (e) => {
    e.stopPropagation();
    togglePath(it.path, chk.checked);
    anchor = { type: g.type, index: idx };
    applyRowSel();
  };
  row.appendChild(chk);

  const body = el("div", "body");

  const name = el("div", "name");
  if (it.isNew) name.appendChild(el("span", "dot"));
  const nameTxt = el("span", "txt", it.name);
  name.appendChild(nameTxt);
  name.title = it.path;
  name.onclick = async (e) => {
    e.stopPropagation();
    const err = await Service.open(it.path);
    if (err) toast("打开失败：" + err);
  };
  // 第一行：文件名 + 操作按钮
  const line1 = el("div", "line1");
  line1.appendChild(name);

  const acts = el("div", "acts");
  const mk = (label: string, kind: string, danger = false) => {
    const b = el("button", "act" + (danger ? " danger" : ""));
    b.textContent = label;
    b.onclick = async (e) => {
      e.stopPropagation();
      await act(kind, [it.path]);
    };
    return b;
  };
  acts.append(
    mk("打开", "open"),
    mk("位置", "reveal"),
    mk("路径", "path"),
    mk("复制", "copy"),
    mk("删除", "delete", true),
  );
  line1.appendChild(acts);
  body.appendChild(line1);

  // 第二行：目录 · 大小 · 时间（占满整行）
  const sub = el("div", "sub");
  const dir = el("span", "dir", it.dir);
  dir.title = it.dir;
  sub.appendChild(dir);
  sub.appendChild(el("span", "sep", "·"));
  sub.appendChild(el("span", "", it.sizeText));
  sub.appendChild(el("span", "sep", "·"));
  sub.appendChild(el("span", "", fmtTime(it.stamp)));
  if (it.isNew) sub.appendChild(el("span", "newtag", "新建"));
  body.appendChild(sub);
  row.appendChild(body);

  // 点击行空白处：切换选择；Shift 连选
  row.onclick = (e: MouseEvent) => {
    if ((e.target as HTMLElement).closest(".act, .name, .chk")) return;
    if (e.shiftKey && anchor && anchor.type === g.type) {
      const [a, b] = anchor.index < idx ? [anchor.index, idx] : [idx, anchor.index];
      for (let i = a; i <= b; i++) {
        const it2 = g.items[i];
        if (it2) sel.add(it2.path);
      }
    } else {
      togglePath(it.path);
    }
    anchor = { type: g.type, index: idx };
    applyRowSel();
  };

  return row;
}

// ------------------------------------------------------------------ 操作
async function act(kind: string, paths: string[]) {
  if (!paths.length) return;
  if (kind === "open") {
    if (paths.length > 3 && !confirm(`打开 ${paths.length} 个文件？`)) return;
    let failed = 0;
    for (const p of paths) {
      const err = await Service.open(p);
      if (err) failed++;
    }
    toast(failed ? `已打开 ${paths.length - failed} 个，失败 ${failed} 个` : `已打开 ${paths.length} 个文件`);
    return;
  }
  if (kind === "reveal") {
    const first = paths[0];
    if (!first) return;
    const err = await Service.reveal(first);
    if (err) { toast("定位失败：" + err); return; }
    toast(paths.length > 1 ? `已定位第一个（共选 ${paths.length} 个）` : "已定位");
    return;
  }
  if (kind === "path") {
    const ok = await copyText(paths.join("\r\n"));
    toast(ok ? `已复制 ${paths.length} 条路径` : "复制失败");
    return;
  }
  if (kind === "copy") {
    const err = await Service.copyFiles(paths);
    toast(err ? "复制失败：" + err : `已复制 ${paths.length} 个文件，可粘贴到文件夹`);
    return;
  }
  if (kind === "delete") {
    const tip = paths.length === 1
      ? `把文件移到回收站？\n\n${(paths[0] ?? "").split(/[\\/]/).pop() ?? ""}`
      : `把 ${paths.length} 个文件移到回收站？`;
    if (!confirm(tip)) return;
    const err = await Service.deleteFiles(paths);
    if (err) { toast("删除失败：" + err); return; }
    for (const p of paths) sel.delete(p);
    updateSelBar();
    toast(`已移到回收站（${paths.length} 个）`);
    void runSearch();
  }
}

// ------------------------------------------------------------------ 设置
async function openSettings() {
  const cfg = await Service.config();
  state.config = cfg;
  $("#cfgExts").value = cfg.exts.join(", ");
  $("#cfgExclude").value = cfg.exclude.join("\n");
  $("#cfgLimit").value = String(cfg.limit);
  $("#cfgRefresh").value = String(cfg.refreshSeconds);
  $("#modalSettings").classList.remove("hide");
}

async function saveSettings() {
  const exts = ($("#cfgExts").value as string)
    .split(/[,\n]/)
    .map((s: string) => s.trim().replace(/^\./, "").toLowerCase())
    .filter(Boolean);
  const exclude = ($("#cfgExclude").value as string)
    .split("\n")
    .map((s: string) => s.trim())
    .filter(Boolean);
  const limit = Math.max(20, Math.min(5000, parseInt($("#cfgLimit").value) || 500));
  const refreshSeconds = Number($("#cfgRefresh").value);
  const typesChanged = exts.join("\n") !== state.config?.exts.join("\n");

  const cfg = await Service.saveConfig({
    exts,
    exclude,
    limit,
    refreshSeconds,
    presets: state.config?.presets ?? [],
  });
  state.config = cfg;
  if (typesChanged) state.exts = cfg.exts;
  state.exclude = cfg.exclude;
  state.limit = cfg.limit;
  scheduleAutoRefresh(cfg.refreshSeconds);
  $("#modalSettings").classList.add("hide");
  renderChips();
  toast("设置已保存");
  void runSearch();
}

// ------------------------------------------------------------------ 方案
function presetDesc(q: SearchQuery) {
  const t =
    q.mode === "range"
      ? `${q.from || "?"} → ${q.to || "现在"}`
      : `过去 ${q.pastValue} ${{ minute: "分钟", hour: "小时", day: "天", week: "周" }[q.pastUnit] || q.pastUnit}`;
  const parts = [t];
  if (q.keyword) parts.push(`含「${q.keyword}」`);
  if (q.exts?.length) parts.push(q.exts.slice(0, 4).join("/") + (q.exts.length > 4 ? ` 等${q.exts.length}种` : ""));
  return parts.join(" · ");
}

async function openPresets() {
  const cfg = state.config ?? (await Service.config());
  state.config = cfg;
  renderPresetList(cfg.presets ?? []);
  $("#modalPresets").classList.remove("hide");
}

function renderPresetList(list: Preset[]) {
  const box = $("#presetList");
  box.innerHTML = "";
  if (!list.length) {
    box.appendChild(el("div", "muted small", "还没有方案。配置好条件后点「保存当前」。"));
    return;
  }
  list.forEach((p, i) => {
    const item = el("div", "preset-item");
    const nm = el("div");
    nm.appendChild(el("div", "nm", p.name));
    nm.appendChild(el("div", "desc", presetDesc(p.query)));
    item.appendChild(nm);

    const apply = el("button", "btn tiny primary", "应用");
    apply.onclick = () => {
      applyQuery(p.query);
      $("#modalPresets").classList.add("hide");
      void runSearch();
    };
    const del = el("button", "btn tiny ghost", "删除");
    del.onclick = async () => {
      const cfg = state.config!;
      const next = (cfg.presets ?? []).filter((_, idx) => idx !== i);
      state.config = await Service.saveConfig({ ...cfg, presets: next });
      renderPresetList(state.config.presets ?? []);
      toast("已删除");
    };
    item.append(apply, del);
    box.appendChild(item);
  });
}

function applyQuery(q: SearchQuery) {
  state.exts = q.exts ?? [];
  state.keyword = q.keyword ?? "";
  state.timeMode = (q.mode === "range" ? "range" : "past") as "past" | "range";
  state.pastValue = q.pastValue || 24;
  state.pastUnit = q.pastUnit || "hour";
  state.from = q.from || "";
  state.to = q.to || "";
  state.timeField = (q.timeField === "modified" ? "modified" : "created") as "created" | "modified";
  state.sortBy = q.sortBy || "time";

  $("#kw").value = state.keyword;
  $("#pastValue").value = String(state.pastValue);
  $("#pastUnit").value = state.pastUnit;
  $("#from").value = state.from;
  $("#to").value = state.to;
  $("#timeField").value = state.timeField;
  $("#sortBy").value = state.sortBy;
  document.querySelectorAll("#timeMode button").forEach((b) => {
    b.classList.toggle("on", (b as HTMLElement).dataset.m === state.timeMode);
  });
  $("#pastBox").classList.toggle("hide", state.timeMode !== "past");
  $("#rangeBox").classList.toggle("hide", state.timeMode !== "range");
  renderChips();
}

async function addPreset() {
  const name = $("#presetName").value.trim();
  if (!name) { toast("先给方案起个名字"); return; }
  const cfg = state.config ?? (await Service.config());
  const presets = [...(cfg.presets ?? []), { name, query: buildQuery() }];
  state.config = await Service.saveConfig({ ...cfg, presets });
  $("#presetName").value = "";
  renderPresetList(state.config.presets ?? []);
  toast("方案已保存");
}

// ------------------------------------------------------------------ 事件
function bind() {
  document.querySelectorAll("#timeMode button").forEach((b) => {
    b.addEventListener("click", () => {
      const m = (b as HTMLElement).dataset.m as "past" | "range";
      state.timeMode = m;
      document.querySelectorAll("#timeMode button").forEach((x) => x.classList.toggle("on", x === b));
      $("#pastBox").classList.toggle("hide", m !== "past");
      $("#rangeBox").classList.toggle("hide", m !== "range");
      if (m === "range" && !state.from) {
        state.from = nowLocalInput(-24 * 3600 * 1000);
        state.to = nowLocalInput();
        $("#from").value = state.from;
        $("#to").value = state.to;
      }
    });
  });

  $("#kw").addEventListener("input", (e: Event) => { state.keyword = (e.target as HTMLInputElement).value; });
  $("#kw").addEventListener("keydown", (e: KeyboardEvent) => {
    if (e.key === "Enter") void runSearch();
  });

  $("#pastValue").addEventListener("input", (e: Event) => { state.pastValue = parseInt((e.target as HTMLInputElement).value) || 1; });
  $("#pastUnit").addEventListener("change", (e: Event) => { state.pastUnit = (e.target as HTMLSelectElement).value; });
  $("#from").addEventListener("change", (e: Event) => { state.from = (e.target as HTMLInputElement).value; });
  $("#to").addEventListener("change", (e: Event) => { state.to = (e.target as HTMLInputElement).value; });
  $("#timeField").addEventListener("change", (e: Event) => { state.timeField = (e.target as HTMLSelectElement).value as "created" | "modified"; });
  $("#sortBy").addEventListener("change", (e: Event) => { state.sortBy = (e.target as HTMLSelectElement).value; });

  $("#btn-search").addEventListener("click", () => void runSearch());
  $("#btn-refresh").addEventListener("click", () => void runSearch());
  $("#btn-settings").addEventListener("click", () => void openSettings());
  $("#btn-presets").addEventListener("click", () => void openPresets());
  $("#btnSaveCfg").addEventListener("click", () => void saveSettings());
  $("#btnAddPreset").addEventListener("click", () => void addPreset());
  $("#btn-types").addEventListener("click", () => void openSettings());

  $("#btnClearSel").addEventListener("click", () => {
    sel.clear();
    applyRowSel();
  });

  document.querySelectorAll("[data-bulk]").forEach((b) => {
    b.addEventListener("click", () => void act((b as HTMLElement).dataset.bulk!, [...sel]));
  });

  document.querySelectorAll<HTMLButtonElement>("[data-group]").forEach((b) => {
    b.addEventListener("click", () => {
      state.exts = [...(TYPE_GROUPS[b.dataset.group!] ?? [])];
      renderChips();
      void runSearch();
    });
  });

  document.querySelectorAll("[data-close]").forEach((b) => {
    b.addEventListener("click", () => $("#" + (b as HTMLElement).dataset.close!).classList.add("hide"));
  });

  document.addEventListener("keydown", (e: KeyboardEvent) => {
    if (e.key === "Escape") {
      document.querySelectorAll(".modal").forEach((m) => m.classList.add("hide"));
      if (sel.size) {
        sel.clear();
        applyRowSel();
      }
    }
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "f") {
      e.preventDefault();
      $("#kw").focus();
    }
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "r") {
      e.preventDefault();
      void runSearch();
    }
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "a") {
      const t = e.target as HTMLElement;
      if (t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA")) return;
      e.preventDefault();
      for (const p of resultPaths) sel.add(p);
      applyRowSel();
    }
  });
}

// ------------------------------------------------------------------ 启动
async function boot() {
  bind();
  const cfg = await Service.config();
  state.config = cfg;
  state.exts = cfg.exts;
  state.exclude = cfg.exclude;
  state.limit = cfg.limit;

  state.from = nowLocalInput(-24 * 3600 * 1000);
  state.to = nowLocalInput();
  $("#from").value = state.from;
  $("#to").value = state.to;

  renderChips();
  scheduleAutoRefresh(cfg.refreshSeconds);
  await runSearch();
}

new ResizeObserver(layoutCards).observe(grid);

void boot();
