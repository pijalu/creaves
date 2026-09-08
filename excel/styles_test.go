package excel

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// testTemplateStyles is a minimal but Excel-shaped template styles part:
// small root element with mc:Ignorable and two custom numFmts.
const testTemplateStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" mc:Ignorable="x14ac xr xr9" xmlns:x14ac="http://schemas.microsoft.com/office/spreadsheetml/2009/9/ac" xmlns:xr="http://schemas.microsoft.com/office/spreadsheetml/2014/revision" xmlns:xr9="http://schemas.microsoft.com/office/spreadsheetml/2016/revision9"><numFmts count="2"><numFmt numFmtId="164" formatCode="General"/><numFmt numFmtId="165" formatCode="dd/mm/yyyy"/></numFmts><fonts count="2"><font><sz val="10"/><name val="Arial"/></font><font><b val="1"/><sz val="10"/><name val="Arial"/></font></fonts><fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills><borders count="1"><border/></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="2"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="165" fontId="1" fillId="0" borderId="0" xfId="0" applyNumberFormat="1" applyFont="1"/></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`

// testSerializedStyles mimics excelize's re-serialization: bloated root
// element with a duplicate mc:Ignorable token, rewritten constructs
// (b val="1", applyFont="true") and one extra cellXfs entry referencing a
// numFmt the template already defines under a different id.
const testSerializedStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" mc:Ignorable="x15 x14ac xr xr9 x15" xmlns:x15="http://x" xmlns:x14ac="http://y" xmlns:xr="http://z" xmlns:xr9="http://w"><numFmts count="3"><numFmt numFmtId="164" formatCode="General"/><numFmt numFmtId="166" formatCode="dd/mm/yyyy"/><numFmt numFmtId="167" formatCode="hh:mm"/></numFmts><fonts count="2"><font><sz val="10"/><name val="Arial"/></font><font><b val="1"/><sz val="10"/><name val="Arial"/></font></fonts><fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills><borders count="1"><border/></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="5"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="165" fontId="1" fillId="0" borderId="0" xfId="0" applyNumberFormat="true" applyFont="true"/><xf numFmtId="22" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="true"><alignment/></xf><xf numFmtId="166" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="true" applyAlignment="false"><alignment></alignment></xf><xf numFmtId="167" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="true"/></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`

// stylesRootTag extracts the styleSheet root start tag for comparison.
func stylesRootTag(t *testing.T, doc []byte) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)<styleSheet\b[^>]*>`).Find(doc)
	require.NotNil(t, m, "no styleSheet root in document")
	return string(m)
}

func TestMergeStylesXML_NoExtrasReturnsTemplateVerbatim(t *testing.T) {
	// Serialized document with the same cellXfs count as the template
	// (excelize rewrote the root but appended no style).
	ser := strings.Replace(testSerializedStyles,
		`<cellXfs count="5"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="165" fontId="1" fillId="0" borderId="0" xfId="0" applyNumberFormat="true" applyFont="true"/><xf numFmtId="22" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="true"><alignment/></xf><xf numFmtId="166" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="true" applyAlignment="false"><alignment></alignment></xf><xf numFmtId="167" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="true"/></cellXfs>`,
		`<cellXfs count="2"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="165" fontId="1" fillId="0" borderId="0" xfId="0" applyNumberFormat="true" applyFont="true"/></cellXfs>`, 1)
	require.NotEqual(t, testSerializedStyles, ser, "fixture replacement must apply")

	out, err := mergeStylesXML([]byte(testTemplateStyles), []byte(ser))
	require.NoError(t, err)
	assert.Equal(t, testTemplateStyles, string(out))
}

func TestMergeStylesXML_RootElementIsTemplateRoot(t *testing.T) {
	out, err := mergeStylesXML([]byte(testTemplateStyles), []byte(testSerializedStyles))
	require.NoError(t, err)
	assert.Equal(t, stylesRootTag(t, []byte(testTemplateStyles)), stylesRootTag(t, out))
}

func TestMergeStylesXML_NoDuplicateIgnorableTokens(t *testing.T) {
	out, err := mergeStylesXML([]byte(testTemplateStyles), []byte(testSerializedStyles))
	require.NoError(t, err)
	m := regexp.MustCompile(`mc:Ignorable="([^"]*)"`).FindSubmatch(out)
	require.NotNil(t, m)
	seen := map[string]bool{}
	for _, tok := range strings.Fields(string(m[1])) {
		assert.False(t, seen[tok], "duplicate mc:Ignorable token %q", tok)
		seen[tok] = true
	}
}

