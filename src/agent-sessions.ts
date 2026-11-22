import { Service, type AgentSession, type AgentPart, type AgentDateRange } from "./mygo";

const names: Record<string, string> = {
  codex: "Codex", claude: "Claude Code", "claude-desktop": "Claude Desktop", cindy: "Cindy",
  pi: "Pi", omp: "omp", opencode: "OpenCode", cline: "Cline", grok: "Grok Build",
  dsh: "DeepSeek Harness", zcode: "ZCode", openmausbot: "OpenMausBot", agy: "Antigravity CLI",
  antigravity: "Antigravity", gemini: "Gemini CLI", kimi: "Kimi Code", cursor: "Cursor",
  droid: "Droid", qoder: "Qoder", "qoder-cn": "Qoder CN", workbuddy: "WorkBuddy",
  "workbuddy-ai": "WorkBuddy AI", codebuddy: "CodeBuddy", hermes: "Hermes Agent",
  alma: "Alma", liveagent: "LiveAgent", "minimax-code": "MiniMax Code", raft: "Raft",
};
const agentName = (id: string) => names[id] || id;
const number = new Intl.NumberFormat("zh-CN");
const timeFormat = new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" });
const dateTime = (value: string) => value && timeFormat.format(new Date(value));
const groupTitle = (path: string) => path.split(/[\\/]/).filter(Boolean).at(-1) || path || "无工作目录";
const channelRank = (label: string) => label.startsWith("电脑端") ? 0 : label.startsWith("Telegram Bot") ? 1 : label.startsWith("Telegram 群") ? 2 : label.startsWith("Discord") ? 3 : 4;
const topicKey = (row: AgentSession) => row.topicId || row.topic || "";
const byId = <T extends HTMLElement = HTMLElement>(id: string) => document.getElementById(id) as T;
const wholeSession: AgentDateRange = { from: "", to: "" };
const sessionTitle = (row: AgentSession) =>
  row.agent === "cindy" && /^\[(?:UI language|渠道说明)\]/.test(row.title)
    ? row.topic || row.channel || "未命名会话"
    : row.title || "未命名会话";
const dateRule = (agent: string) => agent === "cindy"
  ? "按消息日期截取，可跨多个渠道和 Telegram 话题。"
  : agent === "all" ? "Cindy 按消息日期截取，其他 Agent 按会话创建日期筛选。"
    : "按会话创建日期筛选，匹配的会话保留完整内容。";

