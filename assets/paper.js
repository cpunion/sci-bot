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

const setMeta = (selector, value) => {
  const el = document.querySelector(selector);
  if (!el) return;
  el.setAttribute("content", value);
};

const setCanonical = (href) => {
  let link = document.querySelector('link[rel="canonical"]');
  if (!link) {
    link = document.createElement("link");
    link.setAttribute("rel", "canonical");
    document.head.appendChild(link);
  }
  link.setAttribute("href", href);
};

const toPlainText = (value) => String(value || "").replace(/\s+/g, " ").trim();

const clip = (value, max = 180) => {
  const s = toPlainText(value);
  if (!s) return "";
  if (s.length <= max) return s;
  return `${s.slice(0, max - 3)}...`;
};

const upsertJSONLD = (id, obj) => {
  if (!obj || typeof obj !== "object") return;
  let el = document.getElementById(id);
  if (!el) {
    el = document.createElement("script");
    el.id = id;
    el.type = "application/ld+json";
    document.head.appendChild(el);
  }
  el.textContent = JSON.stringify(obj);
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

  let publishedISO = "";
  if (paper.published_at) {
    const dt = new Date(paper.published_at);
    if (!Number.isNaN(dt.getTime()) && dt.getFullYear() > 1970) {
      publishedISO = dt.toISOString();
    }
  }

  const pageTitle = `${title} | Sci-Bot Journal`;
  document.title = pageTitle;
  const desc = clip(paper.abstract, 200) || clip(paper.content, 200) || "Sci-Bot paper page.";
  setMeta('meta[name="description"]', desc);
  setMeta('meta[property="og:title"]', pageTitle);
  setMeta('meta[property="og:description"]', desc);

  const url = new URL(window.location.href);
  url.hash = "";
  setCanonical(url.toString());
  setMeta('meta[property="og:url"]', url.toString());

  upsertJSONLD("scibot-ld-json", {
    "@context": "https://schema.org",
    "@type": "ScholarlyArticle",
    headline: title,
    author: [{ "@type": "Person", name: author }],
    description: desc,
    datePublished: publishedISO || undefined,
    identifier: paper.id || undefined,
    url: url.toString(),
    isAccessibleForFree: true,
  });

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