func TestMergeStylesXML_AppendsSanitizedExtras(t *testing.T) {
	out, err := mergeStylesXML([]byte(testTemplateStyles), []byte(testSerializedStyles))
	require.NoError(t, err)
	s := string(out)

	// Count attribute == number of xf children == template count + extras.
	m := regexp.MustCompile(`<cellXfs count="(\d+)">(.*?)</cellXfs>`).FindStringSubmatch(s)
	require.NotNil(t, m)
	count, _ := strconv.Atoi(m[1])
	xfs := xfRe.FindAllString(m[2], -1)
	assert.Equal(t, 2+3, count)
	assert.Len(t, xfs, count)

	// Extra 1: built-in numFmtId 22 kept, empty alignment dropped,
	// applyNumberFormat normalized to "1".
	assert.Contains(t, s, `<xf numFmtId="22" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>`)
	// Extra 2: custom numFmtId 166 remapped to the template's 165 (same
	// formatCode), false apply* dropped, empty alignment dropped.
	assert.Contains(t, s, `<xf numFmtId="165" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>`)
	// Extra 3: unknown custom format hh:mm appended with a fresh id and
	// the xf remapped to it.
	assert.Contains(t, s, `<numFmt numFmtId="168" formatCode="hh:mm"/>`)
	assert.Contains(t, s, `<xf numFmtId="168" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>`)
	// numFmts count adjusted (2 template + 1 appended).
	assert.Contains(t, s, `<numFmts count="3">`)
}

func TestMergeStylesXML_KeepsTemplateSectionsVerbatim(t *testing.T) {
	out, err := mergeStylesXML([]byte(testTemplateStyles), []byte(testSerializedStyles))
	require.NoError(t, err)
	s := string(out)
	// Fonts/fills/borders/cellStyles come from the template, not excelize.
	assert.Contains(t, s, `<fonts count="2"><font><sz val="10"/><name val="Arial"/></font><font><b val="1"/><sz val="10"/><name val="Arial"/></font></fonts>`)
	assert.Contains(t, s, `<borders count="1"><border/></borders>`)
	assert.Contains(t, s, `<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>`)
	// Template's first two xfs kept byte-identical.
	assert.Contains(t, s, `<xf numFmtId="165" fontId="1" fillId="0" borderId="0" xfId="0" applyNumberFormat="1" applyFont="1"/>`)
}

func TestMergeStylesXML_ErrorsOnAddedFonts(t *testing.T) {
	ser := strings.Replace(testSerializedStyles, `<fonts count="2">`, `<fonts count="3">`, 1)
	_, err := mergeStylesXML([]byte(testTemplateStyles), []byte(ser))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fonts")
}

func TestMergeStylesXML_ErrorsOnAddedBordersAndFills(t *testing.T) {
	ser := strings.Replace(testSerializedStyles, `<fills count="2">`, `<fills count="4">`, 1)
	_, err := mergeStylesXML([]byte(testTemplateStyles), []byte(ser))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fills")

	ser = strings.Replace(testSerializedStyles, `<borders count="1">`, `<borders count="2">`, 1)
	_, err = mergeStylesXML([]byte(testTemplateStyles), []byte(ser))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "borders")
}

func TestMergeStylesXML_ErrorsOnUnknownNumFmt(t *testing.T) {
	// Extra xf references numFmtId 180 with no matching numFmt element.
	ser := strings.Replace(testSerializedStyles, `<xf numFmtId="167"`, `<xf numFmtId="180"`, 1)
	_, err := mergeStylesXML([]byte(testTemplateStyles), []byte(ser))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "180")
}

