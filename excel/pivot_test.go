package excel

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

const testPivotCacheDefinition = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<pivotCacheDefinition xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="rId1" recordCount="1367" createdVersion="3"><cacheSource type="worksheet"><worksheetSource ref="A1:AG1368" sheet="animals"/></cacheSource><cacheFields count="2"><cacheField name="ID" numFmtId="0"><sharedItems count="3"><s v="1"/><s v="2"/><s v="3"/></sharedItems></cacheField><cacheField name="Espèce" numFmtId="0"><sharedItems count="2"><s v="Merle"/><s v="Pigeon"/></sharedItems></cacheField></cacheFields></pivotCacheDefinition>`

const testPivotCacheRecords = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<pivotCacheRecords xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" count="1367"><r><x v="0"/><x v="1"/></r><r><x v="1"/><x v="0"/></r></pivotCacheRecords>`

const testWorkbook = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><definedNames><definedName name="_xlnm._FilterDatabase" localSheetId="5" hidden="1">bdd!$A$1:$N$3696</definedName><definedName name="_xlnm._FilterDatabase" localSheetId="1" hidden="1">Commune!$A$1:$F$560</definedName><definedName name="Nombre">STATS!#REF!</definedName></definedNames></workbook>`

// newTestFile builds an excelize.File whose Pkg contains the given parts.
func newTestFile(t *testing.T, parts map[string][]byte) *excelize.File {
	t.Helper()
	f := excelize.NewFile()
	for path, content := range parts {
		f.Pkg.Store(path, content)
	}
	return f
}

func pkgString(t *testing.T, f *excelize.File, path string) string {
	t.Helper()
	v, ok := f.Pkg.Load(path)
	require.True(t, ok, "package part %s missing", path)
	b, ok := v.([]byte)
	require.True(t, ok, "package part %s not []byte", path)
	return string(b)
}

func TestUpdatePivotCaches_UpdatesDefinition(t *testing.T) {
	f := newTestFile(t, map[string][]byte{
		"xl/pivotCache/pivotCacheDefinition1.xml": []byte(testPivotCacheDefinition),
		"xl/pivotCache/pivotCacheRecords1.xml":    []byte(testPivotCacheRecords),
	})

	require.NoError(t, updatePivotCaches(f, "animals", 101, "AG"))

	def := pkgString(t, f, "xl/pivotCache/pivotCacheDefinition1.xml")
	assert.Contains(t, def, `ref="A1:AG101"`)
	assert.Contains(t, def, `sheet="animals"`)
	assert.Contains(t, def, `recordCount="100"`)
	assert.Contains(t, def, `refreshOnLoad="1"`)
	// Stale shared items are dropped so Excel rebuilds them from live data.
	assert.NotContains(t, def, `<s v="Merle"/>`)
	assert.NotContains(t, def, `<s v="1"/>`)
	assert.Contains(t, def, `<sharedItems/>`)
	// Field structure preserved.
	assert.Contains(t, def, `<cacheFields count="2">`)
	assert.Contains(t, def, `cacheField name="Espèce"`)
}

func TestUpdatePivotCaches_EmptiesRecords(t *testing.T) {
	f := newTestFile(t, map[string][]byte{
		"xl/pivotCache/pivotCacheDefinition1.xml": []byte(testPivotCacheDefinition),
		"xl/pivotCache/pivotCacheRecords1.xml":    []byte(testPivotCacheRecords),
	})

	require.NoError(t, updatePivotCaches(f, "animals", 50, "AG"))

	recs := pkgString(t, f, "xl/pivotCache/pivotCacheRecords1.xml")
	assert.Contains(t, recs, `<pivotCacheRecords`)
	assert.Contains(t, recs, `count="0"`)
	assert.NotContains(t, recs, "<r>")
	assert.NotContains(t, recs, `count="1367"`)
}

func TestUpdatePivotCaches_SetsRefreshOnLoadWhenMissing(t *testing.T) {
	// registre.xlsx template has no refreshOnLoad attribute.
	def := strings.Replace(testPivotCacheDefinition, ` recordCount="1367"`, "", 1)
	f := newTestFile(t, map[string][]byte{
		"xl/pivotCache/pivotCacheDefinition1.xml": []byte(def),
	})

	require.NoError(t, updatePivotCaches(f, "animals", 10, "AG"))

	out := pkgString(t, f, "xl/pivotCache/pivotCacheDefinition1.xml")
	assert.Contains(t, out, `refreshOnLoad="1"`)
	assert.Contains(t, out, `recordCount="9"`)
}

func TestUpdatePivotCaches_HeaderOnly(t *testing.T) {
	f := newTestFile(t, map[string][]byte{
		"xl/pivotCache/pivotCacheDefinition1.xml": []byte(testPivotCacheDefinition),
	})

	require.NoError(t, updatePivotCaches(f, "animals", 1, "AG"))

	out := pkgString(t, f, "xl/pivotCache/pivotCacheDefinition1.xml")
	assert.Contains(t, out, `recordCount="0"`)
	assert.Contains(t, out, `ref="A1:AG1"`)
}

