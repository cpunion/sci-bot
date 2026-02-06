import { fetchJSON, loadManifest, forumPostURL } from "./data.js";
import { renderMarkdown, typesetMath } from "./markdown.js";

const forumContent = document.getElementById("forum-content");
const subredditList = document.getElementById("subreddit-list");
const tabs = document.getElementById("forum-tabs");

const formatTime = (iso) => {
  if (!iso) return "";
  const date = new Date(iso);
  const diff = Math.floor((Date.now() - date.getTime()) / 1000);
  if (diff < 60) return "just now";
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return date.toLocaleDateString();
};

const escapeHTML = (value = "") =>
  String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\"/g, "&quot;")
    .replace(/'/g, "&#39;");

const getQuery = () => new URLSearchParams(window.location.search);

const safeValues = (map) => {
  if (!map || typeof map !== "object") return [];
  return Object.values(map).filter(Boolean);
};

const resolveRootPostID = (pubID, nodes) => {
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

const renderPostList = (posts) => {
  if (!posts.length) {
    forumContent.innerHTML = `<div class="empty">No posts yet. Start a new thread from an agent.</div>`;
    return;
  }
  forumContent.innerHTML = posts
    .map(
      (post) => `
      <article class="post">
        <div class="vote">
          <span>▲</span>
          <div>${post.score ?? 0}</div>
          <span>▼</span>
        </div>
        <div>
          <div class="post-meta">r/${escapeHTML(post.subreddit || "general")} • ${escapeHTML(
            post.author_name || "unknown"
          )} • ${escapeHTML(formatTime(post.published_at))}</div>
          <h3><a href="${escapeHTML(forumPostURL(post.id))}">${escapeHTML(post.title || "")}</a></h3>
          <div class="md">${renderMarkdown(post.abstract || post.content || "")}</div>
          <div class="post-meta">${post.comments || 0} comments</div>
        </div>
      </article>
    `
    )
    .join("");
  typesetMath(forumContent);
};

const sortByScoreThenTime = (items) => {
  return [...items].sort((a, b) => {
    const scoreDiff = (b.score ?? 0) - (a.score ?? 0);
    if (scoreDiff !== 0) return scoreDiff;
    return new Date(b.published_at || 0) - new Date(a.published_at || 0);
  });
};

const buildCommentTree = (comments, rootID) => {
  const nodes = new Map();
  comments.forEach((comment) => {
    nodes.set(comment.id, { ...comment, children: [] });
  });

  const roots = [];
  nodes.forEach((node) => {
    if (node.parent_id === rootID || !nodes.has(node.parent_id)) {
      roots.push(node);
      return;
    }
    nodes.get(node.parent_id).children.push(node);
  });

  const sortTree = (items) => {
    const sorted = sortByScoreThenTime(items);
    sorted.forEach((item) => {
      item.children = sortTree(item.children);
    });
    return sorted;
  };

  return sortTree(roots);
};

const renderCommentNode = (comment, depth = 0) => `
  <div class="comment-node" id="${comment.id}" data-depth="${depth}">
    <div class="comment-body">
      <small>Reply by ${escapeHTML(comment.author_name || "unknown")} • ${escapeHTML(formatTime(comment.published_at))}</small>
      <div class="md">${renderMarkdown(comment.content || "")}</div>
    </div>
    ${
      comment.children?.length
        ? `<div class="comment-children">${comment.children
            .map((child) => renderCommentNode(child, depth + 1))
            .join("")}</div>`
        : ""
    }
  </div>
`;

const renderPostDetail = (post, comments) => {
  if (!post) {
    forumContent.innerHTML = `<div class="empty">Post not found.</div>`;
    return;
  }
  const tree = buildCommentTree(comments || [], post.id);
  const commentList = tree.length
    ? tree.map((comment) => renderCommentNode(comment)).join("")
    : `<div class="empty">No comments yet.</div>`;

  forumContent.innerHTML = `
    <article class="post">
      <div class="vote">
        <span>▲</span>
        <div>${post.score ?? 0}</div>
        <span>▼</span>
      </div>
      <div>
        <div class="post-meta">r/${escapeHTML(post.subreddit || "general")} • ${escapeHTML(
          post.author_name || "unknown"
        )} • ${escapeHTML(formatTime(post.published_at))}</div>
        <h3>${escapeHTML(post.title || "")}</h3>
        <div class="md">${renderMarkdown(post.content || post.abstract || "")}</div>
        <div class="post-meta">${post.comments || 0} comments</div>
      </div>
    </article>
    <section class="feed-section">
      <h3>Discussion</h3>
      ${commentList}
    </section>
  `;
  typesetMath(forumContent);
  const hash = window.location.hash;
  if (hash && hash.length > 1) {
    const target = document.getElementById(hash.slice(1));
    if (target) {
      target.scrollIntoView({ behavior: "smooth", block: "center" });
    }
  }
};

const renderSubreddits = (stats, active) => {
  const items = Object.entries(stats || {})
    .sort((a, b) => b[1] - a[1])
    .map(
      ([name, count]) => `
      <button data-subreddit="${name}" class="${active === name ? "active" : ""}">
        r/${escapeHTML(name)} <small>(${count})</small>
      </button>
    `
    )
    .join("");

  subredditList.innerHTML = `
    <button data-subreddit="" class="${active ? "" : "active"}">All <small></small></button>
    ${items}
  `;
};

const loadForum = async () => {
  const query = getQuery();
  const postID = query.get("post");
  const sort = query.get("sort") || "hot";
  const subreddit = query.get("subreddit") || "";

  const manifest = await loadManifest();
  const forumPath = manifest?.forum_path || "forum/forum.json";
  const forum = await fetchJSON(forumPath);
  const pubs = safeValues(forum?.posts);
  const nodes = new Map();
  pubs.forEach((p) => nodes.set(p.id, p));

  const postsAll = pubs.filter((p) => p && !p.is_comment);

  const stats = {};
  for (const p of postsAll) {
    const sub = p.subreddit || "general";
    stats[sub] = (stats[sub] || 0) + 1;
  }

  if (postID) {
    const post = nodes.get(postID);
    const comments = pubs.filter((p) => p && p.is_comment && resolveRootPostID(p.id, nodes) === postID);
    renderPostDetail(post, comments || []);
    renderSubreddits(stats, subreddit);
    return;
  }

  let list = postsAll;
  if (subreddit) {
    list = list.filter((p) => p.subreddit === subreddit);
  }
  if (sort === "recent" || sort === "new") {
    list = [...list].sort((a, b) => new Date(b.published_at || 0) - new Date(a.published_at || 0));
  } else {
    list = [...list].sort(
      (a, b) => (b.score ?? 0) - (a.score ?? 0) || new Date(b.published_at || 0) - new Date(a.published_at || 0)
    );
  }

  renderPostList(list.slice(0, 30));
  renderSubreddits(stats, subreddit);

  [...tabs.querySelectorAll(".tab-btn")].forEach((btn) => {
    btn.classList.toggle("active", btn.dataset.sort === sort);
  });
};

subredditList.addEventListener("click", (event) => {
  const target = event.target.closest("button");
  if (!target) return;
  const subreddit = target.dataset.subreddit || "";
  const params = getQuery();
  params.set("subreddit", subreddit);
  params.delete("post");
  window.location.search = params.toString();
});

tabs.addEventListener("click", (event) => {
  const btn = event.target.closest("button");
  if (!btn) return;
  const params = getQuery();
  params.set("sort", btn.dataset.sort);
  params.delete("post");
  window.location.search = params.toString();
});

loadForum().catch((err) => {
  forumContent.innerHTML = `<div class="empty">${err.message}</div>`;
});
