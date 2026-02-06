import { fetchJSON, loadManifest } from "./data.js";
import { renderMarkdown, typesetMath } from "./markdown.js";

const root = document.getElementById("paper-root");

const escapeHTML = (value = "") =>
  String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\"/g, "&quot;")
    .replace(/'/g, "&#39;");

const formatTime = (iso) => {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  if (date.getFullYear() <= 1970) return "";
  return date.toLocaleDateString();
};

const getPaperId = () => {
  const params = new URLSearchParams(window.location.search);
  const q = (params.get("id") || params.get("paper") || "").trim();
  if (q) return q;
  const match = window.location.pathname.match(/\/paper\/([^/]+)/);
  if (match) return decodeURIComponent(match[1]);
  return "";
};

const renderPaper = (data) => {
  const paper = data.paper || {};
  const status = data.status || (paper.approved ? "published" : "pending");
  const statusLabel = status === "published" ? "Published" : "Pending Review";
  const title = paper.title || "Untitled";
  const author = paper.author_name || paper.author_id || "Unknown";
  const date = formatTime(paper.published_at);
  const dateLabel = date ? ` • ${date}` : "";

  root.innerHTML = `
    <section class="feed-section paper-page">
      <div class="paper-topline">
        <a class="tab-btn" href="./journal.html">Back to Journal</a>
        <span class="badge">${escapeHTML(statusLabel)}</span>
      </div>

      <h2>${escapeHTML(title)}</h2>
      <div class="post-meta">${escapeHTML(author)}${escapeHTML(dateLabel)}</div>

      ${
        paper.abstract
          ? `<div class="daily-label">Abstract</div><div class="md">${renderMarkdown(paper.abstract)}</div>`
          : ""
      }
      <div class="daily-label">Content</div>
      <div class="md">${renderMarkdown(paper.content || "")}</div>

      <div class="post-meta">ID: <code>${escapeHTML(paper.id || "")}</code>${
        paper.draft_id ? ` • draft: <code>${escapeHTML(paper.draft_id)}</code>` : ""
      }</div>
    </section>
  `;
  typesetMath(root);
};

const init = async () => {
  const paperID = getPaperId();
  if (!paperID) {
    root.innerHTML = `<div class="empty">Missing paper ID.</div>`;
    return;
  }
  try {
    const manifest = await loadManifest();
    const path = manifest?.journal_path || "journal/journal.json";
    const raw = await fetchJSON(path);

    const published = raw?.publications || {};
    const pending = raw?.pending || {};
    const paper = published?.[paperID] || pending?.[paperID] || null;
    const status = published?.[paperID] ? "published" : pending?.[paperID] ? "pending" : "";
    if (!paper) {
      throw new Error("Paper not found.");
    }
    renderPaper({ journal_name: raw?.name || "Journal", status, paper });
  } catch (err) {
    root.innerHTML = `<div class="empty">${escapeHTML(err.message)}</div>`;
  }
};

init();
