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
// stats_communes.xlsx) ship pivot caches whose worksheetSource range,
// recordCount, cached records and shared items all describe the template's
// original rows. excelize v2.8.0 has no API to update existing pivot caches,
// so we patch the parts directly through the exported f.Pkg sync.Map.
//
// A near-identical copy of this logic lives in creaves-console/excel (Bug 6);
// keep both in sync when changing the XML rewrite rules.

// newRefRegexp matches a worksheetSource ref such as A1:AG1368.
var worksheetSourceRefRe = regexp.MustCompile(`(<worksheetSource\b[^>]*\bref=")[^"]*(")`)

// sharedItemsRe matches a complete sharedItems element, self-closed or not.
var sharedItemsRe = regexp.MustCompile(`<sharedItems\b[^>]*/>|<sharedItems\b[^>]*>.*?</sharedItems>`)

// recordCountRe matches the recordCount attribute of pivotCacheDefinition.
var recordCountRe = regexp.MustCompile(`\brecordCount="[^"]*"`)

// refreshOnLoadRe matches the refreshOnLoad attribute of pivotCacheDefinition.
var refreshOnLoadRe = regexp.MustCompile(`\brefreshOnLoad="[^"]*"`)

// filterDatabaseRe matches the _xlnm._FilterDatabase defined name for a given
// sheet (built per sheet in updateFilterDatabase).

// updatePivotCaches rewrites every pivotCacheDefinition part in f so that it
// points at the data range actually written (A1:lastCol+lastRow on dataSheet),
// forces a refresh on open, drops the cached records and empties the cached
// shared items. Excel then rebuilds the cache from live data instead of
// reporting unreadable content.
func updatePivotCaches(f *excelize.File, dataSheet string, lastRow int, lastCol string) error {
	newRef := fmt.Sprintf("A1:%s%d", lastCol, lastRow)
	recordCount := lastRow - 1 // exclude header row
	if recordCount < 0 {
		recordCount = 0
	}

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

		updated := updatePivotCacheDefinition(content, dataSheet, newRef, recordCount)

		// Empty the cached shared items: values came from the template rows
		// and would otherwise be presented by Excel as stale filter entries.
		// With refreshOnLoad="1" and empty records Excel rebuilds them from
		// the source range, so a plain empty element is sufficient and avoids
		// guessing per-field type attributes.
		updated = sharedItemsRe.ReplaceAll(updated, []byte(`<sharedItems/>`))

		f.Pkg.Store(path, updated)

		// Replace the matching pivotCacheRecords part with an empty record set
		// so Excel rebuilds it from the source range on open.
		recordsPath := strings.Replace(path, "pivotCacheDefinition", "pivotCacheRecords", 1)
		if _, exists := f.Pkg.Load(recordsPath); exists {
			empty := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
				`<pivotCacheRecords xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" count="0"/>`)
			f.Pkg.Store(recordsPath, empty)
		}
		return true
	})

	return updateErr
}

// updatePivotCacheDefinition applies the ref/recordCount/refreshOnLoad edits
// to one pivotCacheDefinition document.
func updatePivotCacheDefinition(content []byte, dataSheet, newRef string, recordCount int) []byte {
	updated := content

	// Point the cache at the range really written on the data sheet.
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
