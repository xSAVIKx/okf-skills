package main

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	okf "github.com/xSAVIKx/okf-skills/okf-go"
)

// coChangeIndex counts how often each pair of files changed together across the
// repository's commit history. It is the deterministic, mechanical signal behind
// a file's "# Related Files" section — no LLM, no source-code parsing, just which
// files tend to be committed in the same commit.
type coChangeIndex struct {
	// counts[a][b] = number of commits that touched both a and b (a != b).
	counts map[string]map[string]int
}

// buildCoChangeIndex walks the commit history reachable from HEAD and tallies
// pairwise co-change counts for every commit's set of changed files. Files the
// ignore matcher rejects are excluded so generated/ignored paths never appear as
// partners. Commits whose stats cannot be computed are skipped rather than fatal.
func buildCoChangeIndex(repo *git.Repository, im *gitIgnoreMatcher) (*coChangeIndex, error) {
	idx := &coChangeIndex{counts: map[string]map[string]int{}}

	cIter, err := repo.Log(&git.LogOptions{})
	if err != nil {
		return nil, err
	}
	defer cIter.Close()

	err = cIter.ForEach(func(c *object.Commit) error {
		stats, err := c.Stats()
		if err != nil {
			return nil // skip commits whose diff cannot be computed (e.g. binary edge cases)
		}
		var files []string
		seen := map[string]bool{}
		for _, s := range stats {
			rel := filepath.ToSlash(s.Name)
			if rel == "" || im.Matches(rel) || seen[rel] {
				continue
			}
			seen[rel] = true
			files = append(files, rel)
		}
		for i := 0; i < len(files); i++ {
			for j := i + 1; j < len(files); j++ {
				idx.add(files[i], files[j])
				idx.add(files[j], files[i])
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return idx, nil
}

// add increments the co-change count for the ordered pair (a -> b).
func (idx *coChangeIndex) add(a, b string) {
	if idx.counts[a] == nil {
		idx.counts[a] = map[string]int{}
	}
	idx.counts[a][b]++
}

// relationshipsFor returns the co-change edges for a single file: partners that
// co-changed at least min times, ranked by descending frequency (ties broken by
// name for determinism) and capped at topN, then mapped to bundle-relative links
// (`/<partner>.md`). exists, when non-nil, filters out partners that did not
// produce a concept doc (e.g. files removed from the current tree). The final
// render order is handled by okf.RenderRelationshipsSection; this ordering only
// governs which partners survive the topN cut.
func (idx *coChangeIndex) relationshipsFor(file string, min, topN int, exists func(concept string) bool) []okf.Relationship {
	partners := idx.counts[file]
	if len(partners) == 0 {
		return nil
	}

	type pc struct {
		name string
		n    int
	}
	var list []pc
	for name, n := range partners {
		if n < min {
			continue
		}
		if exists != nil && !exists(name) {
			continue
		}
		list = append(list, pc{name, n})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].n != list[j].n {
			return list[i].n > list[j].n
		}
		return list[i].name < list[j].name
	})
	if topN > 0 && len(list) > topN {
		list = list[:topN]
	}

	rels := make([]okf.Relationship, 0, len(list))
	for _, p := range list {
		rels = append(rels, okf.Relationship{
			Label:  fmt.Sprintf("co-changed (%dx)", p.n),
			Target: "/" + p.name + ".md",
			Text:   p.name,
		})
	}
	return rels
}
