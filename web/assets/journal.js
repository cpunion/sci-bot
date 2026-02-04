const journalList = document.getElementById("journal-list");
const searchInput = document.getElementById("journal-search");
const tabs = document.getElementById("journal-tabs");

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

let journalData = { approved: [], pending: [] };
let activeTab = "approved";

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
      <article class="paper">
        <h3>${paper.title || "Untitled"}</h3>
        <div class="authors">${paper.author_name || "Unknown"} • ${formatTime(paper.published_at)}</div>
        <p>${paper.abstract || paper.content || ""}</p>
        <div class="post-meta">${paper.subreddit ? `Topic: ${paper.subreddit}` : ""}</div>
      </article>
    `
    )
    .join("");
};

const init = async () => {
  try {
    const data = await fetchJSON("/api/journal");
    journalData = data;
    render();
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
  render();
});

init();
