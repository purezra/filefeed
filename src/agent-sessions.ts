import { Service, type AgentSession, type AgentPart, type AgentTranscript } from "./mygo";

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
const byId = <T extends HTMLElement = HTMLElement>(id: string) => document.getElementById(id) as T;

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
    .replace(/^以上[^\n]{0,80}标签块内[\s\S]*?(?:\n[ \t]*\n|$)/gm, "")
    .replace(/^\[渠道说明\][^\n]*$/gm, "")
    .replace(/^\[发言人\][^\n]*$/gm, "")
    .trim();
  return { sender, quote, text };
}

function readingPart(part: AgentPart, seen: Set<string>) {
  if ((part.role !== "user" && part.role !== "assistant") || (part.kind !== "text" && part.kind !== "image")) return;
  const row = node("div", "", `chat-row ${part.role === "user" ? "from-user" : "from-agent"}`);
  const bubble = node("div", "", "chat-bubble");
  if (part.kind === "image") {
    bubble.append(node("span", "[图片]", "chat-attachment"));
  } else if (part.role === "user") {
    const parsed = parseUserText(part.text);
    if (!parsed.text) return;
    // Cindy replays a recorded turn later in the same file; the quote and
    // sender make the copies identical without hiding a short repeat.
    const signature = `${parsed.sender}\u0000${parsed.quote}\u0000${parsed.text}`;
    if (seen.has(signature)) return;
    seen.add(signature);
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
  const refreshButton = byId<HTMLButtonElement>("sessionRefresh");
  const archiveButton = byId<HTMLButtonElement>("saveSessionDay");
  const archiveLink = byId<HTMLButtonElement>("showSessionArchive");
  const exportButton = byId<HTMLButtonElement>("exportSession");
  const cleanExport = byId<HTMLInputElement>("cleanSessionExport");
  const zipExport = byId<HTMLInputElement>("zipSessionExport");
  const cleanView = byId<HTMLInputElement>("cleanSessionView");
  const toggleAll = byId<HTMLButtonElement>("toggleAllSessions");
  const classify = byId<HTMLInputElement>("cindyClassify");
  const sourceButton = byId<HTMLButtonElement>("showSessionSource");
  const more = byId<HTMLButtonElement>("moreSessions");
  const list = byId("sessionList");
  const layout = byId("sessionLayout");
  const detail = byId("sessionDetail");
  const transcript = byId("sessionTranscript");
  const title = byId("sessionTitle");
  const count = byId("sessionCount");
  const feedback = byId("sessionFeedback");
  const pageSize = 100;
  let rows: AgentSession[] | undefined;
  let limit = pageSize;
  let selected: AgentSession | undefined;
  let currentTranscript: AgentTranscript | undefined;
  let archivePath = "";
  let exporting = false;
  let readVersion = 0;
  let listError = "";

  const now = new Date();
  now.setMinutes(now.getMinutes() - now.getTimezoneOffset());
  date.value = now.toISOString().slice(0, 10);

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
    const version = ++readVersion;
    layout.classList.toggle("has-session", !!row);
    byId("sessionSelected").classList.toggle("hide", !row);
    byId("sessionEmpty").classList.toggle("hide", !!row);
    for (const item of list.querySelectorAll<HTMLButtonElement>(".session-row")) {
      item.setAttribute("aria-pressed", String(item.dataset.key === row?.key));
    }
    if (!row) return;
    title.textContent = row.title || "未命名会话";
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
    currentTranscript = undefined;
    transcript.classList.toggle("reading", cleanView.checked);
    transcript.textContent = row.readable ? "正在读取对话…" : "已识别该会话的元数据。当前读取器暂无法显示正文。";
    detail.scrollTop = 0;
    if (focus) title.focus({ preventScroll: true });
    if (!row.readable) return;
    void Service.agentTranscript(row.key).then((result) => {
      if (version !== readVersion) return;
      currentTranscript = result;
      renderTranscript();
    }).catch((error: unknown) => {
      if (version !== readVersion) return;
      currentTranscript = undefined;
      transcript.textContent = message(error);
    });
  }

  function renderTranscript() {
    const result = currentTranscript;
    if (!result) return;
    const content = document.createDocumentFragment();
    let shown = 0;
    if (result.cut) content.append(node("p", "当前为长会话预览，导出 Markdown 可查看完整正文。", "session-preview-note"));
    if (cleanView.checked) {
      const seen = new Set<string>();
      for (const part of result.parts) {
        const item = readingPart(part, seen);
        if (item) {
          content.append(item);
          shown++;
        }
      }
    } else {
      for (const part of result.parts) {
        content.append(partNode(part));
        shown++;
      }
    }
    transcript.classList.toggle("reading", cleanView.checked);
    if (shown) transcript.replaceChildren(content);
    else transcript.textContent = "没有可显示的对话内容。";
  }

  function rowNode(row: AgentSession) {
    const item = node("button", "", "session-row");
    item.type = "button";
    item.dataset.key = row.key;
    item.setAttribute("aria-pressed", String(row.key === selected?.key));
    const summary = [agentName(row.agent), dateTime(row.updated), row.models[0], row.messages ? `${number.format(row.messages)} 条消息` : ""];
    item.append(node("strong", row.title || "未命名会话"), node("span", summary.filter(Boolean).join(" · "), "muted small"));
    item.onclick = () => select(row, true);
    return item;
  }

  function filteredRows() {
    const words = search.value.trim().toLowerCase().split(/\s+/).filter(Boolean);
    return (rows ?? []).filter((row) => {
      if (agent.value !== "all" && row.agent !== agent.value) return false;
      const haystack = `${row.title}\n${row.folder}\n${row.channel ?? ""}\n${row.topic ?? ""}\n${row.topicId ?? ""}\n${row.id}\n${agentName(row.agent)}\n${row.models.join(" ")}`.toLowerCase();
      return words.every((word) => haystack.includes(word));
    });
  }

  function groupKeyFor(row: AgentSession) {
    return classify.checked && row.channel ? row.channel : row.folder;
  }

  function render() {
    const matches = filteredRows();
    if (selected && !matches.some((row) => row.key === selected?.key)) select();
    const classifying = classify.checked;
    const collapsed = new Set(Array.from(list.querySelectorAll<HTMLDetailsElement>(".session-group:not([open])"), (group) => group.dataset.folder));
    const openedTopics = new Set(Array.from(list.querySelectorAll<HTMLDetailsElement>(".session-topic[open]"), (group) => group.dataset.folder));
    const groups = new Map<string, AgentSession[]>();
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
      const download = node("button", "导出", "btn tiny ghost session-group-export");
      download.type = "button";
      download.disabled = exporting;
      download.title = "导出该组全部会话";
      download.setAttribute("aria-label", `导出 ${byChannel ? groupKey : groupTitle(groupKey)} 下的全部会话`);
      download.onclick = (event) => {
        event.preventDefault();
        event.stopPropagation();
        void exportGroup(groupKey);
      };
      heading.append(main, node("span", number.format(group.length), "session-group-count"), download);
      panel.append(heading);
      const topics = new Map<string, AgentSession[]>();
      let hasTopics = false;
      for (const row of group) {
        const topic = row.topic ?? "";
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
          topicHeading.append(node("strong", topic || "未识别话题"), node("span", number.format(items.length), "session-group-count"));
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
    count.textContent = rows ? `显示 ${number.format(Math.min(limit, matches.length))} / ${number.format(matches.length)} 个会话` : "";
    if (!matches.length) {
      const empty = listError || (rows ? rows.length ? "没有匹配的会话。" : "尚未发现本地 Agent 会话。" : "正在查找本地会话…");
      list.append(node("div", empty, "session-empty muted"));
    }
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
    cleanExport.disabled = zipExport.disabled = value;
    exportButton.disabled = value || !selected?.readable;
    for (const button of list.querySelectorAll<HTMLButtonElement>(".session-group-export")) button.disabled = value;
  }

  async function exportGroup(groupKey: string) {
    if (exporting) return;
    const group = filteredRows().filter((row) => groupKeyFor(row) === groupKey);
    if (!group.length) return;
    setExporting(true);
    archivePath = "";
    archiveLink.classList.add("hide");
    feedback.textContent = `正在导出 ${number.format(group.length)} 个会话…`;
    try {
      const label = classify.checked && group[0]?.channel ? groupKey : groupTitle(groupKey);
      const result = await Service.exportAgentSessions(
        label,
        group.map((row) => ({ key: row.key, name: row.title || row.id, folder: classify.checked ? (row.topic ?? "") : "" })),
        cleanExport.checked,
        zipExport.checked,
      );
      if (!result.path) {
        feedback.textContent = "";
        return;
      }
      archivePath = result.path;
      const skipped = result.skipped ? `，${number.format(result.skipped)} 个仅元数据或读取失败` : "";
      feedback.textContent = `已导出 ${number.format(result.exported)} / ${number.format(result.matched)} 个会话${skipped}。`;
      archiveLink.textContent = "查看导出位置";
      archiveLink.classList.remove("hide");
    } catch (error) {
      feedback.textContent = message(error);
    } finally {
      setExporting(false);
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
      agent.replaceChildren(new Option(`全部 Agent · ${number.format(rows.length)}`, "all"));
      for (const [key, total] of [...counts].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))) {
        agent.add(new Option(`${agentName(key)} · ${number.format(total)}`, key));
      }
      agent.value = counts.has(previous) ? previous : "all";
      render();
      if (selected) select(rows.find((row) => row.key === selected?.key));
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
  agent.onchange = () => { clearFeedback(); criteriaChanged(); };
  classify.onchange = criteriaChanged;
  cleanView.onchange = () => renderTranscript();
  date.onchange = clearFeedback;
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
  exportButton.onclick = async () => {
    if (!selected || exporting) return;
    setExporting(true);
    clearFeedback();
    try {
      const path = await Service.exportAgentSession(selected.key, agentName(selected.agent), cleanExport.checked);
      if (path) {
        archivePath = path;
        feedback.textContent = "已导出 Markdown。";
        archiveLink.textContent = "查看文件位置";
        archiveLink.classList.remove("hide");
      }
    } catch (error) {
      feedback.textContent = message(error);
    } finally {
      setExporting(false);
    }
  };

  archiveButton.onclick = async () => {
    if (!date.reportValidity()) return;
    archiveButton.disabled = agent.disabled = date.disabled = true;
    clearFeedback();
    feedback.textContent = "正在保存当天的会话…";
    try {
      const result = await Service.saveAgentArchive(agent.value, date.value.replaceAll("-", ""));
      archivePath = result.path;
      feedback.textContent = !result.matched ? "当天没有匹配的会话。" : result.exported === result.matched
        ? `已归档 ${result.exported} 个会话。`
        : `已导出 ${result.exported} / ${result.matched} 个会话的正文，详情见归档中的 manifest.json。`;
      archiveLink.textContent = "查看归档位置";
      archiveLink.classList.toggle("hide", !archivePath);
    } catch (error) {
      feedback.textContent = message(error);
    } finally {
      archiveButton.disabled = agent.disabled = date.disabled = false;
    }
  };

  byId("copySessionCLI").onclick = async () => {
    if (!date.reportValidity()) return;
    try {
      const arg = `ff-${agent.value}-${date.value.replaceAll("-", "")}`;
      const path = (await Service.sessionCLI()).replaceAll("'", "''");
      feedback.textContent = await copy(`& '${path}' ${arg}`) ? "已复制 PowerShell 命令。" : "无法写入剪贴板。";
    } catch (error) {
      feedback.textContent = message(error);
    }
  };
  archiveLink.onclick = async () => {
    if (!archivePath) return;
    const error = await Service.reveal(archivePath);
    if (error) feedback.textContent = error;
  };
  document.addEventListener("keydown", (event) => {
    if (page.classList.contains("hide") || !(event.ctrlKey || event.metaKey)) return;
    if (event.key.toLowerCase() === "f") {
      event.preventDefault();
      search.focus();
    } else if (event.key.toLowerCase() === "r") {
      event.preventDefault();
      void refresh();
    }
  });
}