func TestMergeStylesXML_NoCellXfsInSerialized(t *testing.T) {
	ser := regexp.MustCompile(`(?s)<cellXfs.*?</cellXfs>`).ReplaceAllString(testSerializedStyles, "")
	out, err := mergeStylesXML([]byte(testTemplateStyles), []byte(ser))
	require.NoError(t, err)
	assert.Equal(t, testTemplateStyles, string(out))
}

// TestSanitizeExtraXF covers the normalization rules in isolation.
func TestSanitizeExtraXF(t *testing.T) {
	in := `<xf numFmtId="49" fontId="2" fillId="0" borderId="1" xfId="0" applyNumberFormat="true" applyFont="false" applyFill="0" applyBorder="1" applyAlignment="true"><alignment/></xf>`
	out := string(sanitizeExtraXF([]byte(in), map[int]int{49: 50}))
	assert.Equal(t, `<xf numFmtId="50" fontId="2" fillId="0" borderId="1" xfId="0" applyNumberFormat="1" applyBorder="1" applyAlignment="1"/>`, out)
}

// --- end-to-end against the real embedded templates ---

// zipPart reads one part from an xlsx archive.
func zipPart(t *testing.T, xlsx []byte, name string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(xlsx), int64(len(xlsx)))
	require.NoError(t, err)
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		require.NoError(t, err)
		b, err := io.ReadAll(rc)
		rc.Close()
		require.NoError(t, err)
		return b
	}
	t.Fatalf("part %s not found", name)
	return nil
}

// buildExport serializes the template through excelize (the Bug 7 trigger)
// with a date cell written, then applies restoreStylesInZip — the same
// pipeline writeExcelResponse runs.
func buildExport(t *testing.T, template, sheet string) (patched, tplStyles []byte) {
	t.Helper()
	tplStyles, err := templateStylesXML(template)
	require.NoError(t, err)

	file, err := excelConfig.Open("config/" + template)
	require.NoError(t, err)
	defer file.Close()
	f, err := excelize.OpenReader(file)
	require.NoError(t, err)
	defer f.Close()

	require.NoError(t, f.SetCellValue(sheet, "A1", "probe"))
	require.NoError(t, f.SetCellValue(sheet, "A2", time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC)))
	var buf bytes.Buffer
	_, err = f.WriteTo(&buf)
	require.NoError(t, err)

	patched, err = restoreStylesInZip(buf.Bytes(), tplStyles)
	require.NoError(t, err)
	return patched, tplStyles
}

func TestRestoreStyles_RootEqualsTemplateRoot(t *testing.T) {
	for _, tc := range []struct{ template, sheet string }{
		{"registre.xlsx", "animals"},
		{"stats_communes.xlsx", "bdd"},
	} {
		t.Run(tc.template, func(t *testing.T) {
			patched, tplStyles := buildExport(t, tc.template, tc.sheet)
			got := zipPart(t, patched, stylesPartPath)
			assert.Equal(t, stylesRootTag(t, tplStyles), stylesRootTag(t, got))
		})
	}
}

func TestRestoreStyles_CellXfsConsistent(t *testing.T) {
	for _, tc := range []struct{ template, sheet string }{
		{"registre.xlsx", "animals"},
		{"stats_communes.xlsx", "bdd"},
	} {
		t.Run(tc.template, func(t *testing.T) {
			patched, _ := buildExport(t, tc.template, tc.sheet)
			got := zipPart(t, patched, stylesPartPath)
			m := cellXfsRe.FindSubmatch(got)
			require.NotNil(t, m)
			count, err := strconv.Atoi(string(m[1]))
			require.NoError(t, err)
			xfs := xfRe.FindAll(m[2], -1)
			assert.Equal(t, count, len(xfs), "cellXfs count attr must equal number of xf children")
			// Every numFmtId referenced by an xf must be built-in or defined.
			defined := map[string]bool{}
			for _, nf := range numFmtRe.FindAllSubmatch(got, -1) {
				defined[string(nf[1])] = true
			}
			for _, xf := range xfs {
				id := numFmtIDAttrRe.FindSubmatch(xf)
				if id == nil {
					continue
				}
				n, _ := strconv.Atoi(string(id[1]))
				if n >= 164 {
					assert.True(t, defined[string(id[1])], "xf references undefined numFmtId %s", id[1])
				}
			}
		})
	}
}

