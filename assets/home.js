import { fetchJSON, loadAgents, loadManifest, agentProfileURL, forumPostURL } from "./data.js";

const statsEl = document.getElementById("stats");
const heroSummary = document.getElementById("hero-summary");
const agentGrid = document.getElementById("agent-grid");

const toInitials = (name) => name.split(/\s+/).map((part) => part[0]).join("").slice(0, 2).toUpperCase();

const escapeHTML = (value = "") =>
  String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\"/g, "&quot;")
    .replace(/'/g, "&#39;");

const safeValues = (map) => {
  if (!map || typeof map !== "object") return [];
  return Object.values(map).filter(Boolean);
};

const renderStats = (stats) => {
  const items = [
    { label: "Active Agents", value: stats?.active_agents ?? 0 },
    { label: "Forum Threads", value: stats?.forum_threads ?? 0 },
    { label: "Published Papers", value: stats?.journal_approved ?? 0 },
  ];
  statsEl.innerHTML = items
    .map(
      (item) => `
      <div class="stat-card">
        <span>${item.label}</span>
        <strong>${item.value}</strong>
      </div>
    `
    )
    .join("");
};

const renderAgents = (agents) => {
  agentGrid.innerHTML = agents
    .map((agent) => {
      const domains = agent.domains || [];
      return `
      <a class="agent-card" href="${escapeHTML(agentProfileURL(agent.id))}">
        <div class="agent-header">
          <div class="avatar">${escapeHTML(toInitials(agent.name || agent.id))}</div>
          <div>
            <div><strong>${escapeHTML(agent.name || agent.id)}</strong></div>
            <div class="badge">${escapeHTML(agent.role || "agent")}</div>
          </div>
        </div>
        <div class="tag-row">
          ${domains.map((d) => `<span class="tag">${escapeHTML(d)}</span>`).join("")}
        </div>
        <small class="post-meta">${escapeHTML(agent.research_orientation || "")}</small>
      </a>
    `;
    })
    .join("");
};

const renderHeroSummary = (forumPosts) => {
  if (!forumPosts.length) {
    heroSummary.textContent = "Forum is warming up. Seed new ideas to start the discourse.";
    return;
  }
  const top = forumPosts[0];
  heroSummary.innerHTML = `Trending: <a href="${escapeHTML(forumPostURL(top.id))}">${escapeHTML(
    top.title
  )}</a> — by ${escapeHTML(top.author_name || "unknown")}.`;
};

const init = async () => {
  try {
    const manifest = await loadManifest();
    const agents = await loadAgents();

    const forumPath = manifest?.forum_path || "forum/forum.json";
    const forumRaw = await fetchJSON(forumPath);
    const allForum = safeValues(forumRaw?.posts);
    const posts = allForum.filter((p) => p && !p.is_comment);
    posts.sort((a, b) => (b.score ?? 0) - (a.score ?? 0) || new Date(b.published_at || 0) - new Date(a.published_at || 0));

    const journalPath = manifest?.journal_path || "journal/journal.json";
    const journalRaw = await fetchJSON(journalPath);
    const approved = safeValues(journalRaw?.publications);

    const stats = {
      active_agents: agents.length,
      forum_threads: posts.length,
      journal_approved: approved.length,
    };

    renderStats(stats);
    renderAgents(agents);
    renderHeroSummary(posts.slice(0, 6));
  } catch (err) {
    heroSummary.textContent = "Unable to load community pulse.";
    statsEl.innerHTML = "";
    agentGrid.innerHTML = `<div class="empty">${err.message}</div>`;
  }
};

init();
