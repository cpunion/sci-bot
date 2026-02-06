import { fetchJSON, loadManifest, paperURL } from "./data.js";
import { renderMarkdown, typesetMath } from "./markdown.js";

const journalList = document.getElementById("journal-list");
const searchInput = document.getElementById("journal-search");
const tabs = document.getElementById("journal-tabs");

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

let journalData = { approved: [], pending: [] };
let activeTab = "approved";

const getURLState = () => {
  const params = new URLSearchParams(window.location.search);
  const tab = (params.get("tab") || "").trim();
  if (tab === "approved" || tab === "pending") {
    activeTab = tab;
  }
};

const setURLState = (tab, replace = false) => {
  const params = new URLSearchParams(window.location.search);
  if (tab) params.set("tab", tab);
  else params.delete("tab");
  const qs = params.toString();
  const url = `${window.location.pathname}${qs ? `?${qs}` : ""}`;
  if (replace) {
    window.history.replaceState({}, "", url);
    return;
  }
  window.history.pushState({}, "", url);
};

const render = () => {
  const query = searchInput.value.trim().toLowerCase();
  const list = (journalData[activeTab] || []).filter((item) => {
    if (!query) return true;
    return [item.title, item.abstract, item.content, item.author_name]
      .filter(Boolean)
      .some((field) => field.toLowerCase().includes(query));
  });

  if (!list.length) {
    journalList.innerHTML = `<div class="empty">No papers found in this section.</div>`;
    return;
  }

  const statusLabel = activeTab === "approved" ? "Published" : "Pending";

  journalList.innerHTML = list
    .map((paper) => {
      const paperID = paper.id || "";
      const href = paperID ? paperURL(paperID) : "#";
      const date = formatTime(paper.published_at);
      const dateLabel = date ? ` • ${escapeHTML(date)}` : "";

      return `
        <article class="paper clickable" data-paper-url="${escapeHTML(href)}">
          <div class="paper-topline">
            <a class="tab-btn" href="${escapeHTML(href)}">Open</a>
            <span class="badge">${escapeHTML(statusLabel)}</span>
          </div>
          <h3><a href="${escapeHTML(href)}">${escapeHTML(paper.title || "Untitled")}</a></h3>
          <div class="authors">${escapeHTML(paper.author_name || "Unknown")}${dateLabel}</div>
          <div class="md">${renderMarkdown(paper.abstract || paper.content || "")}</div>
          <div class="post-meta">${paper.subreddit ? `Topic: ${paper.subreddit}` : ""}</div>
        </article>
      `;
    })
    .join("");

  typesetMath(journalList);
};

const init = async () => {
  try {
    getURLState();
    const manifest = await loadManifest();
    const path = manifest?.journal_path || "journal/journal.json";
    const raw = await fetchJSON(path);

    const approved = Object.values(raw?.publications || {}).filter(Boolean);
    const pending = Object.values(raw?.pending || {}).filter(Boolean);
    approved.sort((a, b) => new Date(b.published_at || 0) - new Date(a.published_at || 0));
    pending.sort((a, b) => new Date(b.published_at || 0) - new Date(a.published_at || 0));
    journalData = { name: raw?.name || "Journal", approved, pending };

    [...tabs.querySelectorAll(".tab-btn")].forEach((tab) => {
      tab.classList.toggle("active", tab.dataset.tab === activeTab);
    });
    render();
  } catch (err) {
    journalList.innerHTML = `<div class="empty">${escapeHTML(err.message)}</div>`;
  }
};

searchInput.addEventListener("input", () => {
  render();
});

tabs.addEventListener("click", (event) => {
  const btn = event.target.closest("button");
  if (!btn) return;
  activeTab = btn.dataset.tab;
  [...tabs.querySelectorAll(".tab-btn")].forEach((tab) => {
    tab.classList.toggle("active", tab.dataset.tab === activeTab);
  });
  setURLState(activeTab);
  render();
});

journalList.addEventListener("click", (event) => {
  if (event.target.closest("a")) return;
  const card = event.target.closest("[data-paper-url]");
  if (!card) return;
  const url = card.dataset.paperUrl;
  if (!url || url === "#") return;
  window.location.href = url;
});

window.addEventListener("popstate", () => {
  getURLState();
  [...tabs.querySelectorAll(".tab-btn")].forEach((tab) => {
    tab.classList.toggle("active", tab.dataset.tab === activeTab);
  });
  render();
});

init();
