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

const renderStats = (agents, forumPosts, journalApproved) => {
  const items = [
    { label: "Active Agents", value: agents.length },
    { label: "Forum Threads", value: forumPosts.length },
    { label: "Published Papers", value: journalApproved.length },
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
    const [{ agents }, forum, journal] = await Promise.all([
      fetchJSON("/api/agents"),
      fetchJSON("/api/forum?sort=hot&limit=6"),
      fetchJSON("/api/journal"),
    ]);

    renderStats(agents, forum.posts || [], journal.approved || []);
    renderAgents(agents);
    renderHeroSummary(forum.posts || []);
  } catch (err) {
    heroSummary.textContent = "Unable to load community pulse.";
    statsEl.innerHTML = "";
    agentGrid.innerHTML = `<div class="empty">${err.message}</div>`;
  }
};

init();
