import { renderMarkdown, typesetMath } from "./markdown.js";

const root = document.getElementById("feed-root");

const fetchJSON = async (url) => {
  const res = await fetch(url);
  if (!res.ok) {
    throw new Error(`Request failed: ${res.status}`);
  }
  return res.json();
};

const escapeHTML = (value = "") =>
  String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\"/g, "&quot;")
    .replace(/'/g, "&#39;");

const formatDateTime = (iso) => {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toLocaleString();
};

const summarizeText = (text) => {
  if (!text) return "";
  const cleaned = String(text).replace(/[#>*_`\\-]/g, " ").replace(/\s+/g, " ").trim();
  if (!cleaned) return "";
  if (cleaned.length <= 160) return cleaned;
  return `${cleaned.slice(0, 160)}…`;
};

const renderTools = (calls = [], responses = []) => {
  if (!calls.length && !responses.length) return "";
  const callHTML = calls.length
    ? `<div class="daily-label">Tool Calls</div><div class="md">${renderMarkdown(calls.map((c) => `- \`${c}\``).join("\n"))}</div>`
    : "";
  const respHTML = responses.length
    ? `<div class="daily-label">Tool Responses</div><div class="md">${renderMarkdown(responses.join("\n\n---\n\n"))}</div>`
    : "";
  return `${callHTML}${respHTML}`;
};

const renderEvent = (ev) => {
  const who = ev.agent_name || ev.agent_id || "agent";
  const action = ev.action || "action";
  const when = formatDateTime(ev.sim_time || ev.timestamp);
  const tick = Number.isFinite(ev.tick) ? ` • tick ${ev.tick}` : "";
  const preview = summarizeText(ev.response || ev.prompt || "");

  return `
    <div class="daily-entry event-entry">
      <div class="daily-header">
        <span class="daily-time">${escapeHTML(when)}${escapeHTML(tick)}</span>
        <div class="daily-summary">${escapeHTML(`${who} · ${action}`)}</div>
      </div>
      ${preview ? `<div class="post-meta">${escapeHTML(preview)}</div>` : ""}
      <details class="event-details">
        <summary>Open</summary>
        ${
          ev.prompt
            ? `<div class="daily-label">Prompt</div><div class="md">${renderMarkdown(ev.prompt)}</div>`
            : ""
        }
        ${
          ev.response
            ? `<div class="daily-label">Response</div><div class="md">${renderMarkdown(ev.response)}</div>`
            : ""
        }
        ${renderTools(ev.tool_calls || [], ev.tool_responses || [])}
      </details>
    </div>
  `;
};

const init = async () => {
  try {
    const params = new URLSearchParams(window.location.search);
    const limit = params.get("limit") || "200";
    const log = params.get("log") || "";

    const url = new URL("/api/feed", window.location.origin);
    url.searchParams.set("limit", limit);
    if (log) url.searchParams.set("log", log);

    const feed = await fetchJSON(url.toString());
    const events = feed.events || [];

    root.innerHTML = `
      <section class="feed-section">
        <h3>Global Feed</h3>
        <div class="post-meta">Log: <code>${escapeHTML(feed.log || "")}</code> • events: ${events.length}</div>
      </section>
      <section class="feed-section">
        ${events.length ? events.map(renderEvent).join("") : `<div class="empty">No events found.</div>`}
      </section>
    `;
    typesetMath(root);
  } catch (err) {
    root.innerHTML = `<div class="empty">${escapeHTML(err.message)}</div>`;
  }
};

init();