func TestUpdatePivotCaches_LeavesOtherPartsUntouched(t *testing.T) {
	sheetXML := []byte(`<worksheet><sheetData/></worksheet>`)
	f := newTestFile(t, map[string][]byte{
		"xl/pivotCache/pivotCacheDefinition1.xml": []byte(testPivotCacheDefinition),
		"xl/worksheets/sheet1.xml":                sheetXML,
	})

	require.NoError(t, updatePivotCaches(f, "animals", 10, "AG"))

	assert.Equal(t, string(sheetXML), pkgString(t, f, "xl/worksheets/sheet1.xml"))
}

func TestUpdateFilterDatabaseXML_UpdatesMatchingSheet(t *testing.T) {
	wb := updateFilterDatabaseXML([]byte(testWorkbook), "bdd", 200, "N")
	out := string(wb)
	assert.Contains(t, out, `>bdd!$A$1:$N$200</definedName>`)
	// Other sheets' defined names untouched.
	assert.Contains(t, out, `>Commune!$A$1:$F$560</definedName>`)
	assert.Contains(t, out, `>STATS!#REF!</definedName>`)
	assert.NotContains(t, out, `bdd!$A$1:$N$3696`)
}

func TestUpdateFilterDatabaseXML_NoMatchIsNoop(t *testing.T) {
	wb := updateFilterDatabaseXML([]byte(testWorkbook), "animals", 200, "AG")
	assert.Equal(t, testWorkbook, string(wb))
}

const testSheetXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><dimension ref="A1:C5"/><sheetData><row r="1"><c r="A1"><v>h1</v></c></row><row r="2"><c r="A2"><v>a</v></c></row><row r="3"><c r="A3"><v>stale</v></c></row><row r="4"><c r="A4"><v>stale</v></c></row><row r="5"/></sheetData></worksheet>`

func TestTruncateSheetXML_RemovesRowsBelowLastRow(t *testing.T) {
	out := string(truncateSheetXML([]byte(testSheetXML), 2, "C"))
	assert.Contains(t, out, `<row r="1">`)
	assert.Contains(t, out, `<row r="2">`)
	assert.NotContains(t, out, `r="3"`)
	assert.NotContains(t, out, `stale`)
	assert.NotContains(t, out, `<row r="5"/>`)
	assert.Contains(t, out, `<dimension ref="A1:C2"/>`)
}

func TestTruncateSheetXML_NothingToRemove(t *testing.T) {
	// Rows untouched, but the dimension is always pinned to the written range.
	out := string(truncateSheetXML([]byte(testSheetXML), 9, "C"))
	assert.Contains(t, out, `<dimension ref="A1:C9"/>`)
	assert.Contains(t, out, `<row r="5"/>`)
	assert.Contains(t, out, "stale")
}

func TestResolveSheetPathFromXML(t *testing.T) {
	wb := `<?xml version="1.0"?><workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="bdd" sheetId="4" r:id="rId6"/><sheet name="STATS" sheetId="1" r:id="rId1"/></sheets></workbook>`
	rels := `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId6" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet6.xml"/><Relationship Id="rId1" Type="http://x/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`

	path, err := resolveSheetPathFromXML([]byte(wb), []byte(rels), "bdd")
	require.NoError(t, err)
	assert.Equal(t, "xl/worksheets/sheet6.xml", path)

	_, err = resolveSheetPathFromXML([]byte(wb), []byte(rels), "nope")
	assert.Error(t, err)
}

func TestTruncateSheetRowsInZip_EndToEnd(t *testing.T) {
	// Build a minimal xlsx archive in memory.
	wb := `<?xml version="1.0"?><workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="animals" sheetId="1" r:id="rId1"/></sheets><definedNames><definedName name="_xlnm._FilterDatabase" localSheetId="0" hidden="1">animals!$A$1:$C$999</definedName></definedNames></workbook>`
	rels := `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://x/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range map[string]string{
		"xl/workbook.xml":             wb,
		"xl/_rels/workbook.xml.rels":  rels,
		"xl/worksheets/sheet1.xml":    testSheetXML,
		"xl/pivotCache/otherpart.xml": "untouched",
	} {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())

	out, err := truncateSheetRowsInZip(buf.Bytes(), "animals", 2, "C")
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	require.NoError(t, err)
	got := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		b, err := io.ReadAll(rc)
		rc.Close()
		require.NoError(t, err)
		got[f.Name] = string(b)
	}

	assert.NotContains(t, got["xl/worksheets/sheet1.xml"], "stale")
	assert.Contains(t, got["xl/workbook.xml"], `>animals!$A$1:$C$2</definedName>`)
	assert.Equal(t, "untouched", got["xl/pivotCache/otherpart.xml"])
}

func TestTruncateSheetRowsInZip_InvalidArchive(t *testing.T) {
	_, err := truncateSheetRowsInZip([]byte("not a zip"), "animals", 2, "C")
	assert.Error(t, err)
}
