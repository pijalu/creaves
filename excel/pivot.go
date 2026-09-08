package excel

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// This file keeps Excel pivot tables functional after excelize rewrites the
// data sheet of an export template. The templates (registre.xlsx,
// stats_communes.xlsx) ship pivot caches whose worksheetSource range
// describes the template's original rows; we repoint it at the written range
// and force a cache refresh on open so Excel rebuilds shared items/records
// from live data. excelize v2.8.0 has no API to update existing pivot
// caches, so we patch the parts directly through the exported f.Pkg
// sync.Map.
//
// IMPORTANT: the cached sharedItems/records must be kept verbatim. Excel
// rejects (repair prompt, "Removed Feature: PivotTable report") any cache
// whose sharedItems/records were rewritten or emptied by a third-party
// tool — even when the rewrite is schema-valid.
//
// A near-identical copy of this logic lives in creaves-console/excel (Bug 6);
// keep both in sync when changing the XML rewrite rules.

// newRefRegexp matches a worksheetSource ref such as A1:AG1368.
var worksheetSourceRefRe = regexp.MustCompile(`(<worksheetSource\b[^>]*\bref=")[^"]*(")`)

// recordCountRe matches the recordCount attribute of pivotCacheDefinition.
var recordCountRe = regexp.MustCompile(`\brecordCount="[^"]*"`)

// refreshOnLoadRe matches the refreshOnLoad attribute of pivotCacheDefinition.
var refreshOnLoadRe = regexp.MustCompile(`\brefreshOnLoad="[^"]*"`)

// filterDatabaseRe matches the _xlnm._FilterDatabase defined name for a given
// sheet (built per sheet in updateFilterDatabase).

// updatePivotCaches rewrites every pivotCacheDefinition part in f: the
// source range is pointed at the data actually written and refreshOnLoad
// forces Excel to rebuild the cache from the sheet on open. The template's
// cached sharedItems/records are kept verbatim: Excel opens the file
// without a repair prompt as long as the cache parts are internally
// consistent (recordCount == number of cached records), and the on-load
// refresh replaces the stale template values with live data.
//
// Rebuilding the cache parts from scratch (typed sharedItems + records)
// was tried and made Excel *more* strict — it flagged the file for repair
// ("Removed Feature: PivotTable report"). Keeping the template cache +
// refreshOnLoad is the combination Excel accepts.
func updatePivotCaches(f *excelize.File, dataSheet string, lastRow int, lastCol string) error {
	newRef := fmt.Sprintf("A1:%s%d", lastCol, lastRow)

	var updateErr error
	f.Pkg.Range(func(k, v interface{}) bool {
		path, ok := k.(string)
		if !ok {
			return true
		}
		if !strings.HasPrefix(path, "xl/pivotCache/pivotCacheDefinition") || !strings.HasSuffix(path, ".xml") {
			return true
		}
		content, ok := v.([]byte)
		if !ok {
			return true
		}

		// Keep the template recordCount (it matches the template's cached
		// records; Excel refreshes on load anyway).
		var updated []byte
		if rc := recordCountRe.Find(content); rc != nil {
			if n, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSuffix(string(rc), `"`), `recordCount="`)); err == nil {
				updated = updatePivotCacheDefinition(content, dataSheet, newRef, n)
			}
		}
		if updated == nil {
			updated = updatePivotCacheDefinition(content, dataSheet, newRef, lastRow-1)
		}
		f.Pkg.Store(path, updated)
		return true
	})

	return updateErr
}

