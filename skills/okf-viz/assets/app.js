(function () {
  "use strict";
  var data = JSON.parse(document.getElementById("okf-data").textContent || "{}");
  var nodes = data.nodes || [], edges = data.edges || [], docs = data.docs || {};
  var manifest = data.manifest || {}; // id -> body-fragment path (lazy mode)
  var bodyCache = {};                  // id -> fetched body HTML
  var selectedId = null;               // current selection (for permalinks)
  var applyingState = false;           // guard against hash<->state feedback loops

  // ---- theme ----
  var themeSel = document.getElementById("theme");
  var saved = localStorage.getItem("okf-theme");
  var initial = saved || document.documentElement.getAttribute("data-theme") || "system";
  applyTheme(initial); themeSel.value = initial;
  themeSel.addEventListener("change", function () { applyTheme(themeSel.value); });
  function applyTheme(t) {
    document.documentElement.setAttribute("data-theme", t);
    localStorage.setItem("okf-theme", t);
    if (window.cy) styleGraph(window.cy);
  }
  function cssVar(name){ return getComputedStyle(document.documentElement).getPropertyValue(name).trim(); }

  // ---- cytoscape ----
  // relation -> CSS variable name for its edge color (see app.css).
  var relationVars = { "references": "--rel-references", "joins-with": "--rel-joins-with", "see-also": "--rel-see-also", "co-changes": "--rel-co-changes" };

  // tabular[id] = column array for concepts that carry a parsed # Columns table.
  var tabular = {};
  nodes.forEach(function (n) {
    var d = docs[n.id];
    if (n.kind === "concept" && d && d.columns && d.columns.length) tabular[n.id] = d.columns;
  });

  var coverageOverlay = false;

  function baseElements() {
    var els = [];
    nodes.forEach(function (n) {
      els.push({ data: { id: n.id, label: n.title, kind: n.kind, ntype: n.type || "", degree: n.degree || 1, coverage: n.coverage || "" } });
    });
    edges.forEach(function (e) {
      var rel = e.relation || "";
      els.push({ data: { id: e.source + "|" + e.target + "|" + e.kind + "|" + rel, source: e.source, target: e.target, kind: e.kind, relation: rel } });
    });
    return els;
  }

  // erElements renders each tabular concept as a single column-listing node (a
  // multi-line label: title + one line per column with PK/FK markers — a clean,
  // dependency-free ER box) and re-types FK edges between two tables as crow's-foot
  // `erfk` edges labeled with the FK column. Non-tabular nodes stay as normal nodes,
  // so an okf-fs bundle is unaffected by the toggle.
  function erElements() {
    var els = [];
    nodes.forEach(function (n) {
      if (tabular[n.id]) {
        var lines = [n.title];
        tabular[n.id].forEach(function (c) {
          var flags = (c.pk ? " 🔑" : "") + (c.fk ? " ↗" : "");
          lines.push(c.name + (c.type ? " : " + c.type : "") + flags);
        });
        els.push({ data: { id: n.id, label: lines.join("\n"), kind: "er-table", ntype: n.type || "", coverage: n.coverage || "" } });
      } else {
        els.push({ data: { id: n.id, label: n.title, kind: n.kind, ntype: n.type || "", degree: n.degree || 1, coverage: n.coverage || "" } });
      }
    });
    edges.forEach(function (e) {
      var rel = e.relation || "";
      if (e.kind === "crosslink" && rel === "references" && tabular[e.source] && tabular[e.target]) {
        els.push({ data: { id: "erfk|" + e.source + "|" + e.target + "|" + (e.label || ""), source: e.source, target: e.target, kind: "crosslink", relation: "references", erfk: "1", label: e.label || "" } });
      } else {
        els.push({ data: { id: e.source + "|" + e.target + "|" + e.kind + "|" + rel, source: e.source, target: e.target, kind: e.kind, relation: rel } });
      }
    });
    return els;
  }

  var erMode = false;

  var palette = ["#4f86c6","#3fae6b","#c6864f","#9b59b6","#e0556e","#1abc9c","#e08e3f","#7f8c8d"];
  var typeColors = {}; var ci = 0;
  nodes.forEach(function (n) { if (n.kind === "concept" && n.type && !(n.type in typeColors)) typeColors[n.type] = palette[ci++ % palette.length]; });

  var cy = window.cy = cytoscape({
    container: document.getElementById("graph"),
    elements: baseElements(),
    minZoom: 0.1, maxZoom: 4,
  });
  styleGraph(cy);
  cy.on("tap", "node", function (evt) {
    var n = evt.target;
    if (traceMode) { handleTraceTap(n); return; }
    var k = n.data("kind");
    if (clustered && (k === "directory" || k === "root")) {
      var dirKey = k === "root" ? "" : n.id();
      if (expandedDirs.has(dirKey)) expandedDirs.delete(dirKey); else expandedDirs.add(dirKey);
      applyClusterDisplay();
      return;
    }
    select(n.id());
  });
  runLayout(document.getElementById("layout").value);
  addLegend();

  // ---- directory clustering (scale) ----
  // Collapse concepts into their directory super-nodes; tap a directory to reveal
  // its concepts. Reuses the containment hierarchy already in the model (node.dir).
  var clustered = false;
  var expandedDirs = new Set();
  var clusterCb = document.getElementById("cluster");
  if (clusterCb) {
    clusterCb.addEventListener("change", function () {
      clustered = clusterCb.checked;
      expandedDirs.clear();
      if (clustered) applyClusterDisplay(); else applyFilter();
    });
  }
  // coverage overlay: recolor nodes by enrichment state (placeholder/enriched).
  var covCb = document.getElementById("coverage-overlay");
  if (covCb) {
    covCb.addEventListener("change", function () {
      coverageOverlay = covCb.checked;
      styleGraph(cy);
    });
  }
  function applyClusterDisplay() {
    cy.nodes('[kind="directory"], [kind="root"]').style("display", "element");
    cy.nodes('[kind="concept"]').forEach(function (cn) {
      var n = nodes.find(function (x) { return x.id === cn.id(); });
      var show = n && expandedDirs.has(n.dir || "") && matches(n);
      cn.style("display", show ? "element" : "none");
    });
  }

  // ER mode: rebuild the graph as column-listing table nodes (no-op visually for
  // bundles with no tabular concepts). dagre suits ER layouts.
  var erCheckbox = document.getElementById("er-mode");
  if (erCheckbox) {
    erCheckbox.addEventListener("change", function () {
      erMode = erCheckbox.checked;
      cy.json({ elements: erMode ? erElements() : baseElements() });
      styleGraph(cy);
      runLayout(erMode ? "dagre" : document.getElementById("layout").value);
    });
  }

  function styleGraph(cy) {
    var structural = cssVar("--containment") || "#c2c8d2";
    var crosslink = cssVar("--crosslink") || "#c64f86";
    var text = cssVar("--text") || "#1c2430";
    cy.style([
      { selector: "node", style: {
        "label": "data(label)", "font-size": 9, "color": text, "text-valign": "bottom",
        "text-margin-y": 3, "width": "mapData(degree, 1, 12, 14, 44)", "height": "mapData(degree, 1, 12, 14, 44)",
        "background-color": function (n) {
          var k = n.data("kind");
          if (k === "root") return "#b3b8c2";
          if (k === "directory") return "#8a94a6";
          return typeColors[n.data("ntype")] || "#4f86c6";
        } } },
      { selector: 'node[kind="directory"], node[kind="root"]', style: { "shape": "round-rectangle" } },
      { selector: "node.selected", style: { "border-width": 3, "border-color": cssVar("--accent") || "#4f86c6" } },
      { selector: "node.dim", style: { "opacity": 0.25 } },
      { selector: "node.traced", style: { "opacity": 1, "border-width": 3, "border-color": cssVar("--accent") || "#4f86c6" } },
      { selector: "edge.traced", style: { "opacity": 1, "width": 4, "line-color": cssVar("--accent") || "#4f86c6", "target-arrow-color": cssVar("--accent") || "#4f86c6" } },
      { selector: 'edge[kind="containment"]', style: {
        "width": 1.5, "line-color": structural, "line-style": "dashed", "curve-style": "straight" } },
      { selector: 'edge[kind="crosslink"]', style: {
        "width": 2, "line-color": crosslink, "curve-style": "bezier",
        "target-arrow-color": crosslink, "target-arrow-shape": "triangle" } },
      { selector: "edge.hidden", style: { "display": "none" } },
      // ER mode: column-listing table node (multi-line label) + crow's-foot FK edge.
      { selector: 'node[kind="er-table"]', style: {
        "shape": "round-rectangle", "label": "data(label)", "text-wrap": "wrap",
        "text-valign": "center", "text-halign": "center", "text-justification": "left",
        "font-size": 8, "padding": "8px", "width": "label", "height": "label",
        "background-color": cssVar("--surface") || "#f6f7f9",
        "border-width": 1, "border-color": cssVar("--border") || "#dfe3e8", "color": text } },
      { selector: 'edge[erfk="1"]', style: {
        "width": 2, "curve-style": "bezier", "label": "data(label)", "font-size": 7,
        "color": cssVar("--muted") || "#5b6470", "text-rotation": "autorotate",
        "line-color": cssVar("--rel-references") || crosslink,
        "source-arrow-shape": "tee", "source-arrow-color": cssVar("--rel-references") || crosslink,
        "target-arrow-shape": "triangle-tee", "target-arrow-color": cssVar("--rel-references") || crosslink } },
    ].concat(relationStyles()).concat(coverageOverlay ? coverageStyles() : []));
  }

  // coverageStyles recolors concept nodes by enrichment state; layered over the
  // type coloring only while the overlay is on, so default coloring is untouched.
  function coverageStyles() {
    return [
      { selector: 'node[coverage="placeholder"]', style: { "background-color": cssVar("--cov-placeholder") || "#d9534f" } },
      { selector: 'node[coverage="enriched"]', style: { "background-color": cssVar("--cov-enriched") || "#3fae6b" } },
    ];
  }

  // relationStyles produces one style rule per known relation, coloring its edges
  // distinctly (theme-aware via CSS vars) and varying the arrow shape so FK,
  // join, see-also, and co-change edges read differently. Unknown relations fall
  // back to the generic crosslink style above.
  function relationStyles() {
    var crosslink = cssVar("--crosslink") || "#c64f86";
    var shapes = { "references": "triangle", "joins-with": "triangle-tee", "see-also": "vee", "co-changes": "diamond" };
    var out = [];
    Object.keys(relationVars).forEach(function (rel) {
      var color = cssVar(relationVars[rel]) || crosslink;
      out.push({ selector: 'edge[relation="' + rel + '"]', style: {
        "line-color": color, "target-arrow-color": color,
        "target-arrow-shape": shapes[rel] || "triangle" } });
    });
    return out;
  }

  function runLayout(name) {
    var opts = { name: name, animate: true, fit: true, padding: 30 };
    if (name === "concentric") opts.concentric = function (n) { return n.data("degree"); };
    if (name === "breadthfirst" || name === "dagre") opts.roots = "#__root__";
    if (name === "dagre") { opts.rankDir = "TB"; }
    try { cy.layout(opts).run(); }
    catch (e) { cy.layout({ name: "cose", animate: true, fit: true }).run(); }
  }
  document.getElementById("layout").addEventListener("change", function (e) { runLayout(e.target.value); });

  // edge toggles via legend checkboxes: one for structure, plus one per observed
  // relation among the crosslink edges (generic, relation-less links read as
  // "cross-links"). Generated from the data so a new relation appears automatically.
  function addLegend() {
    var el = document.createElement("div"); el.className = "legend";
    var rels = Array.from(new Set(edges.filter(function (e) { return e.kind === "crosslink"; })
      .map(function (e) { return e.relation || ""; }))).sort();
    var html = '<label><input type="checkbox" id="show-cont" checked> structure</label> ';
    rels.forEach(function (r) {
      var label = r || "cross-links";
      var cid = "show-rel-" + (r ? r.replace(/\W/g, "-") : "generic");
      html += '<label><input type="checkbox" class="rel-toggle" data-rel="' + (r || "") + '" id="' + cid + '" checked> ' + escapeHtml(label) + '</label> ';
    });
    el.innerHTML = html;
    document.getElementById("graph").appendChild(el);
    el.querySelector("#show-cont").addEventListener("change", function (e) {
      cy.edges('[kind="containment"]').toggleClass("hidden", !e.target.checked);
    });
    el.querySelectorAll(".rel-toggle").forEach(function (cb) {
      cb.addEventListener("change", function (ev) {
        var r = ev.target.getAttribute("data-rel");
        cy.edges('edge[kind="crosslink"][relation="' + r + '"]').toggleClass("hidden", !ev.target.checked);
        writeHash();
      });
    });
  }

  // ---- reader ----
  function currentHops() {
    var el = document.getElementById("hops");
    var v = el ? parseInt(el.value, 10) : 1;
    return isNaN(v) || v < 0 ? 1 : v;
  }
  // applySpotlight dims everything outside an N-hop neighborhood of n.
  function applySpotlight(n) {
    cy.elements().removeClass("dim traced");
    if (n.empty()) return;
    var hops = currentHops();
    var hood = n;
    for (var i = 0; i < hops; i++) hood = hood.closedNeighborhood();
    var keep = hops > 0 ? hood : n;
    cy.nodes().not(keep.nodes()).addClass("dim");
  }
  function select(id) {
    selectedId = id;
    cy.nodes().removeClass("selected");
    var n = cy.getElementById(id);
    if (n.nonempty()) {
      n.addClass("selected");
      applySpotlight(n);
      cy.animate({ center: { eles: n } }, { duration: 200 });
    }
    document.querySelectorAll("#tree li").forEach(function (li) { li.classList.toggle("selected", li.dataset.id === id); });
    var d = docs[id]; var r = document.getElementById("reader");
    if (!d) { r.innerHTML = '<p class="empty">' + escapeHtml(id) + '</p>'; }
    else {
      var fm = [d.type, (d.tags || []).join(", "), d.timestamp].filter(Boolean).join(" · ");
      var head =
        "<h1>" + escapeHtml(d.title) + "</h1>" +
        (fm ? '<div class="fm">' + escapeHtml(fm) + "</div>" : "") +
        (d.description ? "<p>" + escapeHtml(d.description) + "</p>" : "") +
        (d.resource ? '<div class="fm">resource: <code>' + escapeHtml(d.resource) + "</code></div>" : "");
      // Body may be inlined (small bundles) or lazy-loaded from a sibling fragment.
      var inlineBody = d.bodyHtml || bodyCache[id];
      if (inlineBody) {
        r.innerHTML = head + inlineBody;
        decorateReader(r, id, d);
      } else if (manifest[id]) {
        r.innerHTML = head + '<p class="empty">Loading…</p>';
        loadFragment(manifest[id], function (htmlText, ok) {
          if (!ok) { r.innerHTML = head + '<p class="empty">Body unavailable offline; serve the bundle over http to load it.</p>'; return; }
          bodyCache[id] = htmlText;
          r.innerHTML = head + htmlText;
          decorateReader(r, id, d);
        });
      } else {
        r.innerHTML = head;
      }
    }
    writeHash();
  }

  // decorateReader wires intra-bundle links and, when a structured Data Profile is
  // present, replaces its raw table with inline null-ratio bars (legible, no chart
  // dependency). Degrades to the plain body when no profile data exists.
  function decorateReader(r, id, d) {
    wireReaderLinks(r, id);
    if (!d || !d.profile || !d.profile.length) return;
    var charts = '<div class="pf">';
    d.profile.forEach(function (p) {
      var total = (p.nonNull || 0) + (p.null || 0);
      var nullPct = total > 0 ? Math.round(100 * (p.null || 0) / total) : 0;
      var meta = "distinct " + (p.distinct || 0) + (p.semantic ? " · " + p.semantic : "") +
        (p.min || p.max ? " · " + escapeHtml((p.min || "") + "…" + (p.max || "")) : "");
      charts += '<div class="pf-row"><span class="pf-col">' + escapeHtml(p.column) + '</span>' +
        '<span class="pf-bar" title="' + nullPct + '% null"><span class="pf-fill" style="width:' + (100 - nullPct) + '%"></span></span>' +
        '<span class="pf-meta">' + meta + '</span></div>';
    });
    charts += "</div>";
    // Replace the rendered "Data Profile" heading + its table with the charts.
    var headings = r.querySelectorAll("h1, h2, h3");
    for (var i = 0; i < headings.length; i++) {
      if (headings[i].textContent.trim() === "Data Profile") {
        var tbl = headings[i].nextElementSibling;
        while (tbl && tbl.tagName !== "TABLE" && tbl.tagName !== "H1" && tbl.tagName !== "H2" && tbl.tagName !== "H3") tbl = tbl.nextElementSibling;
        if (tbl && tbl.tagName === "TABLE") tbl.outerHTML = charts;
        return;
      }
    }
    // No heading found in body (e.g. lazy/edge cases): append the charts.
    r.insertAdjacentHTML("beforeend", '<h2>Data Profile</h2>' + charts);
  }

  // wireReaderLinks makes intra-bundle .md links navigate within the viewer.
  function wireReaderLinks(r, id) {
    r.querySelectorAll('a[href$=".md"]').forEach(function (a) {
      a.addEventListener("click", function (ev) {
        ev.preventDefault();
        var t = resolveHref(id, a.getAttribute("href"));
        if (docs[t]) select(t);
      });
    });
  }

  // loadFragment fetches a lazy body fragment. Tries fetch() then XHR so it works
  // both over http and, where the browser permits, from a file:// open.
  function loadFragment(path, cb) {
    if (window.fetch) {
      fetch(path).then(function (resp) {
        if (!resp.ok) throw new Error("status " + resp.status);
        return resp.text();
      }).then(function (txt) { cb(txt, true); }).catch(function () { xhrFragment(path, cb); });
    } else {
      xhrFragment(path, cb);
    }
  }
  function xhrFragment(path, cb) {
    try {
      var x = new XMLHttpRequest();
      x.open("GET", path, true);
      x.onreadystatechange = function () {
        if (x.readyState === 4) {
          if (x.status === 200 || x.status === 0 && x.responseText) cb(x.responseText, true);
          else cb("", false);
        }
      };
      x.send();
    } catch (e) { cb("", false); }
  }
  function resolveHref(srcId, href) {
    href = href.split("#")[0].split("?")[0];
    var combined = href.charAt(0) === "/"
      ? href.slice(1)
      : (srcId.indexOf("/") >= 0 ? srcId.slice(0, srcId.lastIndexOf("/") + 1) : "") + href;
    // Normalize "." and ".." segments so parent-relative links resolve correctly
    // (mirrors path.Clean on the Go side).
    var parts = combined.split("/"), out = [];
    for (var i = 0; i < parts.length; i++) {
      var seg = parts[i];
      if (seg === "" || seg === ".") continue;
      if (seg === "..") { out.pop(); continue; }
      out.push(seg);
    }
    return out.join("/").replace(/\.md$/, "");
  }

  // ---- nav tree + filters ----
  var tree = document.getElementById("tree");
  // fuzzyScore ranks a subsequence match: contiguous runs and a prefix match score
  // higher; -1 means no match. Dependency-free.
  function fuzzyScore(q, text) {
    q = q.toLowerCase(); text = text.toLowerCase();
    if (!q) return 0;
    var ti = 0, score = 0, streak = 0, first = -1;
    for (var qi = 0; qi < q.length; qi++) {
      var idx = text.indexOf(q[qi], ti);
      if (idx < 0) return -1;
      if (first < 0) first = idx;
      if (idx === ti) { streak++; score += 5 + streak; } else { streak = 0; score += 1; }
      ti = idx + 1;
    }
    if (first === 0) score += 10;
    return score;
  }
  function searchScore(n) {
    return fuzzyScore(searchEl.value.trim(), n.id + " " + n.title + " " + (n.type || "") + " " + (n.tags || []).join(" "));
  }
  function renderTree() {
    tree.innerHTML = "";
    var q = searchEl.value.trim();
    var list = nodes.filter(function (n) { return n.kind === "concept" && typeMatch(n); });
    if (q) {
      list = list.map(function (n) { return { n: n, s: searchScore(n) }; })
        .filter(function (x) { return x.s >= 0; })
        .sort(function (a, b) { return b.s - a.s || (a.n.id < b.n.id ? -1 : 1); })
        .map(function (x) { return x.n; });
    } else {
      list.sort(function (a, b) { return a.id < b.id ? -1 : 1; });
    }
    list.forEach(function (n) {
      var li = document.createElement("li");
      li.textContent = n.id; li.dataset.id = n.id; li.title = n.description || "";
      li.addEventListener("click", function () { select(n.id); });
      tree.appendChild(li);
    });
    kbdIndex = -1;
  }
  // type filter checkboxes
  var types = Array.from(new Set(nodes.filter(function (n){return n.kind==="concept" && n.type;}).map(function(n){return n.type;}))).sort();
  var active = new Set(types);
  var filtersEl = document.getElementById("filters");
  types.forEach(function (t) {
    var id = "f-" + t.replace(/\W/g, "_");
    var lbl = document.createElement("label");
    lbl.innerHTML = '<input type="checkbox" checked id="' + id + '"> ' + escapeHtml(t);
    filtersEl.appendChild(lbl);
    lbl.querySelector("input").addEventListener("change", function (e) {
      if (e.target.checked) active.add(t); else active.delete(t);
      applyFilter();
    });
  });
  var searchEl = document.getElementById("search");
  searchEl.addEventListener("input", applyFilter);
  // keyboard navigation of the ranked result list.
  var kbdIndex = -1;
  searchEl.addEventListener("keydown", function (ev) {
    var items = tree.querySelectorAll("li");
    if (!items.length) return;
    if (ev.key === "ArrowDown") { kbdIndex = Math.min(kbdIndex + 1, items.length - 1); ev.preventDefault(); }
    else if (ev.key === "ArrowUp") { kbdIndex = Math.max(kbdIndex - 1, 0); ev.preventDefault(); }
    else if (ev.key === "Enter") { if (kbdIndex >= 0 && items[kbdIndex]) select(items[kbdIndex].dataset.id); return; }
    else { return; }
    items.forEach(function (li, i) { li.classList.toggle("kbd", i === kbdIndex); });
    if (items[kbdIndex]) items[kbdIndex].scrollIntoView({ block: "nearest" });
  });
  function typeMatch(n) { return !n.type || active.has(n.type); }
  function matches(n) {
    if (!typeMatch(n)) return false;
    if (!searchEl.value.trim()) return true;
    return searchScore(n) >= 0;
  }
  function applyFilter() {
    renderTree();
    writeHash();
    if (clustered) { applyClusterDisplay(); return; }
    cy.nodes('[kind="concept"]').forEach(function (cn) {
      var n = nodes.find(function (x) { return x.id === cn.id(); });
      cn.style("display", n && matches(n) ? "element" : "none");
    });
  }

  // ---- pane toggles ----
  // Toggle display:none on the pane; the flex layout reflows the rest (no grid
  // track-shifting). cy.resize() refits the graph after its pane changes size.
  wireToggle("toggle-nav", "nav");
  wireToggle("toggle-graph", "graph");
  wireToggle("toggle-reader", "reader");
  function wireToggle(btn, pane) {
    document.getElementById(btn).addEventListener("click", function () {
      document.getElementById(pane).classList.toggle("hidden");
      if (window.cy) cy.resize();
    });
  }

  // ---- permalinks: encode node + filters + layout + hidden relations in the hash ----
  function hiddenRelations() {
    var out = [];
    document.querySelectorAll(".legend .rel-toggle").forEach(function (cb) {
      if (!cb.checked) out.push(cb.getAttribute("data-rel") || "_generic");
    });
    return out;
  }
  function writeHash() {
    if (applyingState) return;
    var parts = [];
    if (selectedId) parts.push("n=" + encodeURIComponent(selectedId));
    if (active.size !== types.length) parts.push("f=" + encodeURIComponent(Array.from(active).sort().join(",")));
    var lay = document.getElementById("layout").value;
    if (lay) parts.push("l=" + encodeURIComponent(lay));
    var hr = hiddenRelations();
    if (hr.length) parts.push("h=" + encodeURIComponent(hr.join(",")));
    var h = parts.join("&");
    if (location.hash.slice(1) !== h) {
      try { history.replaceState(null, "", h ? "#" + h : "#"); } catch (e) { location.hash = h; }
    }
  }
  function readHash() {
    var raw = location.hash.slice(1);
    if (!raw) return null;
    if (raw.indexOf("=") < 0) return { n: decodeURIComponent(raw) }; // back-compat: bare concept id
    var st = {};
    raw.split("&").forEach(function (kv) {
      var i = kv.indexOf("="); if (i < 0) return;
      st[kv.slice(0, i)] = decodeURIComponent(kv.slice(i + 1));
    });
    return st;
  }
  function applyState(st) {
    if (!st) return;
    applyingState = true;
    try {
      if (typeof st.f === "string") {
        var want = new Set(st.f ? st.f.split(",") : []);
        active = new Set();
        types.forEach(function (t) {
          var on = want.has(t);
          if (on) active.add(t);
          var cb = document.getElementById("f-" + t.replace(/\W/g, "_"));
          if (cb) cb.checked = on;
        });
      }
      if (st.l) {
        var ls = document.getElementById("layout");
        if (ls) ls.value = st.l;
      }
      if (typeof st.h === "string") {
        var hide = new Set(st.h ? st.h.split(",") : []);
        document.querySelectorAll(".legend .rel-toggle").forEach(function (cb) {
          var key = cb.getAttribute("data-rel") || "_generic";
          var shouldHide = hide.has(key);
          cb.checked = !shouldHide;
          cy.edges('edge[kind="crosslink"][relation="' + (cb.getAttribute("data-rel") || "") + '"]').toggleClass("hidden", shouldHide);
        });
      }
      renderTree();
      if (!clustered) {
        cy.nodes('[kind="concept"]').forEach(function (cn) {
          var n = nodes.find(function (x) { return x.id === cn.id(); });
          cn.style("display", n && matches(n) ? "element" : "none");
        });
      }
      if (st.l) runLayout(st.l);
    } finally { applyingState = false; }
    if (st.n && docs[st.n]) select(st.n);
  }

  // ---- path tracing over typed edges ----
  var traceMode = false, traceA = null;
  var traceBtn = document.getElementById("trace");
  if (traceBtn) {
    traceBtn.addEventListener("click", function () {
      traceMode = true; traceA = null;
      traceBtn.classList.add("active");
      traceBtn.textContent = "Trace: pick A";
    });
  }
  function handleTraceTap(n) {
    if (!traceA) {
      traceA = n.id();
      traceBtn.textContent = "Trace: pick B";
      return true;
    }
    var dj = cy.elements().dijkstra({ root: cy.getElementById(traceA), directed: false });
    var path = dj.pathTo(n);
    cy.elements().removeClass("traced");
    cy.nodes().addClass("dim");
    if (path && path.length) {
      path.removeClass("dim").addClass("traced");
    }
    traceMode = false; traceA = null;
    traceBtn.classList.remove("active");
    traceBtn.textContent = "Trace";
    return true;
  }

  // hop slider re-applies the spotlight for the current selection.
  var hopsEl = document.getElementById("hops");
  if (hopsEl) hopsEl.addEventListener("change", function () {
    if (selectedId) { var n = cy.getElementById(selectedId); if (n.nonempty()) applySpotlight(n); }
  });
  // re-write the hash when the layout changes too.
  document.getElementById("layout").addEventListener("change", writeHash);

  // ---- init ----
  renderTree();
  window.addEventListener("hashchange", function () { applyState(readHash()); });
  applyState(readHash());

  function escapeHtml(s) { return String(s).replace(/[&<>]/g, function (c) { return { "&": "&amp;", "<": "&lt;", ">": "&gt;" }[c]; }); }
})();
