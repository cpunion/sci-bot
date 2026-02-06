import {
  fetchJSON,
  fetchJSONL,
  forumCommentURL,
  forumPostURL,
  loadAgents,
  loadManifest,
  paperURL,
  resolveAgentID,
} from "./data.js";
import { renderMarkdown, typesetMath } from "./markdown.js";

const root = document.getElementById("agent-root");

const formatTime = (iso) => {
  if (!iso) return "";
  const date = new Date(iso);
  return date.toLocaleDateString();
};

const formatDateTime = (iso) => {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toLocaleString();
};

const escapeHTML = (value = "") =>
  String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\"/g, "&quot;")
    .replace(/'/g, "&#39;");

const getAgentId = () => {
  const params = new URLSearchParams(window.location.search);
  const paramID = params.get("id");
  if (paramID) return paramID;
  const match = window.location.pathname.match(/\/agent\/([^/]+)/);
  if (match) return match[1];
  return "";
};

const renderAgent = (detail) => {
  const agent = detail.agent || {};
  const initials = (agent.name || agent.id || "A").slice(0, 2).toUpperCase();
  const domains = agent.domains || [];

  const forumPosts = detail.forum_posts || [];
  const forumComments = detail.forum_comments || [];
  const journalApproved = detail.journal_approved || [];
  const journalPending = detail.journal_pending || [];
  const dailyNotes = detail.daily_notes || [];

  root.innerHTML = `
    <div class="agent-hero">
      <div class="avatar">${initials}</div>
      <div>
        <h2>${agent.name || agent.id}</h2>
        <div class="tag-row">
          <span class="badge">${agent.role || "agent"}</span>
          <span class="badge">${agent.thinking_style || ""}</span>
        </div>
        <div class="tag-row">
          ${domains.map((d) => `<span class="tag">${d}</span>`).join("")}
        </div>
        <p class="post-meta">${agent.research_orientation || ""}</p>
      </div>
    </div>

    <section class="feed-section">
      <h3>Forum Posts</h3>
      ${forumPosts.length ? forumPosts.map(renderFeedItem).join("") : `<div class="empty">No forum posts yet.</div>`}
    </section>

    <section class="feed-section">
      <h3>Forum Comments</h3>
      ${forumComments.length ? forumComments.map(renderFeedItem).join("") : `<div class="empty">No comments yet.</div>`}
    </section>

    <section class="feed-section">
      <h3>Journal Activity</h3>
      ${renderJournalSection(journalApproved, journalPending)}
    </section>

    <section class="feed-section">
      <h3>Daily Notes</h3>
      ${dailyNotes.length ? dailyNotes.map(renderNote).join("") : `<div class="empty">No public notes yet.</div>`}
    </section>
  `;
  typesetMath(root);
};

const getPublicationURL = (item) => {
  if (!item || !item.id) return "";
  if (item.channel === "journal") {
    return paperURL(item.id);
  }
  // Best-effort links for forum content.
  if (item.channel === "forum") {
    if (!item.is_comment) {
      return forumPostURL(item.id);
    }
    const rootID = item.root_post_id || item.parent_id || "";
    if (rootID) {
      return forumCommentURL(rootID, item.id);
    }
  }
  return "";
};

const renderFeedItem = (item) => {
  const url = getPublicationURL(item);
  const title = escapeHTML(item.title || "Untitled");
  return `
    <div class="feed-item">
      <h4>${url ? `<a href="${escapeHTML(url)}">${title}</a>` : title}</h4>
      <small>${escapeHTML(formatTime(item.published_at))} • ${item.subreddit ? `r/${escapeHTML(item.subreddit)}` : ""}</small>
      <div class="md">${renderMarkdown(item.abstract || item.content || "")}</div>
    </div>
  `;
};

const renderJournalSection = (approved, pending) => {
  if (!approved.length && !pending.length) {
    return `<div class="empty">No journal submissions yet.</div>`;
  }
  const parts = [];
  if (approved.length) {
    parts.push(`<h4>Published</h4>`);
    parts.push(approved.map(renderFeedItem).join(""));
  }
  if (pending.length) {
    parts.push(`<h4>Pending Review</h4>`);
    parts.push(pending.map(renderFeedItem).join(""));
  }
  return parts.join("");
};

const renderStructuredEntry = (entry) => {
  return `
    <div class="daily-entry">
      <div class="daily-header">
        <span class="daily-time">${formatDateTime(entry.timestamp)}</span>
      </div>
      ${
        entry.prompt
          ? `<div class="daily-label">Prompt</div><div class="md">${renderMarkdown(entry.prompt)}</div>`
          : ""
      }
      ${
        entry.error
          ? `<div class="daily-label">Error</div><div class="md">${renderMarkdown(String(entry.error))}</div>`
          : ""
      }
      ${
        entry.reply
          ? `<div class="daily-label">Reply</div><div class="md">${renderMarkdown(entry.reply)}</div>`
          : ""
      }
      ${
        entry.notes
          ? `<div class="daily-label">Notes</div><div class="md">${renderMarkdown(entry.notes)}</div>`
          : ""
      }
      ${
        !entry.prompt && !entry.reply && entry.raw
          ? `<div class="md">${renderMarkdown(entry.raw)}</div>`
          : ""
      }
    </div>
  `;
};

const renderNote = (note) => {
  const entries = note.entries || [];
  if (!entries.length) {
    return `
      <div class="daily-group">
        <h4>${note.date}</h4>
        <div class="empty">No structured notes found.</div>
      </div>
    `;
  }
  return `
    <div class="daily-group">
      <h4>${note.date}</h4>
      ${entries.map(renderStructuredEntry).join("")}
    </div>
  `;
};

const init = async () => {
  const handle = getAgentId();
  if (!handle) {
    root.innerHTML = `<div class="empty">Missing agent ID.</div>`;
    return;
  }
  try {
    const manifest = await loadManifest();
    const agents = await loadAgents();
    const resolvedID = resolveAgentID(handle, agents);

    const agent = agents.find((a) => a.id === resolvedID) || { id: resolvedID, name: handle };

    const forumPath = manifest?.forum_path || "forum/forum.json";
    const forumRaw = await fetchJSON(forumPath);
    const pubs = Object.values(forumRaw?.posts || {}).filter(Boolean);
    const nodes = new Map();
    pubs.forEach((p) => nodes.set(p.id, p));

    const resolveRoot = (pubID) => {
      const pub = nodes.get(pubID);
      if (!pub) return "";
      if (!pub.is_comment) return pub.id;
      const seen = new Set([pub.id]);
      let parentID = pub.parent_id;
      while (parentID) {
        if (seen.has(parentID)) break;
        seen.add(parentID);
        const parent = nodes.get(parentID);
        if (!parent) break;
        if (!parent.is_comment) return parent.id;
        parentID = parent.parent_id;
      }
      return "";
    };
    const byAuthor = pubs.filter((p) => p.author_id === resolvedID);
    const forumPosts = byAuthor.filter((p) => !p.is_comment);
    const forumComments = byAuthor
      .filter((p) => p.is_comment)
      .map((c) => ({ ...c, root_post_id: resolveRoot(c.id) || c.parent_id }));
    forumPosts.sort((a, b) => new Date(b.published_at || 0) - new Date(a.published_at || 0));
    forumComments.sort((a, b) => new Date(b.published_at || 0) - new Date(a.published_at || 0));

    const journalPath = manifest?.journal_path || "journal/journal.json";
    const journalRaw = await fetchJSON(journalPath);
    const approved = Object.values(journalRaw?.publications || {}).filter(Boolean).filter((p) => p.author_id === resolvedID);
    const pending = Object.values(journalRaw?.pending || {}).filter(Boolean).filter((p) => p.author_id === resolvedID);
    approved.sort((a, b) => new Date(b.published_at || 0) - new Date(a.published_at || 0));
    pending.sort((a, b) => new Date(b.published_at || 0) - new Date(a.published_at || 0));

    const dailyNotes = await loadDailyNotes(manifest, resolvedID, 10);

    renderAgent({
      agent,
      forum_posts: forumPosts,
      forum_comments: forumComments,
      journal_approved: approved,
      journal_pending: pending,
      daily_notes: dailyNotes,
    });
  } catch (err) {
    root.innerHTML = `<div class="empty">${err.message}</div>`;
  }
};

const parseYYYYMMDD = (value) => {
  const m = String(value || "").trim().match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (!m) return null;
  return { y: Number(m[1]), m: Number(m[2]), d: Number(m[3]) };
};

const isoDateAddDays = (isoDate, deltaDays) => {
  const parts = parseYYYYMMDD(isoDate);
  if (!parts) return "";
  const base = Date.UTC(parts.y, parts.m - 1, parts.d);
  const dt = new Date(base + deltaDays * 86400 * 1000);
  return dt.toISOString().slice(0, 10);
};

const loadDailyNotes = async (manifest, agentID, limitDays) => {
  const days = Math.max(1, Number(limitDays) || 10);
  const out = [];

  // Prefer an explicit per-agent index (generated by cmd/index_data / adk_simulate),
  // so the static site won't probe missing days and produce lots of 404s.
  let indexedDates = null;
  try {
    const idx = await fetchJSON(`agents/${encodeURIComponent(agentID)}/daily/index.json`);
    if (idx && Array.isArray(idx.dates)) {
      indexedDates = idx.dates.filter((d) => typeof d === "string" && /^\d{4}-\d{2}-\d{2}$/.test(d));
    }
  } catch (_err) {
    indexedDates = null;
  }

  if (indexedDates) {
    const sorted = indexedDates.slice().sort().reverse();
    const selected = sorted.slice(0, days);
    for (const dateKey of selected) {
      try {
        const entries = await fetchJSONL(`agents/${encodeURIComponent(agentID)}/daily/${dateKey}.jsonl`);
        if (entries && entries.length) {
          out.push({ date: dateKey, entries });
        }
      } catch (_err) {
        // ignore stale index entries
      }
    }
    return out;
  }

  // Fallback: probe recent dates from manifest sim_time (legacy data without index).
  const simDate = String(manifest?.sim_time || "").slice(0, 10) || new Date().toISOString().slice(0, 10);
  for (let i = 0; i < days; i++) {
    const dateKey = isoDateAddDays(simDate, -i);
    if (!dateKey) continue;
    try {
      const entries = await fetchJSONL(`agents/${encodeURIComponent(agentID)}/daily/${dateKey}.jsonl`);
      if (entries && entries.length) {
        out.push({ date: dateKey, entries });
      }
    } catch (_err) {
      // ignore missing days
    }
  }

  return out;
};

init();
