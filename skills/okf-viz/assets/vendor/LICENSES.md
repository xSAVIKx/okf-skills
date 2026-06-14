# Vendored JavaScript Libraries

All libraries are pinned to exact versions and licensed under the MIT License.

| File | Package | Version | License | CDN URL |
|------|---------|---------|---------|---------|
| `cytoscape.min.js` | cytoscape | 3.30.2 | MIT | https://cdn.jsdelivr.net/npm/cytoscape@3.30.2/dist/cytoscape.min.js |
| `layout-base.min.js` | layout-base | 2.0.1 | MIT | https://cdn.jsdelivr.net/npm/layout-base@2.0.1/layout-base.min.js |
| `cose-base.min.js` | cose-base | 2.2.0 | MIT | https://cdn.jsdelivr.net/npm/cose-base@2.2.0/cose-base.min.js |
| `cytoscape-fcose.min.js` | cytoscape-fcose | 2.2.0 | MIT | https://cdn.jsdelivr.net/npm/cytoscape-fcose@2.2.0/cytoscape-fcose.min.js |
| `cola.min.js` | webcola | 3.4.0 | MIT | https://cdn.jsdelivr.net/npm/webcola@3.4.0/WebCola/cola.min.js |
| `cytoscape-cola.min.js` | cytoscape-cola | 2.5.1 | MIT | https://cdn.jsdelivr.net/npm/cytoscape-cola@2.5.1/cytoscape-cola.min.js |
| `dagre.min.js` | dagre | 0.8.5 | MIT | https://cdn.jsdelivr.net/npm/dagre@0.8.5/dist/dagre.min.js |
| `cytoscape-dagre.min.js` | cytoscape-dagre | 2.5.0 | MIT | https://cdn.jsdelivr.net/npm/cytoscape-dagre@2.5.0/cytoscape-dagre.min.js |

## Load Order

The files must be loaded in dependency-first order (as defined by `vendorOrder` in `render.go`):

1. `cytoscape.min.js` — Cytoscape.js core graph library
2. `layout-base.min.js` — Base layout utilities (required by cose-base and fcose)
3. `cose-base.min.js` — CoSE layout base (required by fcose)
4. `cytoscape-fcose.min.js` — fCoSE force-directed layout for Cytoscape
5. `cola.min.js` — WebCola constraint-based layout engine (required by cytoscape-cola)
6. `cytoscape-cola.min.js` — Cola layout adapter for Cytoscape
7. `dagre.min.js` — Dagre directed-acyclic graph layout engine (required by cytoscape-dagre)
8. `cytoscape-dagre.min.js` — Dagre layout adapter for Cytoscape

## SRI Hashes (sha384)

These hashes match the files in this directory and the `integrity=` attributes in `cdn.go`:

- `cytoscape.min.js`: `sha384-IWROdLKRsN1UuJywMlWl7/blXQ8GEooN2n7dzTxfEPd7ybYIKCUJ2Ol/1Gpf3YV4`
- `layout-base.min.js`: `sha384-wORSveLcAX75yM0BmukpnoPBNNhzBkTW19ggbZt2Adj/OGO871ZAiQAuHUDO9OV7`
- `cose-base.min.js`: `sha384-UrN5MK6+mjwxHnGlBPp2bUV0WkAYIGjmrx8C35EV5z7mAyfeIPMsJg4AnmTOjL3T`
- `cytoscape-fcose.min.js`: `sha384-Z4ysnuh0vXITdK1HwTvkKEhx03x06ZvweXnxnPvV0xKagye5YfD6ad/MJWybSpm0`
- `cola.min.js`: `sha384-o4yPeUKY7q5q4fuMcFuJWSBJPJgSHtssnfVZvjNRGOEuBwT8zxXnzyGJcy5Ojpeo`
- `cytoscape-cola.min.js`: `sha384-WWuGu1EcZ0HZKT1myqP0xQf4g0nAYz9bjgbrDr/QUXWC0vD6RFcAwFopc55Gkub/`
- `dagre.min.js`: `sha384-2IH3T69EIKYC4c+RXZifZRvaH5SRUdacJW7j6HtE5rQbvLhKKdawxq6vpIzJ7j9M`
- `cytoscape-dagre.min.js`: `sha384-EHCdyFVbhtbpgI+4x7ETlZUvJwOkxJublmhTpH114NSk3fqfiUgcLl6pQm8JQwg9`
