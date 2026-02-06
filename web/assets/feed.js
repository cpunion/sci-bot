import {
  agentProfileURL,
  fetchJSON,
  fetchJSONL,
  forumCommentURL,
  forumPostURL,
  loadManifest,
} from "./data.js";
import { renderMarkdown, typesetMath } from "./markdown.js";

const root = document.getElementById("feed-root");

const escapeHTML = (value = "") =>
  String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\"/g, "&quot;")
    .replace(/'/g, "&#39;");

const formatDateTime = (iso) => {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toLocaleString();
};

const summarizeText = (text) => {
  if (!text) return "";
  const cleaned = String(text).replace(/[#>*_`\\-]/g, " ").replace(/\s+/g, " ").trim();
  if (!cleaned) return "";
  if (cleaned.length <= 160) return cleaned;
  return `${cleaned.slice(0, 160)}…`;
};

const renderTools = (calls = [], responses = []) => {
  if (!calls.length && !responses.length) return "";
  const callHTML = calls.length
    ? `<div class="daily-label">Tool Calls</div><div class="md">${renderMarkdown(calls.map((c) => `- \`${c}\``).join("\n"))}</div>`
    : "";
  const respHTML = responses.length
    ? `
      <details class="event-tool-responses">
        <summary>Tool Responses</summary>
        <div class="md">${renderMarkdown(responses.join("\n\n---\n\n"))}</div>
      </details>
    `
    : "";
  return `${callHTML}${respHTML}`;
};

const renderEvent = (ev) => {
  const who = ev.agent_name || ev.agent_id || "agent";
  const whoURL = ev.actor_url || (ev.agent_id ? agentProfileURL(ev.agent_id) : "");
  const action = ev.action || "action";
  const when = formatDateTime(ev.sim_time || ev.timestamp);
  const tick = Number.isFinite(ev.tick) ? ` • tick ${ev.tick}` : "";
  const tokens =
    ev.total_tokens && Number(ev.total_tokens) > 0
      ? ` • tokens ${Number(ev.total_tokens)}${ev.usage_events ? `/${Number(ev.usage_events)} calls` : ""}`
      : "";
  const model = ev.model_name ? ` • <code>${escapeHTML(ev.model_name)}</code>` : "";

  const contentURL = ev.content_url || "";
  const contentTitle = ev.content_title || "";
  const contentLabel = contentTitle || (contentURL ? "Open content" : "");

  return `
    <div class="daily-entry event-entry">
      <div class="daily-header">
        <span class="daily-time">${escapeHTML(when)}${escapeHTML(tick)}${escapeHTML(tokens)}${model}</span>
        <div class="daily-summary">
          ${whoURL ? `<a href="${escapeHTML(whoURL)}">${escapeHTML(who)}</a>` : escapeHTML(who)}
          <span class="post-meta"> · ${escapeHTML(action)}</span>
          ${
            contentURL
              ? ` <span class="post-meta"> · </span><a class="content-link" href="${escapeHTML(contentURL)}">${escapeHTML(contentLabel)}</a>`
              : ""
          }
        </div>
      </div>
      ${
        ev.response
          ? `<div class="daily-label">Response</div><div class="md">${renderMarkdown(ev.response)}</div>`
          : ""
      }
      ${
        ev.prompt
          ? `<details class="event-details"><summary>Prompt</summary><div class="md">${renderMarkdown(ev.prompt)}</div></details>`
          : ""
      }
      ${
        (ev.tool_calls || []).length || (ev.tool_responses || []).length
          ? `<details class="event-details"><summary>Tools</summary>${renderTools(ev.tool_calls || [], ev.tool_responses || [])}</details>`
          : ""
      }
    </div>
  `;
};

const loadFeedIndex = async (manifest) => {
  const rel = String(manifest?.feed_index_path || "").trim() || "feed/index.json";
  try {
    const idx = await fetchJSON(rel);
    if (!idx || !Array.isArray(idx.shards) || !idx.shards.length) return null;
    return { path: rel, idx };
  } catch (_err) {
    return null;
  }
};

const pathDir = (p) => {
  const s = String(p || "").trim().replace(/^\/+/, "");
  const i = s.lastIndexOf("/");
  if (i <= 0) return "";
  return s.slice(0, i);
};

const renderShardedFeed = async (manifest, feedIndexPath, feedIdx, targetEvents) => {
  const feedDir = pathDir(feedIndexPath);
  const shards = Array.isArray(feedIdx?.shards) ? feedIdx.shards : [];
  let cursor = shards.length - 1;
  let loadedEvents = 0;

  let forumRaw = null;
  try {
    const forumPath = manifest?.forum_path || "forum/forum.json";
    forumRaw = await fetchJSON(forumPath);
  } catch (_err) {
    forumRaw = null;
  }

  root.innerHTML = `
    <section class="feed-section">
      <h3>Global Feed</h3>
      <div class="post-meta" id="feed-meta"></div>
      <div class="feed-actions">
        <button class="tab-btn" id="feed-load-more" type="button">Load more</button>
      </div>
    </section>
    <section class="feed-section" id="feed-events"></section>
  `;

  const metaEl = document.getElementById("feed-meta");
  const eventsEl = document.getElementById("feed-events");
  const btn = document.getElementById("feed-load-more");

  const total = Number(feedIdx?.total_events) > 0 ? Number(feedIdx.total_events) : 0;

  const updateMeta = () => {
    const remainingShards = cursor + 1;
    const totalLabel = total > 0 ? ` / ${total}` : "";
    metaEl.innerHTML = `Source: <code>${escapeHTML(feedIndexPath)}</code> • events: ${loadedEvents}${totalLabel} • remaining shards: ${remainingShards}`;
    btn.disabled = cursor < 0;
    btn.textContent = cursor < 0 ? "No more" : "Load more";
  };

  const loadOneShard = async () => {
    if (cursor < 0) return false;
    const shard = shards[cursor];
    cursor -= 1;

    const file = String(shard?.file || "").trim();
    if (!file) {
      updateMeta();
      return true;
    }

    const rel = feedDir ? `${feedDir}/${file}` : file;
    let evs = [];
    try {
      evs = await fetchJSONL(rel);
    } catch (_err) {
      evs = [];
    }

    // Shards are append-only oldest->newest, but the feed is displayed newest-first.
    evs.reverse();

    await hydrateFromDailyNotes(evs);
    if (forumRaw) enrichFromForum(evs, forumRaw);

    const chunk = document.createElement("div");
    chunk.innerHTML = evs.length ? evs.map(renderEvent).join("") : "";
    eventsEl.appendChild(chunk);
    typesetMath(chunk);

    loadedEvents += evs.length;
    updateMeta();
    return true;
  };

  btn.addEventListener("click", async () => {
    btn.disabled = true;
    btn.textContent = "Loading…";
    try {
      await loadOneShard();
    } finally {
      updateMeta();
    }
  });

  updateMeta();
  while (cursor >= 0 && loadedEvents < targetEvents) {
    // eslint-disable-next-line no-await-in-loop
    const ok = await loadOneShard();
    if (!ok) break;
  }

  if (loadedEvents === 0) {
    eventsEl.innerHTML = `<div class="empty">No events found.</div>`;
  }
  updateMeta();
};

const renderLogsFeed = async (manifest, lim, params) => {
  const requestedLog = String(params?.get("log") || "").trim();
  const logNames = Array.isArray(manifest?.logs) ? manifest.logs : [];

  let logsToLoad = [];
  let logLabel = requestedLog || "all";
  if (!requestedLog || requestedLog === "all") {
    logsToLoad = logNames.length ? logNames : ["logs.jsonl"];
    logLabel = "all";
  } else {
    logsToLoad = [requestedLog];
  }

  const all = [];
  for (const name of logsToLoad) {
    try {
      const evs = await fetchJSONL(name);
      for (const ev of evs || []) {
        all.push(ev);
      }
    } catch (_err) {
      // ignore missing logs
    }
  }

  all.sort((a, b) => {
    const at = new Date(a.sim_time || a.timestamp || 0).getTime();
    const bt = new Date(b.sim_time || b.timestamp || 0).getTime();
    if (at === bt) {
      return new Date(b.timestamp || 0) - new Date(a.timestamp || 0);
    }
    return bt - at;
  });

  const events = all.slice(0, lim);

  await hydrateFromDailyNotes(events);

  // Best-effort enrich content links via forum timestamps.
  try {
    const forumPath = manifest?.forum_path || "forum/forum.json";
    const forumRaw = await fetchJSON(forumPath);
    enrichFromForum(events, forumRaw);
  } catch (_err) {
    // ignore enrichment errors
  }

  root.innerHTML = `
    <section class="feed-section">
      <h3>Global Feed</h3>
      <div class="post-meta">Source: <code>${escapeHTML(logLabel)}</code> • events: ${events.length}</div>
    </section>
    <section class="feed-section">
      ${events.length ? events.map(renderEvent).join("") : `<div class="empty">No events found.</div>`}
    </section>
  `;
  typesetMath(root);
};

const init = async () => {
  try {
    const params = new URLSearchParams(window.location.search);
    const limit = params.get("limit") || "200";
    const lim = Math.max(1, Math.min(5000, Number(limit) || 200));

    const manifest = await loadManifest();

    const feed = await loadFeedIndex(manifest);
    if (feed) {
      await renderShardedFeed(manifest, feed.path, feed.idx, lim);
      return;
    }

    await renderLogsFeed(manifest, lim, params);
  } catch (err) {
    root.innerHTML = `<div class="empty">${escapeHTML(err.message)}</div>`;
  }
};

const normalizeToSeconds = (iso) => {
  if (!iso) return "";
  return String(iso).replace(/\.\d+(?=Z|[+-]\d\d:\d\d$)/, "");
};

const dailyCache = new Map(); // key: `${agent_id}|${YYYY-MM-DD}` -> Promise<map>

const loadDailyIndex = async (agentID, dateKey) => {
  const key = `${agentID}|${dateKey}`;
  if (dailyCache.has(key)) return dailyCache.get(key);

  const p = (async () => {
    try {
      const entries = await fetchJSONL(`agents/${encodeURIComponent(agentID)}/daily/${dateKey}.jsonl`);
      const map = new Map();
      for (const e of entries || []) {
        if (e && e.timestamp) {
          map.set(String(e.timestamp), e);
        }
      }
      return map;
    } catch (_err) {
      return new Map();
    }
  })();

  dailyCache.set(key, p);
  return p;
};

const hydrateFromDailyNotes = async (events) => {
  const tasks = [];
  for (const ev of events || []) {
    const sim = String(ev?.sim_time || "");
    const agentID = String(ev?.agent_id || "");
    if (!sim || !agentID) continue;
    const dateKey = sim.slice(0, 10);
    if (!dateKey) continue;
    const tsKey = normalizeToSeconds(sim);

    tasks.push(
      (async () => {
        const idx = await loadDailyIndex(agentID, dateKey);
        const entry = idx.get(tsKey);
        if (!entry) return;
        if (entry.prompt) ev.prompt = entry.prompt;
        if (entry.reply) ev.response = entry.reply;
      })()
    );
  }
  await Promise.all(tasks);
};

const enrichFromForum = (events, forumRaw) => {
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

  const postsByAuthor = new Map();
  const commentsByAuthor = new Map();
  for (const p of pubs) {
    if (!p?.author_id) continue;
    const m = p.is_comment ? commentsByAuthor : postsByAuthor;
    if (!m.has(p.author_id)) m.set(p.author_id, []);
    m.get(p.author_id).push(p);
  }

  for (const arr of postsByAuthor.values()) {
    arr.sort((a, b) => new Date(b.published_at || 0) - new Date(a.published_at || 0));
  }
  for (const arr of commentsByAuthor.values()) {
    arr.sort((a, b) => new Date(b.published_at || 0) - new Date(a.published_at || 0));
  }

  const maxDelta = 10 * 60 * 1000;
  const findClosest = (items, atMS) => {
    if (!items?.length || !Number.isFinite(atMS)) return null;
    let best = null;
    let bestDelta = 0;
    for (const item of items) {
      const t = new Date(item.published_at || 0).getTime();
      if (!Number.isFinite(t)) continue;
      const d = Math.abs(t - atMS);
      if (d > maxDelta) continue;
      if (!best || d < bestDelta) {
        best = item;
        bestDelta = d;
      }
    }
    return best;
  };

  for (const ev of events || []) {
    if (!ev || !ev.agent_id) continue;
    ev.actor_url = agentProfileURL(ev.agent_id);
    if (ev.content_url) continue;

    const atMS = new Date(ev.timestamp || 0).getTime();
    const calls = Array.isArray(ev.tool_calls) ? ev.tool_calls : [];

    if (calls.includes("create_post")) {
      const post = findClosest(postsByAuthor.get(ev.agent_id), atMS);
      if (post) {
        ev.content_kind = "forum_post";
        ev.content_id = post.id;
        ev.content_title = post.title || "";
        ev.content_url = forumPostURL(post.id);
        continue;
      }
    }

    if (calls.includes("comment") || calls.includes("request_consensus")) {
      const comment = findClosest(commentsByAuthor.get(ev.agent_id), atMS);
      if (comment) {
        const rootID = resolveRoot(comment.id) || comment.parent_id || "";
        if (rootID) {
          const root = nodes.get(rootID);
          ev.content_kind = "forum_comment";
          ev.content_id = comment.id;
          ev.content_title = root?.title || "Open thread";
          ev.content_url = forumCommentURL(rootID, comment.id);
        }
      }
    }
  }
};

init();