func TestRestoreStyles_WorksheetStyleRefsInRange(t *testing.T) {
	for _, tc := range []struct{ template, sheet string }{
		{"registre.xlsx", "animals"},
		{"stats_communes.xlsx", "bdd"},
	} {
		t.Run(tc.template, func(t *testing.T) {
			patched, _ := buildExport(t, tc.template, tc.sheet)
			got := zipPart(t, patched, stylesPartPath)
			count := countOf(cellXfsRe, got)
			require.Greater(t, count, 0)

			zr, err := zip.NewReader(bytes.NewReader(patched), int64(len(patched)))
			require.NoError(t, err)
			sRef := regexp.MustCompile(`\bs="(\d+)"`)
			for _, f := range zr.File {
				if !strings.HasPrefix(f.Name, "xl/worksheets/") {
					continue
				}
				rc, err := f.Open()
				require.NoError(t, err)
				b, err := io.ReadAll(rc)
				rc.Close()
				require.NoError(t, err)
				for _, m := range sRef.FindAllSubmatch(b, -1) {
					n, _ := strconv.Atoi(string(m[1]))
					assert.Less(t, n, count, "%s uses style s=%d but cellXfs count is %d", f.Name, n, count)
				}
			}
		})
	}
}

func TestRestoreStyles_NoDuplicateIgnorableTokens(t *testing.T) {
	for _, tc := range []struct{ template, sheet string }{
		{"registre.xlsx", "animals"},
		{"stats_communes.xlsx", "bdd"},
	} {
		t.Run(tc.template, func(t *testing.T) {
			patched, _ := buildExport(t, tc.template, tc.sheet)
			got := zipPart(t, patched, stylesPartPath)
			m := regexp.MustCompile(`mc:Ignorable="([^"]*)"`).FindSubmatch(got)
			if m == nil {
				return
			}
			seen := map[string]bool{}
			for _, tok := range strings.Fields(string(m[1])) {
				assert.False(t, seen[tok], "duplicate mc:Ignorable token %q", tok)
				seen[tok] = true
			}
		})
	}
}

func TestRestoreStyles_OtherPartsByteIdentical(t *testing.T) {
	file, err := excelConfig.Open("config/stats_communes.xlsx")
	require.NoError(t, err)
	defer file.Close()
	f, err := excelize.OpenReader(file)
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, f.SetCellValue("bdd", "A1", "probe"))
	var buf bytes.Buffer
	_, err = f.WriteTo(&buf)
	require.NoError(t, err)

	tplStyles, err := templateStylesXML("stats_communes.xlsx")
	require.NoError(t, err)
	patched, err := restoreStylesInZip(buf.Bytes(), tplStyles)
	require.NoError(t, err)

	before, beforeOrder, err := readZipParts(buf.Bytes())
	require.NoError(t, err)
	after, afterOrder, err := readZipParts(patched)
	require.NoError(t, err)
	assert.Equal(t, beforeOrder, afterOrder, "zip entry order must be preserved")
	require.Equal(t, len(before), len(after))
	for name, b := range before {
		if name == stylesPartPath {
			continue
		}
		assert.Equal(t, b, after[name], "part %s must be byte-identical", name)
	}
}

func TestRestoreStylesInZip_InvalidArchive(t *testing.T) {
	_, err := restoreStylesInZip([]byte("not a zip"), []byte(testTemplateStyles))
	assert.Error(t, err)
}

func TestTemplateStylesXML_ReadsEmbeddedTemplates(t *testing.T) {
	for _, tpl := range []string{"registre.xlsx", "stats_communes.xlsx"} {
		b, err := templateStylesXML(tpl)
		require.NoError(t, err, tpl)
		assert.Contains(t, string(b), "<styleSheet")
		assert.Contains(t, string(b), "<cellXfs")
	}
	_, err := templateStylesXML("does_not_exist.xlsx")
	assert.Error(t, err)
}
