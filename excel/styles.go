package excel

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
)

// This file works around excelize re-serializing xl/styles.xml on WriteTo
// (Bug 7): the rewritten root element replaces the template's namespace
// declarations with excelize's generic templateNamespaceIDMap (including a
// duplicate x15 token in mc:Ignorable) and rewrites many style constructs.
// Microsoft Excel rejects the part outright ("Removed Part: /xl/styles.xml")
// and then cascades repair prompts through worksheets and pivot tables.
// LibreOffice/openpyxl accept it, which is why only Excel users saw the bug.
//
// The fix replaces the serialized styles.xml in the final archive with the
// template's original part, appending only the extra cellXfs <xf> entries
// excelize legitimately created while writing cells (observed: the default
// date style for time.Time values written into unstyled cells). All other
// style sections are taken from the template verbatim — the export only ever
// writes values into cells styled by the template, plus these extras.
//
// A near-identical copy of this logic lives in creaves-console/excel;
// keep both in sync when changing the merge rules.

// stylesPartPath is the archive path of the styles document.
const stylesPartPath = "xl/styles.xml"

var (
	// cellXfsRe captures the cellXfs section (count attr in group 1,
	// inner XML in group 2).
	cellXfsRe = regexp.MustCompile(`(?s)<cellXfs(?:\s+count="(\d+)")?\s*>(.*?)</cellXfs>`)
	// numFmtsRe captures the optional numFmts section.
	numFmtsRe = regexp.MustCompile(`(?s)<numFmts(?:\s+count="(\d+)")?\s*>(.*?)</numFmts>`)
	// numFmtRe matches one numFmt element.
	numFmtRe = regexp.MustCompile(`<numFmt\s+numFmtId="(\d+)"\s+formatCode="([^"]*)"\s*/>`)
	// xfRe matches one xf element: either self-closed or with children.
	xfRe = regexp.MustCompile(`(?s)<xf\b[^>]*/>|<xf\b[^>]*>.*?</xf>`)
	// fontsRe/fillsRe/bordersRe capture the count attributes of the
	// sections whose indices extra xfs must stay within.
	fontsRe   = regexp.MustCompile(`<fonts\s+count="(\d+)"`)
	fillsRe   = regexp.MustCompile(`<fills\s+count="(\d+)"`)
	bordersRe = regexp.MustCompile(`<borders\s+count="(\d+)"`)
	// rootTagRe splits a styles document into prolog (1), root start
	// tag (2) and everything after it (3).
	rootTagRe = regexp.MustCompile(`(?s)^(.*?)(<styleSheet\b[^>]*>)(.*)$`)
	// numFmtIDAttrRe extracts the numFmtId attribute of an xf element.
	numFmtIDAttrRe = regexp.MustCompile(`\bnumFmtId="(\d+)"`)
	// emptyAlignmentRe matches an alignment element carrying no
	// attribute (excelize emits these on appended xfs).
	emptyAlignmentRe = regexp.MustCompile(`<alignment\s*/>|<alignment\s*>\s*</alignment>`)
	// emptyElementRe matches an xf left with no children after sanitizing.
	emptyElementRe = regexp.MustCompile(`(<xf\b[^>]*)>\s*</xf>`)
	// falseApplyRe matches apply* attributes excelize serializes as
	// "false" or "0" (false is the schema default; the attribute is noise).
	falseApplyRe = regexp.MustCompile(`\s+apply(?:NumberFormat|Font|Fill|Border|Alignment|Protection)="(?:false|0)"`)
	// trueApplyRe normalizes boolean apply* attributes to "1".
	trueApplyRe = regexp.MustCompile(`(\s+apply(?:NumberFormat|Font|Fill|Border|Alignment|Protection)=")true"`)
)

