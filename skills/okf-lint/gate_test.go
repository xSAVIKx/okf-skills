package main

import (
	"testing"

	"github.com/xSAVIKx/okf-skills/okf-go"
)

func TestGateFailures(t *testing.T) {
	clean := &okf.LintReport{TotalConcepts: 2, EnrichedConcepts: 2, EnrichedPct: 100}
	if f := gateFailures(clean, gateOpts{requireTypes: true}); len(f) != 0 {
		t.Errorf("clean bundle should pass, got %v", f)
	}

	// structural conformance violation always fails
	rep := &okf.LintReport{Conformance: []okf.Finding{{Rule: okf.RuleSubdirIndexFM}}}
	if f := gateFailures(rep, gateOpts{requireTypes: true}); len(f) != 1 {
		t.Errorf("expected 1 failure for structural violation, got %v", f)
	}

	// missing-type gated by requireTypes
	mt := &okf.LintReport{Conformance: []okf.Finding{{Rule: okf.RuleMissingType}}}
	if f := gateFailures(mt, gateOpts{requireTypes: true}); len(f) != 1 {
		t.Errorf("require-types should fail on missing type, got %v", f)
	}
	if f := gateFailures(mt, gateOpts{requireTypes: false}); len(f) != 0 {
		t.Errorf("require-types=false should not fail on missing type, got %v", f)
	}

	// broken links vs max
	bl := &okf.LintReport{BrokenLinks: []string{"a -> b", "c -> d"}}
	if f := gateFailures(bl, gateOpts{maxBroken: 0}); len(f) != 1 {
		t.Errorf("expected broken-link failure, got %v", f)
	}
	if f := gateFailures(bl, gateOpts{maxBroken: 2}); len(f) != 0 {
		t.Errorf("max-broken-links=2 should tolerate 2 broken links, got %v", f)
	}

	// enrichment threshold
	low := &okf.LintReport{EnrichedPct: 50}
	if f := gateFailures(low, gateOpts{minPct: 90}); len(f) != 1 {
		t.Errorf("expected enrichment-threshold failure, got %v", f)
	}

	// strict orphans
	orph := &okf.LintReport{Orphans: []string{"x"}}
	if f := gateFailures(orph, gateOpts{}); len(f) != 0 {
		t.Errorf("orphans should not fail without --strict, got %v", f)
	}
	if f := gateFailures(orph, gateOpts{strict: true}); len(f) != 1 {
		t.Errorf("--strict should fail on orphans, got %v", f)
	}
}
