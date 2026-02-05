const statsEl = document.getElementById("stats");
const heroSummary = document.getElementById("hero-summary");
const agentGrid = document.getElementById("agent-grid");

const toInitials = (name) => name.split(/\s+/).map((part) => part[0]).join("").slice(0, 2).toUpperCase();

const fetchJSON = async (url) => {
  const res = await fetch(url);
  if (!res.ok) {
    throw new Error(`Request failed: ${res.status}`);
  }
  return res.json();
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
      <a class="agent-card" href="/agent/${agent.id}">
        <div class="agent-header">
          <div class="avatar">${toInitials(agent.name || agent.id)}</div>
          <div>
            <div><strong>${agent.name || agent.id}</strong></div>
            <div class="badge">${agent.role || "agent"}</div>
          </div>
        </div>
        <div class="tag-row">
          ${domains.map((d) => `<span class="tag">${d}</span>`).join("")}
        </div>
        <small class="post-meta">${agent.research_orientation || ""}</small>
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
  heroSummary.textContent = `Trending: ${top.title} — by ${top.author_name || "unknown"}.`;
};

const init = async () => {
  try {
    const [{ agents }, forum, journal, stats] = await Promise.all([
      fetchJSON("/api/agents"),
      fetchJSON("/api/forum?sort=hot&limit=6"),
      fetchJSON("/api/journal"),
      fetchJSON("/api/stats"),
    ]);

    renderStats(stats);
    renderAgents(agents);
    renderHeroSummary(forum.posts || []);
  } catch (err) {
    heroSummary.textContent = "Unable to load community pulse.";
    statsEl.innerHTML = "";
    agentGrid.innerHTML = `<div class="empty">${err.message}</div>`;
  }
};

init();
