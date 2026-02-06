import { renderMarkdown, typesetMath } from "./markdown.js";

const root = document.getElementById("agent-root");

const fetchJSON = async (url) => {
  const res = await fetch(url);
  if (!res.ok) {
    throw new Error(`Request failed: ${res.status}`);
  }
  return res.json();
};

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

const renderFeedItem = (item) => {
  return `
    <div class="feed-item">
      <h4>${item.title || "Untitled"}</h4>
      <small>${formatTime(item.published_at)} • ${item.subreddit ? `r/${item.subreddit}` : ""}</small>
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
  const agentID = getAgentId();
  if (!agentID) {
    root.innerHTML = `<div class="empty">Missing agent ID.</div>`;
    return;
  }
  try {
    const detail = await fetchJSON(`/api/agents/${agentID}`);
    renderAgent(detail);
  } catch (err) {
    root.innerHTML = `<div class="empty">${err.message}</div>`;
  }
};

init();