// updatePivotCacheDefinition applies the ref/recordCount/refreshOnLoad edits
// to one pivotCacheDefinition document.
func updatePivotCacheDefinition(content []byte, dataSheet, newRef string, recordCount int) []byte {
	updated := content
	if worksheetSourceRefRe.Match(updated) {
		updated = worksheetSourceRefRe.ReplaceAll(updated, []byte("${1}"+newRef+"${2}"))
	} else if bytes.Contains(updated, []byte("<cacheSource")) {
		// No ref attribute (should not happen for worksheet sources): insert one.
		updated = bytes.Replace(updated, []byte("<worksheetSource"),
			[]byte(`<worksheetSource ref="`+newRef+`"`), 1)
	}

	// Ensure the cache reads from the sheet we actually wrote.
	updated = rewriteAttr(updated, "worksheetSource", "sheet", dataSheet)

	// recordCount must match the number of data rows.
	if recordCountRe.Match(updated) {
		updated = recordCountRe.ReplaceAll(updated,
			[]byte(`recordCount="`+strconv.Itoa(recordCount)+`"`))
	} else {
		updated = addAttrToElement(updated, "pivotCacheDefinition",
			`recordCount="`+strconv.Itoa(recordCount)+`"`)
	}

	// Force Excel to refresh the cache from the source data on open.
	if refreshOnLoadRe.Match(updated) {
		updated = refreshOnLoadRe.ReplaceAll(updated, []byte(`refreshOnLoad="1"`))
	} else {
		updated = addAttrToElement(updated, "pivotCacheDefinition", `refreshOnLoad="1"`)
	}

	return updated
}

// rewriteAttr sets attr to value on every element named elem in doc. If the
// attribute is absent it is appended after the element name.
func rewriteAttr(doc []byte, elem, attr, value string) []byte {
	re := regexp.MustCompile(`(<` + elem + `\b[^>]*?)\s` + attr + `="[^"]*"`)
	if re.Match(doc) {
		return re.ReplaceAll(doc, []byte(`${1} `+attr+`="`+value+`"`))
	}
	return addAttrToElement(doc, elem, attr+`="`+value+`"`)
}

// addAttrToElement inserts an attribute right after the first occurrence of
// an element's opening tag name.
func addAttrToElement(doc []byte, elem, attr string) []byte {
	needle := []byte("<" + elem)
	idx := bytes.Index(doc, needle)
	if idx < 0 {
		return doc
	}
	insertAt := idx + len(needle)
	out := make([]byte, 0, len(doc)+len(attr)+1)
	out = append(out, doc[:insertAt]...)
	out = append(out, ' ')
	out = append(out, attr...)
	out = append(out, doc[insertAt:]...)
	return out
}

// sheetRowRe matches a complete row element (self-closed or not) carrying an
// explicit r attribute; group 1 captures the row number.
var sheetRowRe = regexp.MustCompile(`<row\b[^>]*\br="(\d+)"[^>]*/>|<row\b[^>]*\br="(\d+)"[^>]*>.*?</row>`)

// sheetDimRe matches the dimension element of a worksheet.
var sheetDimRe = regexp.MustCompile(`(<dimension\b[^>]*\bref=")[^"]*(")`)

// truncateSheetXML drops every row below lastRow from a serialized worksheet
// and fixes the dimension element. The templates ship ~1000/3700 example
// rows; if left in place the export appears to contain stale template data
// below the freshly written rows (Bug 5 companion fix). Worksheet parts must
// be patched on the final archive bytes: excelize re-serializes sheets from
// its in-memory model on WriteTo, discarding direct Pkg edits.
func truncateSheetXML(content []byte, lastRow int, lastCol string) []byte {
	updated := sheetRowRe.ReplaceAllFunc(content, func(m []byte) []byte {
		num := sheetRowRe.FindSubmatch(m)
		digits := num[1]
		if len(digits) == 0 {
			digits = num[2]
		}
		n, err := strconv.Atoi(string(digits))
		if err != nil || n <= lastRow {
			return m
		}
		return nil
	})

	// Always pin the dimension to the real written range: excelize keeps the
	// template's original dimension, which is stale in both directions (too
	// small when the export has more rows, too large after truncation) and a
	// dimension smaller than the actual data triggers Excel's repair prompt.
	updated = sheetDimRe.ReplaceAll(updated,
		[]byte("${1}A1:"+lastCol+strconv.Itoa(lastRow)+"${2}"))
	return updated
}