function localToday() {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(now.getDate()).padStart(2, "0")}`;
}

function dateLabel({ from, to }: AgentDateRange) {
  if (from === to) return from;
  if (!from) return `截至 ${to}`;
  if (!to) return `${from} 起`;
  return `${from} 至 ${to}`;
}

function node<K extends keyof HTMLElementTagNameMap>(tag: K, text = "", css = "") {
  const element = document.createElement(tag);
  element.textContent = text;
  element.className = css;
  return element;
}

function message(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function partNode(part: AgentPart) {
  const role = part.role === "user" ? "用户" : part.role === "assistant" ? "助手" : "工具";
  const kinds: Record<string, string> = { thinking: "思考", tool_use: "工具调用", tool_result: "工具结果", context: "上下文" };
  const kind = kinds[part.kind];
  const text = part.kind === "image" ? "[图片]" : part.text;
  const content = node("pre", part.cut ? `${text}\n… 另有 ${number.format(part.cut)} 字符` : text);
  if (kind) {
    const detail = node("details", "", "session-part auxiliary");
    detail.append(node("summary", [kind, part.name].filter(Boolean).join(" · ")), content);
    return detail;
  }
  const block = node("article", "", `session-part${part.role === "user" ? " user-part" : ""}`);
  block.append(node("strong", role), content);
  return block;
}

// Metadata blocks the agents inject around a message. The reading view drops
// them; an unclosed block also drops the rest, since it is always appended.
const metaTags = [
  "environment_context", "group_chat_context", "group_chat_attachments", "reply_context", "bot_persona",
  "cindy-host-image-references", "cindy-extra-writable-directories", "group_history_result",
  "user_instructions",
];

function stripMetaTags(text: string) {
  let out = text;
  for (const tag of metaTags) {
    out = out.replace(new RegExp(`<${tag}>[\\s\\S]*?</${tag}>`, "g"), "");
    out = out.replace(new RegExp(`<${tag}>[\\s\\S]*$`, "g"), "");
  }
  return out;
}

// parseUserText separates the injected channel note and quote from what the
// person actually typed, so the reading view can look like a chat.
function parseUserText(raw: string) {
  const source = raw.replace(/\r\n/g, "\n");
  const lines = source.split("\n");
  const marker = lines.findIndex((line) => line.trimStart().startsWith("[发言人]"));
  const sender = marker >= 0 ? (lines[marker]?.match(/^\[发言人\]\s*([^(@·]+)/)?.[1]?.trim() ?? "") : "";
  const quote = source.match(/<reply_context>([\s\S]*?)<\/reply_context>/)?.[1]?.trim() ?? "";
  let text = marker >= 0 ? lines.slice(marker + 1).join("\n") : source;
  text = stripMetaTags(text)
    .replace(/^\[UI language\] System note appended each turn\.[^\n]*\n*/, "")
    .replace(/^以上[^\n]{0,80}标签块内[\s\S]*?(?:\n[ \t]*\n|$)/gm, "")
    .replace(/^\[渠道说明\][^\n]*$/gm, "")
    .replace(/^\[发言人\][^\n]*$/gm, "")
    .trim();
  return { sender, quote, text };
}

function readingPart(part: AgentPart) {
  if ((part.role !== "user" && part.role !== "assistant") || (part.kind !== "text" && part.kind !== "image")) return;
  const row = node("div", "", `chat-row ${part.role === "user" ? "from-user" : "from-agent"}`);
  const bubble = node("div", "", "chat-bubble");
  if (part.kind === "image") {
    bubble.append(node("span", "[图片]", "chat-attachment"));
  } else if (part.role === "user") {
    const parsed = parseUserText(part.text);
    if (!parsed.text) return;
    if (parsed.sender) bubble.append(node("span", parsed.sender, "chat-sender"));
    if (parsed.quote) bubble.append(node("blockquote", parsed.quote, "chat-quote"));
    bubble.append(node("p", parsed.text));
  } else if (part.text.trim()) {
    bubble.append(node("p", part.text));
  }
  if (!bubble.childElementCount) return;
  if (part.cut) bubble.append(node("span", `… 另有 ${number.format(part.cut)} 字符`, "muted small"));
  row.append(bubble);
  return row;
}

export function bindSessions(copy: (value: string) => Promise<boolean>, resumeFiles: () => void) {
  const page = byId("sessionPage");
  const filesButton = byId<HTMLButtonElement>("showFiles");
  const sessionsButton = byId<HTMLButtonElement>("showSessions");
  const agent = byId<HTMLSelectElement>("sessionAgent");
  const search = byId<HTMLInputElement>("sessionSearch");
  const date = byId<HTMLInputElement>("sessionDate");
  const browseButton = byId<HTMLButtonElement>("browseSessions");
  const dateButton = byId<HTMLButtonElement>("findSessionsByDate");
  const dateForm = byId<HTMLFormElement>("sessionDateForm");
  const fromDate = byId<HTMLInputElement>("sessionFrom");
  const toDate = byId<HTMLInputElement>("sessionTo");
  const applyDates = byId<HTMLButtonElement>("applySessionDates");
  const mergeDates = byId<HTMLButtonElement>("mergeDatedSessions");
  const zipDates = byId<HTMLButtonElement>("zipDatedSessions");
  const refreshButton = byId<HTMLButtonElement>("sessionRefresh");
  const archiveButton = byId<HTMLButtonElement>("saveSessionDay");
  const archiveLink = byId<HTMLButtonElement>("showSessionArchive");
  const exportButton = byId<HTMLButtonElement>("exportSession");
  const exportDialog = byId<HTMLDialogElement>("sessionExportDialog");
  const exportFrom = byId<HTMLInputElement>("sessionExportFrom");
  const exportTo = byId<HTMLInputElement>("sessionExportTo");
  const cleanExport = byId<HTMLInputElement>("cleanSessionExport");
  const cleanView = byId<HTMLInputElement>("cleanSessionView");
  const toggleAll = byId<HTMLButtonElement>("toggleAllSessions");
  const classify = byId<HTMLInputElement>("cindyClassify");
  const sourceButton = byId<HTMLButtonElement>("showSessionSource");
  const more = byId<HTMLButtonElement>("moreSessions");
  const list = byId("sessionList");
  const layout = byId("sessionLayout");
  const reading = byId("sessionReading");
  const older = byId<HTMLButtonElement>("olderSessionMessages");
  const transcript = byId("sessionTranscript");
  const title = byId("sessionTitle");
  const count = byId("sessionCount");
  const feedback = byId("sessionFeedback");
  const pageSize = 100;
  let rows: AgentSession[] | undefined;
  let limit = pageSize;
  let selected: AgentSession | undefined;
  let selectedGroup: string | undefined;
  let before = 0;
  let loadingOlder = false;
  let dateMode = false;
  let appliedDates: AgentDateRange | undefined;
  let dateRows: AgentSession[] | undefined;
  let dateVersion = 0;
  let finding = false;
  let dateSkipped = 0;
  let exportRequest: { rows: AgentSession[]; label: string; format: "single" | "markdown" | "zip"; dates?: AgentDateRange };
  let archivePath = "";
  let exporting = false;
  let readVersion = 0;
  let listError = "";

  date.value = fromDate.value = toDate.value = localToday();

  function setPage(sessions: boolean) {
    document.body.classList.toggle("show-sessions", sessions);
    page.classList.toggle("hide", !sessions);
    for (const [button, active] of [[filesButton, !sessions], [sessionsButton, sessions]] as const) {
      button.classList.toggle("on", active);
      button.setAttribute("aria-pressed", String(active));
    }
  }

  function select(row?: AgentSession, focus = false) {
    selected = row;
    selectedGroup = undefined;
    ++readVersion;
    layout.classList.toggle("has-session", !!row);
    byId("sessionSelected").classList.toggle("hide", !row);
    byId("sessionEmpty").classList.toggle("hide", !!row);
    for (const item of list.querySelectorAll<HTMLButtonElement>(".session-row")) {
      item.setAttribute("aria-pressed", String(item.dataset.key === row?.key));
    }
    updateGroupSelection();
    if (!row) return;
    title.textContent = sessionTitle(row);
    byId("sessionDetailAgent").textContent = agentName(row.agent);
    const meta = byId("sessionMeta");
    meta.replaceChildren();
    for (const [label, value] of [
      ["会话 ID", row.id], ["话题", row.topic], ["话题 ID", row.topicId], ["工作目录", row.folder], ["源文件", row.path], ["模型", row.models.join("、")],
      ["开始时间", dateTime(row.started)], ["最后活动", dateTime(row.updated)],
      ["消息数", row.messages ? number.format(row.messages) : ""],
      ["源文件大小", row.size ? `${number.format(row.size)} 字节` : ""],
      ["Token", `输入 ${number.format(row.input)} · 输出 ${number.format(row.output)} · 缓存 ${number.format(row.cache)}`],
    ]) {
      if (value) meta.append(node("dt", label), node("dd", value));
    }
    sourceButton.disabled = !row.path;
    exportButton.disabled = !row.readable || exporting;
    byId("sessionMeta").parentElement!.removeAttribute("open");
    if (focus) title.focus({ preventScroll: true });
    void loadTranscript();
  }

  async function loadTranscript(prepend = false) {
    if (prepend && (loadingOlder || !before)) return;
    const row = selected;
    const clean = cleanView.checked;
    const version = prepend ? readVersion : ++readVersion;
    if (!prepend) {
      before = 0;
      older.classList.add("hide");
      transcript.classList.toggle("reading", clean);
      transcript.textContent = row?.readable ? "正在读取对话…" : "当前读取器仅支持该会话的元数据。";
    }
    if (!row?.readable) return;
    loadingOlder = older.disabled = true;
    older.textContent = "正在加载…";
    try {
      const result = await Service.agentTranscript(row.key, clean, appliedDates ?? wholeSession, before);
      if (version !== readVersion) return;
      const previousHeight = reading.scrollHeight;
      const previousTop = reading.scrollTop;
      const content = document.createDocumentFragment();
      for (const part of result.parts) {
        const item = clean ? readingPart(part) : partNode(part);
        if (item) content.append(item);
      }
      before = result.before;
      older.classList.toggle("hide", !before);
      if (prepend) transcript.prepend(content);
      else transcript.replaceChildren(content);
      if (!transcript.childElementCount) transcript.textContent = "本页没有可显示的正文。";
      else if (prepend && transcript.firstChild?.nodeType === Node.TEXT_NODE) transcript.firstChild.remove();
      else if (prepend && transcript.lastChild?.nodeType === Node.TEXT_NODE) transcript.lastChild.remove();
      reading.scrollTop = prepend ? previousTop + reading.scrollHeight - previousHeight : reading.scrollHeight;
    } catch (error) {
      if (version !== readVersion) return;
      if (prepend) {
        older.textContent = "加载失败，重试";
        older.title = message(error);
      } else transcript.textContent = message(error);
    } finally {
      if (version === readVersion) {
        loadingOlder = older.disabled = false;
        if (older.textContent === "正在加载…") {
          older.textContent = "加载更早消息";
          older.removeAttribute("title");
        }
      }
    }
  }

  function rowNode(row: AgentSession) {
    const item = node("button", "", "session-row");
    item.type = "button";
    item.dataset.key = row.key;
    item.setAttribute("aria-pressed", String(row.key === selected?.key));
    const timestamp = row.rangeLast || (dateMode ? row.started : row.updated);
    const summary = [agentName(row.agent), dateTime(timestamp), row.models[0], row.messages ? `${number.format(row.messages)} 条消息` : ""];
    item.append(node("strong", sessionTitle(row)), node("span", summary.filter(Boolean).join(" · "), "muted small"));
    item.onclick = () => select(row, true);
    return item;
  }

  function filteredRows() {
    const words = search.value.trim().toLowerCase().split(/\s+/).filter(Boolean);
    if (!agent.value) return [];
    return ((dateMode ? dateRows : rows) ?? []).filter((row) => {
      if (agent.value !== "all" && row.agent !== agent.value) return false;
      const haystack = `${sessionTitle(row)}\n${row.title}\n${row.folder}\n${row.channel ?? ""}\n${row.topic ?? ""}\n${row.topicId ?? ""}\n${row.id}\n${agentName(row.agent)}\n${row.models.join(" ")}`.toLowerCase();
      return words.every((word) => haystack.includes(word));
    });
  }

  function groupKeyFor(row: AgentSession) {
    return classify.checked && row.channel ? row.channel : row.folder;
  }

  function updateGroupSelection() {
    for (const group of list.querySelectorAll<HTMLDetailsElement>(".session-group")) {
      const active = group.dataset.folder === selectedGroup;
      group.dataset.selected = String(active);
      group.querySelector<HTMLButtonElement>(".session-group-export")!.classList.toggle("hide", !active);
    }
    byId("sessionEmpty").textContent = selectedGroup === undefined
      ? "选择一条会话查看内容"
      : "已选中分组，可导出为 ZIP。";
  }

  function render() {
    const matches = filteredRows();
    if (selected && !matches.some((row) => row.key === selected?.key)) select();
    if (selectedGroup !== undefined && !matches.some((row) => groupKeyFor(row) === selectedGroup)) selectedGroup = undefined;
    const picking = !!agent.value;
    browseButton.disabled = !picking;
    dateButton.disabled = !picking || agent.value === "all";
    browseButton.classList.toggle("on", !dateMode);
    dateButton.classList.toggle("on", dateMode);
    browseButton.setAttribute("aria-pressed", String(!dateMode));
    dateButton.setAttribute("aria-pressed", String(dateMode));
    dateForm.classList.toggle("hide", !dateMode);
    byId("sessionDateRule").textContent = dateRule(agent.value);
    byId("sessionFilters").classList.toggle("hide", !picking || (dateMode && !appliedDates));
    byId("cindyClassifyLabel").classList.toggle("hide", agent.value !== "all" && agent.value !== "cindy");
    byId("sessionDateActions").classList.toggle("hide", !dateMode || !matches.length);
    mergeDates.classList.toggle("hide", agent.value !== "cindy");
    applyDates.disabled = finding;
    const classifying = classify.checked;
    const collapsed = new Set(Array.from(list.querySelectorAll<HTMLDetailsElement>(".session-group:not([open])"), (group) => group.dataset.folder));
    const openedTopics = new Set(Array.from(list.querySelectorAll<HTMLDetailsElement>(".session-topic[open]"), (group) => group.dataset.folder));
    const groups = new Map<string, AgentSession[]>();
    const totals = new Map<string, number>();
    const matchingTopics = new Map<string, Set<string>>();
    for (const row of matches) {
      const key = groupKeyFor(row);
      totals.set(key, (totals.get(key) ?? 0) + 1);
      if (classifying && row.channel?.startsWith("Telegram 群") && topicKey(row)) {
        if (!matchingTopics.has(key)) matchingTopics.set(key, new Set());
        matchingTopics.get(key)!.add(topicKey(row));
      }
    }
    for (const row of matches.slice(0, limit)) {
      const key = groupKeyFor(row);
      const group = groups.get(key);
      if (group) group.push(row);
      else groups.set(key, [row]);
    }
    const ordered = [...groups];
    if (classifying) {
      ordered.sort((a, b) => {
        const left = a[1][0]?.channel, right = b[1][0]?.channel;
        if (left && right) return channelRank(left) - channelRank(right) || left.localeCompare(right, "zh");
        return left ? -1 : right ? 1 : 0;
      });
    }
    list.replaceChildren();
    for (const [groupKey, group] of ordered) {
      const panel = node("details", "", "session-group");
      panel.dataset.folder = groupKey;
      panel.open = !collapsed.has(groupKey);
      const heading = node("summary", "", "session-group-title");
      const main = node("span", "", "session-group-main");
      const byChannel = classifying && !!group[0]?.channel;
      const folders = new Set(group.map((row) => row.folder));
      main.append(node("strong", byChannel ? groupKey : groupTitle(groupKey)));
      main.append(node("span", byChannel ? (folders.size === 1 ? group[0]!.folder : `${number.format(folders.size)} 个目录`) : groupKey, "muted small"));
      const download = node("button", "导出 ZIP", "btn tiny ghost session-group-export hide");
      download.type = "button";
      download.disabled = exporting;
      download.title = "将该组匹配的会话导出为 ZIP";
      download.setAttribute("aria-label", `导出 ${byChannel ? groupKey : groupTitle(groupKey)} 为 ZIP`);
      download.onclick = (event) => {
        event.preventDefault();
        event.stopPropagation();
        const items = filteredRows().filter((row) => groupKeyFor(row) === groupKey);
        openExport(items, byChannel ? groupKey : groupTitle(groupKey), "zip");
      };
      heading.addEventListener("click", () => {
        select();
        selectedGroup = groupKey;
        updateGroupSelection();
      });
      const sessions = number.format(totals.get(groupKey) ?? group.length);
      const topicsFound = matchingTopics.get(groupKey);
      const allTopics = topicsFound && new Set((rows ?? []).filter((row) => row.channel === groupKey).map(topicKey).filter(Boolean));
      const topicCount = allTopics && topicsFound
        ? topicsFound.size === allTopics.size ? `${number.format(allTopics.size)} 个话题`
          : `${number.format(topicsFound.size)} / ${number.format(allTopics.size)} 个话题`
        : "";
      const groupCount = node("span", topicCount || sessions, "session-group-count");
      groupCount.title = `${sessions} 个匹配会话`;
      heading.append(main, groupCount, download);
      panel.append(heading);
      const topics = new Map<string, AgentSession[]>();
      let hasTopics = false;
      for (const row of group) {
        const topic = topicKey(row);
        if (topic) hasTopics = true;
        const items = topics.get(topic);
        if (items) items.push(row);
        else topics.set(topic, [row]);
      }
      if (hasTopics) {
        for (const [topic, items] of [...topics].sort((a, b) => Number(!a[0]) - Number(!b[0]))) {
          const key = `${groupKey}\u0000${topic}`;
          const topicPanel = node("details", "", "session-topic");
          topicPanel.dataset.folder = key;
          topicPanel.open = openedTopics.has(key);
          const topicHeading = node("summary", "", "session-topic-title");
          topicHeading.append(node("strong", items[0]!.topic || "未识别话题"), node("span", number.format(items.length), "session-group-count"));
          topicPanel.append(topicHeading);
          for (const row of items) topicPanel.append(rowNode(row));
          panel.append(topicPanel);
        }
      } else {
        for (const row of group) panel.append(rowNode(row));
      }
      list.append(panel);
    }
    more.classList.toggle("hide", matches.length <= limit);
    const skipped = dateMode && dateSkipped ? ` · ${number.format(dateSkipped)} 个会话读取失败` : "";
    count.textContent = (dateMode ? appliedDates : rows && picking)
      ? `显示 ${number.format(Math.min(limit, matches.length))} / ${number.format(matches.length)} 个会话${skipped}` : "";
    if (!matches.length) {
      const empty = listError || (!rows ? "正在查找本地会话…"
        : !rows.length ? "尚未发现本地 Agent 会话。"
          : !picking ? "先选择一个 Agent，再选择查找方式。"
            : finding ? "正在按日期查找会话…"
              : dateMode && !appliedDates ? "选择日期，再点击「查找会话」。" : "没有匹配的会话。");
      list.append(node("div", empty, "session-empty muted"));
    }
    updateGroupSelection();
    updateToggleAll();
  }

  function updateToggleAll() {
    const panels = Array.from(list.querySelectorAll<HTMLDetailsElement>(".session-group, .session-topic"));
    const open = panels.some((panel) => panel.open);
    toggleAll.textContent = open ? "全部折叠" : "全部展开";
    toggleAll.disabled = !panels.length;
  }

  function clearFeedback() {
    feedback.textContent = "";
    archivePath = "";
    archiveLink.classList.add("hide");
  }

  function setExporting(value: boolean) {
    exporting = value;
    mergeDates.disabled = zipDates.disabled = value;
    exportButton.disabled = value || !selected?.readable;
    for (const button of list.querySelectorAll<HTMLButtonElement>(".session-group-export")) button.disabled = value;
  }

  function openExport(items: AgentSession[], label: string, format: typeof exportRequest.format) {
    if (exporting || !items.length) return;
    const dates = appliedDates;
    exportRequest = { rows: items, label, format, dates };
    byId("sessionExportTitle").textContent = format === "zip" ? "导出 ZIP"
      : format === "markdown" ? "合并 Markdown" : "导出 Markdown";
    const scope = format === "single" ? sessionTitle(items[0]!) : `${label} · ${number.format(items.length)} 个会话`;
    byId("sessionExportScope").textContent = dates ? `${scope} · ${dateLabel(dates)}` : scope;
    const agents = new Set(items.map((row) => row.agent));
    byId("sessionExportRule").textContent = `${dateRule(agents.size === 1 ? items[0]!.agent : "all")} 日期留空时导出完整会话。`;
    byId("sessionExportDates").classList.toggle("hide", !!dates);
    exportFrom.value = exportTo.value = "";
    exportDialog.showModal();
  }

  function readDates(from: HTMLInputElement, to: HTMLInputElement, required: boolean): AgentDateRange | undefined {
    from.setCustomValidity("");
    to.setCustomValidity("");
    if (required && !from.value && !to.value) from.setCustomValidity("请选择开始日期或结束日期");
    if (from.value && to.value && from.value > to.value) to.setCustomValidity("结束日期不能早于开始日期");
    if (!from.reportValidity() || !to.reportValidity()) return;
    return { from: from.value, to: to.value };
  }

  async function runExport(dates: AgentDateRange) {
    const request = exportRequest;
    const clean = cleanExport.checked;
    const { rows: items, label, format } = request;
    setExporting(true);
    clearFeedback();
    try {
      if (format === "single") {
        const row = items[0]!;
        archivePath = await Service.exportAgentSession(row.key, agentName(row.agent), clean, dates);
        if (archivePath) feedback.textContent = "已导出 Markdown。";
      } else {
        const range = dateLabel(dates);
        const result = await Service.exportAgentSessions(
          range ? `${label} ${range}` : label,
          items.map((row) => ({
            key: row.key,
            name: sessionTitle(row),
            folder: format === "markdown" ? (row.channel ?? "") : [row.channel && dateMode ? row.channel : "", row.topic]
              .filter(Boolean).join(" · "),
          })),
          clean, format, dates,
        );
        if (!result.matched) return;
        archivePath = result.path;
        const empty = result.empty ? ` · ${number.format(result.empty)} 个没有可导出内容` : "";
        const skipped = result.skipped ? ` · ${number.format(result.skipped)} 个仅有元数据或读取失败` : "";
        feedback.textContent = archivePath
          ? `已导出 ${number.format(result.exported)} / ${number.format(result.matched)} 个会话${empty}${skipped}。`
          : `没有可导出的会话${empty}${skipped}。`;
      }
      archiveLink.textContent = "查看导出位置";
      archiveLink.classList.toggle("hide", !archivePath);
    } catch (error) {
      feedback.textContent = message(error);
    } finally {
      setExporting(false);
    }
  }

  function resetDateResults() {
    ++dateVersion;
    finding = false;
    dateRows = appliedDates = undefined;
    dateSkipped = 0;
    select();
    clearFeedback();
  }

  async function findDates() {
    const dates = readDates(fromDate, toDate, true);
    if (!dates) return;
    resetDateResults();
    const version = dateVersion;
    finding = true;
    criteriaChanged();
    try {
      const result = await Service.agentDateSessions(agent.value, dates);
      if (version !== dateVersion) return;
      appliedDates = dates;
      dateRows = result.rows;
      dateSkipped = result.skipped;
    } catch (error) {
      if (version !== dateVersion) return;
      feedback.textContent = message(error);
    } finally {
      if (version === dateVersion) {
        finding = false;
        render();
      }
    }
  }

  async function refresh() {
    if (refreshButton.disabled) return;
    refreshButton.disabled = true;
    count.textContent = "正在扫描本地会话…";
    listError = "";
    if (!rows) render();
    try {
      rows = await Service.agentSessions();
      const counts = new Map<string, number>();
      for (const row of rows) counts.set(row.agent, (counts.get(row.agent) ?? 0) + 1);
      const previous = agent.value;
      agent.replaceChildren(new Option("选择 Agent", ""), new Option(`全部 Agent · ${number.format(rows.length)}`, "all"));
      for (const [key, total] of [...counts].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))) {
        agent.add(new Option(`${agentName(key)} · ${number.format(total)}`, key));
      }
      agent.value = previous === "all" || counts.has(previous) ? previous : "";
      if (!agent.value) {
        dateMode = false;
        resetDateResults();
      }
      render();
      if (dateMode && appliedDates) await findDates();
      else if (selected) select(rows.find((row) => row.key === selected?.key));
    } catch (error) {
      listError = message(error);
      render();
      if (rows?.length) feedback.textContent = listError;
    } finally {
      refreshButton.disabled = false;
    }
  }

  function criteriaChanged() {
    limit = pageSize;
    render();
    list.parentElement!.scrollTop = 0;
  }

  sessionsButton.onclick = () => {
    setPage(true);
    if (!rows) void refresh();
  };
  filesButton.onclick = () => {
    if (page.classList.contains("hide")) return;
    setPage(false);
    resumeFiles();
  };
  search.oninput = criteriaChanged;
  agent.onchange = () => {
    if (agent.value === "all" || !agent.value) dateMode = false;
    search.value = "";
    resetDateResults();
    criteriaChanged();
  };
  for (const [button, byDate] of [[browseButton, false], [dateButton, true]] as const) {
    button.onclick = () => {
      if (dateMode === byDate) return;
      dateMode = byDate;
      search.value = "";
      resetDateResults();
      criteriaChanged();
    };
  }
  classify.onchange = () => { selectedGroup = undefined; criteriaChanged(); };
  cleanView.onchange = () => void loadTranscript();
  older.onclick = () => void loadTranscript(true);
  reading.addEventListener("scroll", () => {
    if (reading.scrollTop <= 24 && older.textContent === "加载更早消息") void loadTranscript(true);
  }, { passive: true });
  for (const input of [fromDate, toDate, exportFrom, exportTo]) {
    input.oninput = () => {
      const isQuery = input === fromDate || input === toDate;
      for (const field of isQuery ? [fromDate, toDate] : [exportFrom, exportTo]) field.setCustomValidity("");
      if (isQuery) {
        resetDateResults();
        criteriaChanged();
      }
    };
  }
  byId("sessionToday").onclick = () => {
    fromDate.value = toDate.value = localToday();
    void findDates();
  };
  dateForm.noValidate = true;
  dateForm.onsubmit = (event) => { event.preventDefault(); void findDates(); };
  zipDates.onclick = () => openExport(filteredRows(), agentName(agent.value), "zip");
  mergeDates.onclick = () => openExport(filteredRows(), agentName(agent.value), "markdown");
  refreshButton.onclick = () => void refresh();
  more.onclick = () => { limit += pageSize; render(); };
  toggleAll.onclick = () => {
    const panels = Array.from(list.querySelectorAll<HTMLDetailsElement>(".session-group, .session-topic"));
    const open = panels.some((panel) => panel.open);
    for (const panel of panels) panel.open = !open;
    updateToggleAll();
  };
  list.addEventListener("toggle", updateToggleAll, true);
  byId("closeSessionDetail").onclick = () => {
    const key = selected?.key;
    select();
    Array.from(list.querySelectorAll<HTMLButtonElement>(".session-row")).find((item) => item.dataset.key === key)?.focus();
  };

  sourceButton.onclick = async () => {
    if (selected?.path) feedback.textContent = await Service.reveal(selected.path);
  };
  exportButton.onclick = () => {
    if (selected) openExport([selected], agentName(selected.agent), "single");
  };
  byId("sessionExportToday").onclick = () => { exportFrom.value = exportTo.value = localToday(); };
  byId("cancelSessionExport").onclick = () => exportDialog.close();
  const exportForm = byId<HTMLFormElement>("sessionExportForm");
  exportForm.noValidate = true;
  exportForm.onsubmit = (event) => {
    event.preventDefault();
    const dates = exportRequest.dates ?? readDates(exportFrom, exportTo, false);
    if (!dates) return;
    exportDialog.close();
    void runExport(dates);
  };

  const archiveDialog = byId<HTMLDialogElement>("sessionArchiveDialog");
  const archiveFeedback = byId("sessionArchiveFeedback");
  byId("openSessionArchive").onclick = () => {
    archiveFeedback.textContent = "";
    archiveDialog.showModal();
  };
  archiveButton.onclick = async () => {
    if (!date.reportValidity()) return;
    archiveButton.disabled = date.disabled = true;
    clearFeedback();
    archiveFeedback.textContent = "正在归档完整会话…";
    try {
      const result = await Service.saveAgentArchive(agent.value || "all", date.value.replaceAll("-", ""));
      archivePath = result.path;
      archiveFeedback.textContent = !result.matched ? "当天没有匹配的会话。" : result.exported === result.matched
        ? `已归档 ${result.exported} 个会话。`
        : `已导出 ${result.exported} / ${result.matched} 个会话的正文，详情见归档中的 manifest.json。`;
      archiveLink.textContent = "查看归档位置";
      archiveLink.classList.toggle("hide", !archivePath);
      if (archivePath) {
        feedback.textContent = archiveFeedback.textContent;
        archiveDialog.close();
      }
    } catch (error) {
      archiveFeedback.textContent = message(error);
    } finally {
      archiveButton.disabled = date.disabled = false;
    }
  };

  byId("copySessionCLI").onclick = async () => {
    if (!date.reportValidity()) return;
    try {
      const arg = `ff-${agent.value || "all"}-${date.value.replaceAll("-", "")}`;
      const path = (await Service.sessionCLI()).replaceAll("'", "''");
      archiveFeedback.textContent = await copy(`& '${path}' ${arg}`) ? "已复制 PowerShell 命令。" : "无法写入剪贴板。";
    } catch (error) {
      archiveFeedback.textContent = message(error);
    }
  };
  archiveLink.onclick = async () => {
    if (!archivePath) return;
    const error = await Service.reveal(archivePath);
    if (error) feedback.textContent = error;
  };
  document.addEventListener("keydown", (event) => {
    if (page.classList.contains("hide") || exportDialog.open || archiveDialog.open || !(event.ctrlKey || event.metaKey)) return;
    if (event.key.toLowerCase() === "f") {
      event.preventDefault();
      search.focus();
    } else if (event.key.toLowerCase() === "r") {
      event.preventDefault();
      void refresh();
    }
  });
}