// templateStylesXML reads xl/styles.xml from an embedded template archive.
func templateStylesXML(template string) ([]byte, error) {
	file, err := excelConfig.Open("config/" + template)
	if err != nil {
		return nil, fmt.Errorf("error opening template %s: %w", template, err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("template %s is not a valid xlsx: %w", template, err)
	}
	for _, f := range zr.File {
		if f.Name != stylesPartPath {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		return b, nil
	}
	return nil, fmt.Errorf("%s not found in template %s", stylesPartPath, template)
}

// countOf returns the integer count attribute matched by re, or -1 when the
// section is absent.
func countOf(re *regexp.Regexp, doc []byte) int {
	m := re.FindSubmatch(doc)
	if m == nil {
		return -1
	}
	n, err := strconv.Atoi(string(m[1]))
	if err != nil {
		return -1
	}
	return n
}

// sectionSpan locates the section matched by re inside doc and returns the
// offsets of the section start, the end of its opening tag and the start of
// its closing tag.
func sectionSpan(re *regexp.Regexp, doc []byte, closeTag string) (start, openEnd, innerEnd int, ok bool) {
	loc := re.FindStringIndex(string(doc))
	if loc == nil {
		return 0, 0, 0, false
	}
	rel := bytes.IndexByte(doc[loc[0]:loc[1]], '>')
	if rel < 0 || loc[1]-loc[0] < len(closeTag) {
		return 0, 0, 0, false
	}
	return loc[0], loc[0] + rel + 1, loc[1] - len(closeTag), true
}

// mergeStylesXML builds the styles.xml to ship in the export: the template
// document (root element, namespaces, mc:Ignorable and every section)
// verbatim, plus the cellXfs <xf> entries the serialized document carries
// beyond the template count. It returns an error when the serialized
// document added fonts, fills or borders (extra xfs would reference indices
// the template does not define) — the caller then keeps the excelize
// document rather than produce an invalid file.
func mergeStylesXML(template, serialized []byte) ([]byte, error) {
	tplRoot := rootTagRe.FindSubmatch(template)
	if tplRoot == nil {
		return nil, fmt.Errorf("template styles.xml has no styleSheet root")
	}
	tplCellXfsCount := countOf(cellXfsRe, template)
	if tplCellXfsCount < 0 {
		return nil, fmt.Errorf("template styles.xml has no cellXfs section")
	}
	serCellXfs := cellXfsRe.FindSubmatch(serialized)
	if serCellXfs == nil {
		// excelize dropped cellXfs entirely: the template part is valid
		// as-is (no cell style can reference an entry beyond the count).
		return template, nil
	}

	// Safety check (plan step 4): extra xfs must not reference fonts,
	// fills or borders beyond the template's — those sections are kept
	// from the template verbatim.
	for _, sec := range []struct {
		name string
		re   *regexp.Regexp
	}{{"fonts", fontsRe}, {"fills", fillsRe}, {"borders", bordersRe}} {
		if tplN, serN := countOf(sec.re, template), countOf(sec.re, serialized); serN > tplN {
			return nil, fmt.Errorf("serialized styles.xml added %s (%d > template %d)", sec.name, serN, tplN)
		}
	}

	serXfs := xfRe.FindAll(serCellXfs[2], -1)
	if len(serXfs) <= tplCellXfsCount {
		// excelize did not append any style: the template part is valid
		// as-is.
		return template, nil
	}
	extras := serXfs[tplCellXfsCount:]

	// Remap custom numFmts referenced by the extras (plan step 3).
	numFmtInsert, remap, err := resolveExtraNumFmts(template, serialized, extras)
	if err != nil {
		return nil, err
	}

	var extraXML bytes.Buffer
	for _, xf := range extras {
		extraXML.Write(sanitizeExtraXF(xf, remap))
	}

	// Reassemble on the template bytes: prolog and root start tag are
	// copied verbatim (this is what stops the Excel repair prompt), the
	// cellXfs count is adjusted and the sanitized extras are appended.
	body := tplRoot[3]
	if len(numFmtInsert) > 0 {
		body = appendNumFmts(body, numFmtInsert)
	}
	start, openEnd, innerEnd, ok := sectionSpan(cellXfsRe, body, "</cellXfs>")
	if !ok {
		return nil, fmt.Errorf("template styles.xml cellXfs section vanished")
	}

	var out bytes.Buffer
	out.Grow(len(template) + extraXML.Len() + 64)
	out.Write(tplRoot[1])
	out.Write(tplRoot[2])
	out.Write(body[:start])
	out.WriteString(`<cellXfs count="`)
	out.WriteString(strconv.Itoa(tplCellXfsCount + len(extras)))
	out.WriteString(`">`)
	out.Write(body[openEnd:innerEnd])
	out.Write(extraXML.Bytes())
	out.Write(body[innerEnd:])
	return out.Bytes(), nil
}

// numFmtTable indexes numFmt elements by id and formatCode.
type numFmtTable struct {
	byID   map[int]string
	byCode map[string]int
	maxID  int
}

// indexNumFmts builds the table of numFmt elements of a styles document.
func indexNumFmts(doc []byte) numFmtTable {
	t := numFmtTable{byID: map[int]string{}, byCode: map[string]int{}}
	for _, m := range numFmtRe.FindAllSubmatch(doc, -1) {
		id, _ := strconv.Atoi(string(m[1]))
		code := string(m[2])
		t.byID[id] = code
		t.byCode[code] = id
		if id > t.maxID {
			t.maxID = id
		}
	}
	return t
}

// resolveExtraNumFmts inspects the numFmtId attributes of the extra xfs.
// Built-in ids (< 164) are kept as-is. Custom ids must exist in the merged
// document: when the template already defines a numFmt with the same
// formatCode the xf is remapped to it, otherwise the serialized numFmt is
// appended with a fresh id (max known id + 1). It returns the numFmt
// elements to insert and the id remapping table.
func resolveExtraNumFmts(template, serialized []byte, extras [][]byte) ([][]byte, map[int]int, error) {
	tpl := indexNumFmts(template)
	ser := indexNumFmts(serialized)
	if ser.maxID > tpl.maxID {
		tpl.maxID = ser.maxID
	}

	var insert [][]byte
	remap := map[int]int{}
	for _, id := range extraNumFmtIDs(extras) {
		code, ok := ser.byID[id]
		if !ok {
			return nil, nil, fmt.Errorf("extra xf references unknown numFmtId %d", id)
		}
		if tplID, ok := tpl.byCode[code]; ok {
			remap[id] = tplID
			continue
		}
		tpl.maxID++
		remap[id] = tpl.maxID
		tpl.byCode[code] = tpl.maxID
		insert = append(insert, []byte(fmt.Sprintf(`<numFmt numFmtId="%d" formatCode="%s"/>`, tpl.maxID, code)))
	}
	return insert, remap, nil
}

// extraNumFmtIDs returns the distinct custom (>= 164) numFmt ids referenced
// by the extra xfs, in first-use order.
func extraNumFmtIDs(extras [][]byte) []int {
	seen := map[int]bool{}
	var ids []int
	for _, xf := range extras {
		m := numFmtIDAttrRe.FindSubmatch(xf)
		if m == nil {
			continue
		}
		id, _ := strconv.Atoi(string(m[1]))
		if id < 164 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

// appendNumFmts inserts numFmt elements into the template body: inside the
// existing numFmts section when present (count adjusted), otherwise as a new
// section right at the start of the body (numFmts is the first element of
// styleSheet per the schema).
func appendNumFmts(body []byte, numFmts [][]byte) []byte {
	var out bytes.Buffer
	if start, openEnd, innerEnd, ok := sectionSpan(numFmtsRe, body, "</numFmts>"); ok {
		count := countOf(numFmtsRe, body) + len(numFmts)
		out.Write(body[:start])
		out.WriteString(`<numFmts count="`)
		out.WriteString(strconv.Itoa(count))
		out.WriteString(`">`)
		out.Write(body[openEnd:innerEnd])
		for _, nf := range numFmts {
			out.Write(nf)
		}
		out.WriteString(`</numFmts>`)
		out.Write(body[innerEnd+len("</numFmts>"):])
		return out.Bytes()
	}
	out.WriteString(`<numFmts count="`)
	out.WriteString(strconv.Itoa(len(numFmts)))
	out.WriteString(`">`)
	for _, nf := range numFmts {
		out.Write(nf)
	}
	out.WriteString(`</numFmts>`)
	out.Write(body)
	return out.Bytes()
}

// sanitizeExtraXF normalizes one appended xf element for the template
// document: empty <alignment/> children are dropped, apply* attributes that
// are false are removed, boolean "true" values are normalized to "1" and the
// numFmtId is remapped when needed.
func sanitizeExtraXF(xf []byte, remap map[int]int) []byte {
	out := emptyAlignmentRe.ReplaceAll(xf, nil)
	out = falseApplyRe.ReplaceAll(out, nil)
	out = trueApplyRe.ReplaceAll(out, []byte(`${1}1"`))
	out = emptyElementRe.ReplaceAll(out, []byte(`${1}/>`))
	if m := numFmtIDAttrRe.FindSubmatch(out); m != nil {
		id, _ := strconv.Atoi(string(m[1]))
		if newID, ok := remap[id]; ok && newID != id {
			out = numFmtIDAttrRe.ReplaceAll(out, []byte(`numFmtId="`+strconv.Itoa(newID)+`"`))
		}
	}
	return out
}

// restoreStylesInZip replaces xl/styles.xml in the final archive with the
// merged template document. It returns an error when the merge is unsafe —
// the caller logs and ships the excelize output unchanged.
func restoreStylesInZip(xlsx, templateStyles []byte) ([]byte, error) {
	parts, order, err := readZipParts(xlsx)
	if err != nil {
		return nil, err
	}
	serialized, ok := parts[stylesPartPath]
	if !ok {
		return nil, fmt.Errorf("%s not found in archive", stylesPartPath)
	}
	merged, err := mergeStylesXML(templateStyles, serialized)
	if err != nil {
		return nil, err
	}
	parts[stylesPartPath] = merged
	return writeZipParts(parts, order)
}
