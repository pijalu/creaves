package actions

// Builder round-trip tests for the visual matcher builder (bugs.md U18/U27
// R5-1b): the flat-DSL ⇄ visual parser/serializer shipped inside
// templates/care_matchers/_builder.plush.html must parse and re-serialize
// the explicit case-insensitive operators (=*, !=*, INCI) byte-identically,
// keep CS spellings untouched, and keep the raw-mode fallback for
// constructs the visual model cannot represent (nested groups).
//
// The builder is browser JS; the test executes the real script in node
// (skipped when node is unavailable — the e2e gate covers the DOM wiring).

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var builderScriptRe = regexp.MustCompile(`(?s)<script>\n(.*?)\n</script>`)

func TestMatcherBuilderRoundTripCI(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available — builder JS covered by the e2e gate")
	}
	raw, err := os.ReadFile("../templates/care_matchers/_builder.plush.html")
	if err != nil {
		t.Fatalf("read builder template: %v", err)
	}
	m := builderScriptRe.FindSubmatch(raw)
	if m == nil {
		t.Fatalf("builder <script> block not found in template")
	}

	driver := `
globalThis.window = {};
` + string(m[1]) + `
const mb = window.matcherBuilder;
if (!mb || !mb.parseFlat || !mb.serialize) { throw new Error('parseFlat/serialize not exported'); }

const cases = [
  ['cage =* "X"', 'cage =* "X"'],
  ['cage = "X"', 'cage = "X"'],
  ['zone INCI ("E", "F")', 'zone INCI ("E", "F")'],
  ['cage inci ("A")', 'cage INCI ("A")'],            // keyword case canonicalized
  ['cage !=* "VE5"', 'cage !=* "VE5"'],
  ['species != "Renard roux"', 'species != "Renard roux"'],
  ['species = "A" AND cage =* "B"', 'species = "A" AND cage =* "B"'],
  ['NOT cage =* "enclos renards" OR cage IN ("VE5")', 'NOT cage =* "enclos renards" OR cage IN ("VE5")'],
  ['weight_g BETWEEN 100 AND 250', 'weight_g BETWEEN 100 AND 250'],
  ['species IN ("A", "B") AND cage INCI ("Enclos Renards")', 'species IN ("A", "B") AND cage INCI ("Enclos Renards")'],
];
for (const [src, want] of cases) {
  const parsed = mb.parseFlat(src);
  if (!parsed) { throw new Error('not representable (must parse): ' + src); }
  const out = mb.serialize(parsed);
  if (out !== want) { throw new Error('round-trip ' + JSON.stringify(src) + ' -> ' + JSON.stringify(out) + ', want ' + JSON.stringify(want)); }
}

// The parsed clause carries the exact CI op spelling.
const p = mb.parseFlat('cage =* "X"');
if (p.clauses[0].op !== '=*') { throw new Error('CI op lost, got ' + p.clauses[0].op); }
const p2 = mb.parseFlat('cage INCI ("A")');
if (p2.clauses[0].op !== 'INCI' || p2.clauses[0].values.length !== 1) { throw new Error('INCI clause misparsed'); }

// Unrepresentable constructs stay raw-mode (Postel guarantee, U18).
if (mb.parseFlat('(species = "A" OR cage = "B")') !== null) { throw new Error('paren group must not parse flat'); }
if (mb.parseFlat('species = "A" AND cage = "B" OR zone = "C"') !== null) { throw new Error('mixed combinators must not parse flat'); }

console.log('BUILDER_RT_OK');
`

	dir := t.TempDir()
	file := filepath.Join(dir, "builder_roundtrip.js")
	if err := os.WriteFile(file, []byte(driver), 0o644); err != nil {
		t.Fatalf("write driver: %v", err)
	}
	out, err := exec.Command(node, file).CombinedOutput()
	if err != nil {
		t.Fatalf("builder round-trip failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "BUILDER_RT_OK") {
		t.Fatalf("driver did not report success:\n%s", out)
	}
}
