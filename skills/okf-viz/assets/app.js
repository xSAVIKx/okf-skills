(function () {
  "use strict";
  var data = JSON.parse(document.getElementById("okf-data").textContent || "{}");
  var nodes = data.nodes || [], edges = data.edges || [], docs = data.docs || {};

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
  var elements = [];
  nodes.forEach(function (n) {
    elements.push({ data: { id: n.id, label: n.title, kind: n.kind, ntype: n.type || "", degree: n.degree || 1 } });
  });
  edges.forEach(function (e) {
    elements.push({ data: { id: e.source + "|" + e.target + "|" + e.kind, source: e.source, target: e.target, kind: e.kind } });
  });

  var palette = ["#4f86c6","#3fae6b","#c6864f","#9b59b6","#e0556e","#1abc9c","#e08e3f","#7f8c8d"];
  var typeColors = {}; var ci = 0;
  nodes.forEach(function (n) { if (n.kind === "concept" && n.type && !(n.type in typeColors)) typeColors[n.type] = palette[ci++ % palette.length]; });

  var cy = window.cy = cytoscape({
    container: document.getElementById("graph"),
    elements: elements,
    minZoom: 0.1, maxZoom: 4,
  });
  styleGraph(cy);
  cy.on("tap", "node", function (evt) { select(evt.target.id()); });
  runLayout(document.getElementById("layout").value);
  addLegend();

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
      { selector: 'edge[kind="containment"]', style: {
        "width": 1.5, "line-color": structural, "line-style": "dashed", "curve-style": "straight" } },
      { selector: 'edge[kind="crosslink"]', style: {
        "width": 2, "line-color": crosslink, "curve-style": "bezier",
        "target-arrow-color": crosslink, "target-arrow-shape": "triangle" } },
      { selector: "edge.hidden", style: { "display": "none" } },
    ]);
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

  // edge-kind toggles via legend checkboxes
  function addLegend() {
    var el = document.createElement("div"); el.className = "legend";
    el.innerHTML =
      '<label><input type="checkbox" id="show-cont" checked> structure</label> ' +
      '<label><input type="checkbox" id="show-link" checked> cross-links</label>';
    document.getElementById("graph").appendChild(el);
    el.querySelector("#show-cont").addEventListener("change", function (e) {
      cy.edges('[kind="containment"]').toggleClass("hidden", !e.target.checked);
    });
    el.querySelector("#show-link").addEventListener("change", function (e) {
      cy.edges('[kind="crosslink"]').toggleClass("hidden", !e.target.checked);
    });
  }

  // ---- reader ----
  function select(id) {
    cy.nodes().removeClass("selected dim");
    var n = cy.getElementById(id);
    if (n.nonempty()) {
      n.addClass("selected");
      var keep = n.closedNeighborhood().nodes();
      cy.nodes().not(keep).addClass("dim");
      cy.animate({ center: { eles: n } }, { duration: 200 });
    }
    document.querySelectorAll("#tree li").forEach(function (li) { li.classList.toggle("selected", li.dataset.id === id); });
    var d = docs[id]; var r = document.getElementById("reader");
    if (!d) { r.innerHTML = '<p class="empty">' + escapeHtml(id) + '</p>'; }
    else {
      var fm = [d.type, (d.tags || []).join(", "), d.timestamp].filter(Boolean).join(" · ");
      r.innerHTML =
        "<h1>" + escapeHtml(d.title) + "</h1>" +
        (fm ? '<div class="fm">' + escapeHtml(fm) + "</div>" : "") +
        (d.description ? "<p>" + escapeHtml(d.description) + "</p>" : "") +
        (d.resource ? '<div class="fm">resource: <code>' + escapeHtml(d.resource) + "</code></div>" : "") +
        d.bodyHtml;
      // intra-bundle links navigate within the viewer
      r.querySelectorAll('a[href$=".md"]').forEach(function (a) {
        a.addEventListener("click", function (ev) {
          ev.preventDefault();
          var t = resolveHref(id, a.getAttribute("href"));
          if (docs[t]) location.hash = encodeURIComponent(t);
        });
      });
    }
    if (location.hash.slice(1) !== encodeURIComponent(id)) location.hash = encodeURIComponent(id);
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
  function renderTree(filterFn) {
    tree.innerHTML = "";
    nodes.filter(function (n) { return n.kind === "concept"; })
      .filter(filterFn)
      .sort(function (a, b) { return a.id < b.id ? -1 : 1; })
      .forEach(function (n) {
        var li = document.createElement("li");
        li.textContent = n.id; li.dataset.id = n.id; li.title = n.description || "";
        li.addEventListener("click", function () { select(n.id); });
        tree.appendChild(li);
      });
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
  function matches(n) {
    var q = searchEl.value.toLowerCase();
    var typeOk = !n.type || active.has(n.type);
    var qOk = !q || (n.id + " " + n.title + " " + (n.description || "")).toLowerCase().indexOf(q) >= 0;
    return typeOk && qOk;
  }
  function applyFilter() {
    renderTree(matches);
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

  // ---- init ----
  renderTree(matches);
  window.addEventListener("hashchange", function () {
    var id = decodeURIComponent(location.hash.slice(1));
    if (id && docs[id]) select(id);
  });
  var startId = decodeURIComponent(location.hash.slice(1));
  if (startId && docs[startId]) select(startId);

  function escapeHtml(s) { return String(s).replace(/[&<>]/g, function (c) { return { "&": "&amp;", "<": "&lt;", ">": "&gt;" }[c]; }); }
})();