// resolveSheetPathFromXML maps a sheet name to its xl/worksheets/sheetN.xml
// part path given the workbook.xml and workbook.xml.rels documents.
func resolveSheetPathFromXML(wb, rels []byte, sheetName string) (string, error) {
	sheetRe := regexp.MustCompile(`<sheet\b[^>]*name="` + regexp.QuoteMeta(sheetName) +
		`"[^>]*\br:id="([^"]+)"|<sheet\b[^>]*\br:id="([^"]+)"[^>]*name="` + regexp.QuoteMeta(sheetName) + `"`)
	m := sheetRe.FindSubmatch(wb)
	if m == nil {
		return "", fmt.Errorf("sheet %q not found in workbook.xml", sheetName)
	}
	rid := m[1]
	if len(rid) == 0 {
		rid = m[2]
	}

	relRe := regexp.MustCompile(`<Relationship\b[^>]*Id="` + string(rid) + `"[^>]*Target="([^"]+)"`)
	m = relRe.FindSubmatch(rels)
	if m == nil {
		return "", fmt.Errorf("relationship %s for sheet %q not found", rid, sheetName)
	}
	target := string(m[1])
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(target, "/"), nil
	}
	return "xl/" + target, nil
}

// truncateSheetRowsInZip rewrites an XLSX archive after excelize WriteTo:
//  1. truncate the rows below lastRow on the named data sheet,
//  2. rewrite the _xlnm._FilterDatabase defined name of that sheet.
//
// Both parts are re-serialized by excelize from its in-memory model on
// WriteTo, so they must be patched on the final archive bytes — direct Pkg
// edits would be discarded. All other parts are copied verbatim.
func truncateSheetRowsInZip(xlsx []byte, sheetName string, lastRow int, lastCol string) ([]byte, error) {
	parts, order, err := readZipParts(xlsx)
	if err != nil {
		return nil, err
	}

	wb, ok := parts["xl/workbook.xml"]
	if !ok {
		return nil, fmt.Errorf("xl/workbook.xml not found in archive")
	}
	rels, ok := parts["xl/_rels/workbook.xml.rels"]
	if !ok {
		return nil, fmt.Errorf("xl/_rels/workbook.xml.rels not found in archive")
	}
	path, err := resolveSheetPathFromXML(wb, rels, sheetName)
	if err != nil {
		return nil, err
	}
	content, ok := parts[path]
	if !ok {
		return nil, fmt.Errorf("sheet part %s not found in archive", path)
	}
	parts[path] = truncateSheetXML(content, lastRow, lastCol)
	parts["xl/workbook.xml"] = updateFilterDatabaseXML(wb, sheetName, lastRow, lastCol)

	return writeZipParts(parts, order)
}

// readZipParts loads every file of a zip archive into a map, preserving the
// original entry order.
func readZipParts(xlsx []byte) (map[string][]byte, []string, error) {
	zr, err := zip.NewReader(bytes.NewReader(xlsx), int64(len(xlsx)))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid xlsx archive: %w", err)
	}

	parts := make(map[string][]byte, len(zr.File))
	order := make([]string, 0, len(zr.File))
	for _, file := range zr.File {
		rc, err := file.Open()
		if err != nil {
			return nil, nil, err
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, nil, err
		}
		parts[file.Name] = b
		order = append(order, file.Name)
	}
	return parts, order, nil
}

// writeZipParts serializes parts back into a zip archive in the given order.
func writeZipParts(parts map[string][]byte, order []string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range order {
		w, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(parts[name]); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// updateFilterDatabaseXML rewrites the _xlnm._FilterDatabase defined name
// that targets sheetName inside a workbook.xml document so it covers the
// range really written. Other sheets' defined names are left untouched.
// Called on the final archive bytes (workbook.xml is re-serialized by
// excelize on WriteTo, so Pkg edits would be lost).
func updateFilterDatabaseXML(wb []byte, sheetName string, lastRow int, lastCol string) []byte {
	newRef := fmt.Sprintf("%s!$A$1:$%s$%d", sheetName, lastCol, lastRow)
	re := regexp.MustCompile(`(<definedName\b[^>]*name="_xlnm\._FilterDatabase"[^>]*>)[^<]*` +
		regexp.QuoteMeta(sheetName) + `!\$?[A-Z]+\$?\d+:\$?[A-Z]+\$?\d+([^<]*</definedName>)`)
	// Function replacement: the range contains '$' characters which would
	// otherwise be parsed as group references in a replacement template.
	return re.ReplaceAllFunc(wb, func(m []byte) []byte {
		parts := re.FindSubmatch(m)
		out := make([]byte, 0, len(parts[1])+len(newRef)+len(parts[2]))
		out = append(out, parts[1]...)
		out = append(out, newRef...)
		out = append(out, parts[2]...)
		return out
	})
}
