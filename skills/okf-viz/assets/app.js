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
      els.push({ data: { id: n.id, label: n.title, kind: n.kind, ntype: n.type || "", degree: n.degree || 1, coverage: n.coverage || "", diff: n.diff || "" } });
    });
    edges.forEach(function (e) {
      var rel = e.relation || "";
      els.push({ data: { id: e.source + "|" + e.target + "|" + e.kind + "|" + rel + "|" + (e.diff || ""), source: e.source, target: e.target, kind: e.kind, relation: rel, diff: e.diff || "" } });
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

  // ---- deferred graph start ----
  // A force layout of a large graph takes seconds of CPU time. The page does not
  // do it before first paint: the navigator, search and reader work at once,
  // and the graph appears when its positions are ready. The default fcose
  // layout runs in a Web Worker (built from the same inlined or SRI-checked
  // library code) so the main thread stays free. If a worker is not available,
  // the layout runs on the main thread after first paint.
  var cy = null;          // created by initGraph when node positions are known
  var appliedLayout = ""; // name of the layout last run on the graph
  var layoutSel = document.getElementById("layout");
  // graph-only controls stay disabled until the graph exists.
  var graphControls = ["layout", "hops", "trace", "er-mode", "cluster", "coverage-overlay", "export-png", "export-json"];
  setGraphControls(false);
  var loadingNote = document.createElement("p");
  loadingNote.className = "graph-loading";
  loadingNote.setAttribute("role", "status");
  loadingNote.textContent = "Laying out the graph…";
  document.getElementById("graph").appendChild(loadingNote);

  function setGraphControls(on) {
    graphControls.forEach(function (id) {
      var el = document.getElementById(id);
      if (el) el.disabled = !on;
    });
  }

  function startGraph(name) {
    var els = baseElements();
    var started = false;
    function begin(positions) {
      if (started) return;
      started = true;
      // The user or a permalink can pick another layout while the worker runs.
      // Then the worker positions are for the wrong layout: lay out again.
      if (layoutSel.value !== name) positions = null;
      initGraph(els, positions, name);
    }
    function onMainThread() {
      // Two frames: let the browser paint the page before the layout work.
      requestAnimationFrame(function () { setTimeout(function () { begin(null); }, 0); });
    }
    if (name === "fcose") layoutInWorker(els, function (positions) { if (positions) begin(positions); else onMainThread(); });
    else onMainThread();
  }

  function initGraph(els, positions, name) {
    if (positions) {
      els.forEach(function (el) {
        var p = !el.data.source && positions[el.data.id];
        if (p) el.position = { x: p.x, y: p.y };
      });
    }
    cy = window.cy = cytoscape({
      container: document.getElementById("graph"),
      elements: els,
      style: graphStyle(),
      layout: positions ? { name: "preset", fit: true, padding: 30 } : undefined,
      minZoom: 0.1, maxZoom: 4,
    });
    if (positions) appliedLayout = name;
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
    if (loadingNote.parentNode) loadingNote.parentNode.removeChild(loadingNote);
    setGraphControls(true);
    // Apply the UI state that the user (or the permalink) set before the graph existed.
    applyEdgeToggles();
    applyDisplay();
    if (selectedId) highlightSelection(selectedId, false);
    // The first layout has nothing to animate from, so it does not animate.
    if (!positions) runLayout(layoutSel.value, { animate: false });
  }

  // layoutInWorker runs the fcose layout in a Web Worker and returns the node
  // positions, or null when a worker cannot run it (the caller then lays out on
  // the main thread). The worker gets the library code that the page already
  // has: the inline source (--offline) or the CDN file fetched with its SRI hash.
  function layoutInWorker(els, cb) {
    if (!window.Worker || !window.Blob || !window.URL || !window.Promise) { cb(null); return; }
    var names = ["cytoscape.min.js", "layout-base.min.js", "cose-base.min.js", "cytoscape-fcose.min.js"];
    var parts;
    try {
      parts = names.map(function (name) {
        var el = document.querySelector('script[data-okf-lib="' + name + '"]');
        if (!el) throw new Error("missing " + name);
        if (!el.src) return Promise.resolve(el.textContent);
        if (!window.fetch) throw new Error("no fetch");
        return fetch(el.src, { integrity: el.integrity || "", mode: "cors", credentials: "omit" })
          .then(function (r) { if (!r.ok) throw new Error("status " + r.status); return r.text(); });
      });
    } catch (e) { cb(null); return; }
    Promise.all(parts).then(function (srcs) {
      var url = URL.createObjectURL(new Blob([srcs.join("\n;\n") + "\n;(" + layoutWorkerMain.toString() + ")();\n"], { type: "text/javascript" }));
      var worker, done = false;
      function finish(positions) {
        if (done) return;
        done = true;
        try { worker.terminate(); } catch (e) { /* already gone */ }
        URL.revokeObjectURL(url);
        cb(positions);
      }
      try { worker = new Worker(url); } catch (e) { URL.revokeObjectURL(url); cb(null); return; }
      worker.onmessage = function (ev) { finish(ev.data && ev.data.positions ? ev.data.positions : null); };
      worker.onerror = function (ev) { if (ev && ev.preventDefault) ev.preventDefault(); finish(null); };
      worker.postMessage({ elements: els.map(function (el) {
        return { data: el.data.source ? { id: el.data.id, source: el.data.source, target: el.data.target } : { id: el.data.id, degree: el.data.degree } };
      }) });
    }, function () { cb(null); });
  }

  // layoutWorkerMain is the worker entry point. It is serialized into the worker
  // source, so it must not use anything from the enclosing scope. Node sizes
  // match the page style (mapData on degree), so the layout spacing is the same.
  function layoutWorkerMain() {
    self.onmessage = function (e) {
      try {
        var g = cytoscape({
          headless: true, styleEnabled: true, elements: e.data.elements,
          style: [{ selector: "node", style: { "width": "mapData(degree, 1, 12, 14, 44)", "height": "mapData(degree, 1, 12, 14, 44)" } }],
        });
        g.layout({ name: "fcose", animate: false, fit: false }).run();
        var out = {};
        g.nodes().forEach(function (n) { var p = n.position(); out[n.id()] = { x: p.x, y: p.y }; });
        self.postMessage({ positions: out });
      } catch (err) {
        self.postMessage({ error: String(err) });
      }
    };
  }

  // ---- lazy layout libraries ----
  // cola and dagre are inert <script type="text/plain"> blocks until their
  // layout is first used; ensureLayout runs them (inline source, or a CDN
  // <script> with the same SRI hash) and then calls cb.
  var lazyLibs = { cola: ["cola.min.js", "cytoscape-cola.min.js"], dagre: ["dagre.min.js", "cytoscape-dagre.min.js"] };
  var lazyDone = {}, lazyWaiters = {};
  function ensureLayout(name, cb) {
    var files = lazyLibs[name];
    if (!files || lazyDone[name] === true) { cb(); return; }
    (lazyWaiters[name] = lazyWaiters[name] || []).push(cb);
    if (lazyDone[name] === "loading") return;
    lazyDone[name] = "loading";
    var i = 0;
    (function next() {
      if (i >= files.length) {
        lazyDone[name] = true;
        var waiting = lazyWaiters[name]; lazyWaiters[name] = [];
        waiting.forEach(function (f) { f(); });
        return;
      }
      var el = document.querySelector('script[data-okf-lib="' + files[i++] + '"]');
      if (!el || el.getAttribute("type") !== "text/plain" || el.getAttribute("data-okf-run")) { next(); return; }
      el.setAttribute("data-okf-run", "1");
      var s = document.createElement("script");
      var src = el.getAttribute("data-src");
      if (src) {
        var sri = el.getAttribute("data-integrity");
        if (sri) { s.setAttribute("integrity", sri); s.crossOrigin = "anonymous"; }
        s.onload = s.onerror = next;
        s.src = src;
        document.head.appendChild(s);
      } else {
        s.text = el.textContent;
        document.head.appendChild(s); // inline code runs synchronously
        next();
      }
    })();
  }

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
      if (cy) styleGraph(cy);
    });
  }
  function applyClusterDisplay() {
    if (!cy) return;
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
      if (!cy) return;
      cy.json({ elements: erMode ? erElements() : baseElements() });
      styleGraph(cy);
      runLayout(erMode ? "dagre" : layoutSel.value);
    });
  }

  function styleGraph(cy) { cy.style(graphStyle()); }

  function graphStyle() {
    var structural = cssVar("--containment") || "#c2c8d2";
    var crosslink = cssVar("--crosslink") || "#c64f86";
    var text = cssVar("--text") || "#1c2430";
    return [
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
    ].concat(relationStyles()).concat(coverageOverlay ? coverageStyles() : []).concat(diffStyles());
  }

  // coverageStyles recolors concept nodes by enrichment state; layered over the
  // type coloring only while the overlay is on, so default coloring is untouched.
  function coverageStyles() {
    return [
      { selector: 'node[coverage="placeholder"]', style: { "background-color": cssVar("--cov-placeholder") || "#d9534f" } },
      { selector: 'node[coverage="enriched"]', style: { "background-color": cssVar("--cov-enriched") || "#3fae6b" } },
    ];
  }

  // diffStyles highlights added/removed/changed elements when diff data is present
  // (always applied — diff fields are absent unless --diff was used).
  function diffStyles() {
    var added = cssVar("--diff-added") || "#3fae6b", removed = cssVar("--diff-removed") || "#d9534f", changed = cssVar("--diff-changed") || "#e0a106";
    return [
      { selector: 'node[diff="added"]', style: { "border-width": 4, "border-color": added } },
      { selector: 'node[diff="removed"]', style: { "border-width": 4, "border-color": removed, "border-style": "dashed", "opacity": 0.6 } },
      { selector: 'node[diff="changed"]', style: { "border-width": 4, "border-color": changed } },
      { selector: 'edge[diff="added"]', style: { "line-color": added, "width": 3 } },
      { selector: 'edge[diff="removed"]', style: { "line-color": removed, "line-style": "dashed", "opacity": 0.6 } },
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

  // runLayout runs a layout on the graph. o.animate === false skips the animation
  // (used for the first layout). Optional libraries load on first use.
  function runLayout(name, o) {
    var animate = !(o && o.animate === false);
    ensureLayout(name, function () {
      if (!cy) return;
      var opts = { name: name, animate: animate, fit: true, padding: 30 };
      if (name === "concentric") opts.concentric = function (n) { return n.data("degree"); };
      if (name === "breadthfirst" || name === "dagre") opts.roots = "#__root__";
      if (name === "dagre") { opts.rankDir = "TB"; }
      appliedLayout = name;
      try { cy.layout(opts).run(); }
      catch (e) { cy.layout({ name: "cose", animate: animate, fit: true }).run(); }
    });
  }
  layoutSel.addEventListener("change", function (e) { runLayout(e.target.value); });

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
    el.querySelector("#show-cont").addEventListener("change", applyEdgeToggles);
    el.querySelectorAll(".rel-toggle").forEach(function (cb) {
      cb.addEventListener("change", function () {
        applyEdgeToggles();
        writeHash();
      });
    });
  }
  // applyEdgeToggles hides the edges whose legend checkbox is clear.
  function applyEdgeToggles() {
    if (!cy) return;
    var cont = document.getElementById("show-cont");
    if (cont) cy.edges('[kind="containment"]').toggleClass("hidden", !cont.checked);
    document.querySelectorAll(".legend .rel-toggle").forEach(function (cb) {
      cy.edges('edge[kind="crosslink"][relation="' + (cb.getAttribute("data-rel") || "") + '"]').toggleClass("hidden", !cb.checked);
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
  // highlightSelection marks the selected node in the graph and dims the rest.
  function highlightSelection(id, animate) {
    if (!cy) return;
    cy.nodes().removeClass("selected");
    var n = cy.getElementById(id);
    if (n.nonempty()) {
      n.addClass("selected");
      applySpotlight(n);
      if (animate) cy.animate({ center: { eles: n } }, { duration: 200 });
      else cy.center(n);
    }
  }
  function select(id) {
    selectedId = id;
    if (cy) {
      highlightSelection(id, true);
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
    applyDisplay();
  }
  // applyDisplay shows the graph nodes that pass the type filters and search.
  function applyDisplay() {
    if (!cy) return;
    if (clustered) { applyClusterDisplay(); return; }
    var byId = {};
    nodes.forEach(function (x) { byId[x.id] = x; });
    cy.batch(function () {
      cy.nodes('[kind="concept"]').forEach(function (cn) {
        var n = byId[cn.id()];
        cn.style("display", n && matches(n) ? "element" : "none");
      });
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

  // ---- export (PNG + subgraph JSON) ----
  function download(url, name) {
    var a = document.createElement("a"); a.href = url; a.download = name;
    document.body.appendChild(a); a.click(); a.remove();
  }
  var pngBtn = document.getElementById("export-png");
  if (pngBtn) pngBtn.addEventListener("click", function () {
    if (!cy) return;
    try { download(cy.png({ full: true, scale: 2, bg: cssVar("--bg") || "#fff" }), "okf-graph.png"); } catch (e) { /* png unsupported */ }
  });
  var jsonBtn = document.getElementById("export-json");
  if (jsonBtn) jsonBtn.addEventListener("click", function () {
    if (!cy) return;
    var ids = {}; cy.nodes(":visible").forEach(function (n) { ids[n.id()] = true; });
    var exp = { nodes: [], edges: [] };
    cy.nodes(":visible").forEach(function (n) { exp.nodes.push(n.data()); });
    cy.edges().forEach(function (e) { if (ids[e.data("source")] && ids[e.data("target")]) exp.edges.push(e.data()); });
    download(URL.createObjectURL(new Blob([JSON.stringify(exp, null, 2)], { type: "application/json" })), "okf-subgraph.json");
  });

  // ---- a11y: keyboard traversal of the graph ----
  var graphEl = document.getElementById("graph");
  graphEl.addEventListener("keydown", function (ev) {
    if (ev.key !== "ArrowRight" && ev.key !== "ArrowLeft" && ev.key !== "Enter") return;
    if (!cy) return;
    var current = selectedId ? cy.getElementById(selectedId) : cy.nodes('[kind="concept"]').first();
    if (current.empty()) return;
    if (ev.key === "Enter") { select(current.id()); ev.preventDefault(); return; }
    var neighbors = current.closedNeighborhood().nodes().filter(function (n) { return n.id() !== current.id(); });
    if (neighbors.length) {
      var next = ev.key === "ArrowRight" ? neighbors.first() : neighbors.last();
      select(next.id());
      ev.preventDefault();
    }
  });

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
    var lay = layoutSel.value;
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
          cb.checked = !hide.has(cb.getAttribute("data-rel") || "_generic");
        });
        applyEdgeToggles();
      }
      renderTree();
      applyDisplay();
      // Run a layout only when it differs from the one on the graph.
      if (st.l && cy && st.l !== appliedLayout) runLayout(st.l);
    } finally { applyingState = false; }
    if (st.n && docs[st.n]) select(st.n);
  }

  // ---- path tracing over typed edges ----
  var traceMode = false, traceA = null;
  var traceBtn = document.getElementById("trace");
  if (traceBtn) {
    traceBtn.addEventListener("click", function () {
      if (!cy) return;
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
    if (selectedId && cy) { var n = cy.getElementById(selectedId); if (n.nonempty()) applySpotlight(n); }
  });
  // re-write the hash when the layout changes too.
  layoutSel.addEventListener("change", writeHash);

  // ---- init ----
  // The navigator, legend and permalink state come first; the graph follows
  // (see startGraph). applyState skips the graph parts until the graph exists,
  // and initGraph applies them.
  addLegend();
  renderTree();
  window.addEventListener("hashchange", function () { applyState(readHash()); });
  applyState(readHash());
  startGraph(layoutSel.value);

  function escapeHtml(s) { return String(s).replace(/[&<>]/g, function (c) { return { "&": "&amp;", "<": "&lt;", ">": "&gt;" }[c]; }); }
})();
