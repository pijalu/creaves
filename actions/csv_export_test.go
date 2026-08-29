package actions

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
)

func parseCSVBody(t *testing.T, body string) (bool, [][]string) {
	t.Helper()
	bom := strings.HasPrefix(body, "\ufeff")
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(body, "\ufeff")))
	r.Comma = ';'
	recs, err := r.ReadAll()
	if err != nil {
		t.Fatalf("body is not valid CSV: %v\nbody:\n%s", err, body)
	}
	return bom, recs
}

func TestCSVWriteHeadersAndRows(t *testing.T) {
	var buf bytes.Buffer

	header := []string{"Numero", "Type", "Reason"}
	rows := [][]string{
		{"1", "Hedgehog", "simple"},
		{"2", "Owl", ""},
	}

	if err := writeCSVTo(&buf, header, rows); err != nil {
		t.Fatalf("writeCSVTo failed: %v", err)
	}

	bom, recs := parseCSVBody(t, buf.String())
	if !bom {
		t.Errorf("expected UTF-8 BOM prefix")
	}
	if len(recs) != 3 {
		t.Fatalf("expected 3 records, got %d", len(recs))
	}
	if strings.Join(recs[0], ";") != "Numero;Type;Reason" {
		t.Errorf("unexpected header: %v", recs[0])
	}
	if recs[2][2] != "" {
		t.Errorf("expected NULL/empty to render as empty string, got %q", recs[2][2])
	}
}

func TestCSVWriteEscaping(t *testing.T) {
	var buf bytes.Buffer

	header := []string{"col"}
	rows := [][]string{
		{`semi;colon`},
		{`with "quotes" inside`},
		{"multi\nline\nvalue"},
		{"accents éàü ù"},
	}

	if err := writeCSVTo(&buf, header, rows); err != nil {
		t.Fatalf("writeCSVTo failed: %v", err)
	}

	_, recs := parseCSVBody(t, buf.String())
	if len(recs) != len(rows)+1 {
		t.Fatalf("expected %d records, got %d", len(rows)+1, len(recs))
	}
	for i, row := range rows {
		if recs[i+1][0] != row[0] {
			t.Errorf("row %d: expected %q, got %q", i, row[0], recs[i+1][0])
		}
	}
}
