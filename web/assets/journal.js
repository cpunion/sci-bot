import { renderMarkdown, typesetMath } from "./markdown.js";

const journalList = document.getElementById("journal-list");
const searchInput = document.getElementById("journal-search");
const tabs = document.getElementById("journal-tabs");
const sidebar = document.getElementById("journal-sidebar");
const aboutHTML = sidebar ? sidebar.innerHTML : "";

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

const formatTime = (iso) => {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  if (date.getFullYear() <= 1970) return "";
  return date.toLocaleDateString();
};

let journalData = { approved: [], pending: [] };
let activeTab = "approved";
let selectedPaperID = "";

const getURLState = () => {
  const params = new URLSearchParams(window.location.search);
  const tab = (params.get("tab") || "").trim();
  const paper = (params.get("paper") || "").trim();
  if (tab === "approved" || tab === "pending") {
    activeTab = tab;
  }
  selectedPaperID = paper;
};

const setURLState = (tab, paperID, replace = false) => {
  const params = new URLSearchParams(window.location.search);
  if (tab) params.set("tab", tab);
  else params.delete("tab");
  if (paperID) params.set("paper", paperID);
  else params.delete("paper");
  const qs = params.toString();
  const url = `${window.location.pathname}${qs ? `?${qs}` : ""}`;
  if (replace) {
    window.history.replaceState({}, "", url);
    return;
  }
  window.history.pushState({}, "", url);
};

const findPaper = (paperID) => {
  if (!paperID) return null;
  for (const p of journalData.approved || []) {
    if (p && p.id === paperID) return { paper: p, section: "approved" };
  }
  for (const p of journalData.pending || []) {
    if (p && p.id === paperID) return { paper: p, section: "pending" };
  }
  return null;
};

const renderSidebar = () => {
  if (!sidebar) return;

  const match = findPaper(selectedPaperID);
  if (!selectedPaperID) {
    sidebar.innerHTML = aboutHTML;
    return;
  }
  if (!match) {
    sidebar.innerHTML = `
      <div class="sidebar-top">
        <h4>Paper Not Found</h4>
        <button class="tab-btn" type="button" data-action="close-paper">Back</button>
      </div>
      <p class="post-meta">No journal item found for <code>${escapeHTML(selectedPaperID)}</code>.</p>
    `;
    return;
  }

  const { paper, section } = match;
  const statusLabel = section === "approved" ? "Published" : "Pending Review";
  const date = formatTime(paper.published_at);
  const dateLabel = date ? ` • ${date}` : "";
  const abstract = paper.abstract ? `<div class="md">${renderMarkdown(paper.abstract)}</div>` : "";

  sidebar.innerHTML = `
    <div class="sidebar-top">
      <h4>Paper</h4>
      <button class="tab-btn" type="button" data-action="close-paper">Back</button>
    </div>
    <div class="paper-detail">
      <h3>${escapeHTML(paper.title || "Untitled")}</h3>
      <div class="post-meta">${escapeHTML(paper.author_name || "Unknown")} • <span class="badge">${escapeHTML(statusLabel)}</span>${escapeHTML(
        dateLabel
      )}</div>
      ${abstract ? `<div class="daily-label">Abstract</div>${abstract}` : ""}
      <div class="daily-label">Content</div>
      <div class="md">${renderMarkdown(paper.content || "")}</div>
      <div class="post-meta">ID: <code>${escapeHTML(paper.id || "")}</code>${paper.draft_id ? ` • draft: <code>${escapeHTML(paper.draft_id)}</code>` : ""}</div>
    </div>
  `;
  typesetMath(sidebar);
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

  journalList.innerHTML = list
    .map(
      (paper) => `
      <article class="paper clickable ${paper.id === selectedPaperID ? "is-selected" : ""}" data-paper-id="${escapeHTML(paper.id || "")}">
        <h3>${escapeHTML(paper.title || "Untitled")}</h3>
        <div class="authors">${escapeHTML(paper.author_name || "Unknown")} • ${escapeHTML(formatTime(paper.published_at))}</div>
        <div class="md">${renderMarkdown(paper.abstract || paper.content || "")}</div>
        <div class="post-meta">${paper.subreddit ? `Topic: ${paper.subreddit}` : ""}</div>
      </article>
    `
    )
    .join("");
  typesetMath(journalList);
};

const init = async () => {
  try {
    getURLState();
    const data = await fetchJSON("/api/journal");
    journalData = data;
    if (selectedPaperID) {
      const match = findPaper(selectedPaperID);
      if (match && match.section && match.section !== activeTab) {
        activeTab = match.section;
      }
    }

    [...tabs.querySelectorAll(".tab-btn")].forEach((tab) => {
      tab.classList.toggle("active", tab.dataset.tab === activeTab);
    });
    render();
    renderSidebar();
  } catch (err) {
    journalList.innerHTML = `<div class="empty">${err.message}</div>`;
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
  setURLState(activeTab, selectedPaperID);
  render();
});

journalList.addEventListener("click", (event) => {
  const card = event.target.closest("[data-paper-id]");
  if (!card) return;
  const paperID = card.dataset.paperId;
  if (!paperID) return;
  selectedPaperID = paperID;
  setURLState(activeTab, selectedPaperID);
  render();
  renderSidebar();
});

if (sidebar) {
  sidebar.addEventListener("click", (event) => {
    const btn = event.target.closest("button");
    if (!btn) return;
    if (btn.dataset.action === "close-paper") {
      selectedPaperID = "";
      setURLState(activeTab, "", false);
      render();
      renderSidebar();
    }
  });
}

window.addEventListener("popstate", () => {
  getURLState();
  render();
  renderSidebar();
});

init();
