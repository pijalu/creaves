package utils

import (
	"strings"
	"testing"
)

func TestExtractInsertStatements_SkipsDDLAndComments(t *testing.T) {
	dump := `-- MySQL dump 10.13
--
-- Table structure for table ` + "`animaltypes`" + `
--

/*!40101 SET NAMES utf8mb4 */;
SET FOREIGN_KEY_CHECKS=0;

DROP TABLE IF EXISTS ` + "`animaltypes`" + `;
CREATE TABLE ` + "`animaltypes`" + ` (
  ` + "`id`" + ` int(11) NOT NULL AUTO_INCREMENT,
  PRIMARY KEY (` + "`id`" + `)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

LOCK TABLES ` + "`animaltypes`" + ` WRITE;
ALTER TABLE ` + "`animaltypes`" + ` DISABLE KEYS;

INSERT INTO ` + "`animaltypes`" + ` VALUES (1,'Bird');

ALTER TABLE ` + "`animaltypes`" + ` ENABLE KEYS;
UNLOCK TABLES;
SET FOREIGN_KEY_CHECKS=1;
`
	order, stmts, err := ExtractInsertStatements(strings.NewReader(dump))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 1 || order[0] != "animaltypes" {
		t.Fatalf("unexpected order: %v", order)
	}
	if got := stmts["animaltypes"]; len(got) != 1 || got[0] != "INSERT INTO `animaltypes` VALUES (1,'Bird');" {
		t.Fatalf("unexpected stmts: %#v", got)
	}
}

func TestExtractInsertStatements_MultiLineInsert(t *testing.T) {
	dump := "INSERT INTO `species` VALUES\n(1,'Fox','Vulpes vulpes'),\n(2,'Badger','Meles meles');\n"
	order, stmts, err := ExtractInsertStatements(strings.NewReader(dump))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 1 || order[0] != "species" {
		t.Fatalf("unexpected order: %v", order)
	}
	want := "INSERT INTO `species` VALUES\n(1,'Fox','Vulpes vulpes'),\n(2,'Badger','Meles meles');"
	if got := stmts["species"]; len(got) != 1 || got[0] != want {
		t.Fatalf("unexpected stmt:\ngot:  %q\nwant: %q", stmts["species"], want)
	}
}

func TestExtractInsertStatements_EscapedQuotesAndNewlines(t *testing.T) {
	dump := "INSERT INTO `notes` VALUES (1,'it\\'s fine','line1\\nline2');\n"
	_, stmts, err := ExtractInsertStatements(strings.NewReader(dump))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "INSERT INTO `notes` VALUES (1,'it\\'s fine','line1\\nline2');"
	if got := stmts["notes"]; len(got) != 1 || got[0] != want {
		t.Fatalf("unexpected stmt: %#v", got)
	}
}

func TestExtractInsertStatements_SemicolonInsideStringMidLine(t *testing.T) {
	// The ';' inside the string must not terminate the statement; the
	// statement ends only when a trimmed line ends with ';'.
	dump := "INSERT INTO `notes` VALUES (1,'a;b'),(2,'c;still string')\n,(3,'tail');\n"
	_, stmts, err := ExtractInsertStatements(strings.NewReader(dump))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "INSERT INTO `notes` VALUES (1,'a;b'),(2,'c;still string')\n,(3,'tail');"
	if got := stmts["notes"]; len(got) != 1 || got[0] != want {
		t.Fatalf("unexpected stmt: %#v", got)
	}
}

func TestExtractInsertStatements_TwoTablesGroupingAndOrder(t *testing.T) {
	dump := `INSERT INTO ` + "`animaltypes`" + ` VALUES (1,'Bird');
INSERT INTO ` + "`animaltypes`" + ` VALUES (2,'Mammal');
INSERT INTO ` + "`species`" + ` VALUES (10,'Fox',1);
INSERT INTO ` + "`animaltypes`" + ` VALUES (3,'Reptile');
INSERT INTO ` + "`species`" + ` VALUES (11,'Badger',2);
`
	order, stmts, err := ExtractInsertStatements(strings.NewReader(dump))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 2 || order[0] != "animaltypes" || order[1] != "species" {
		t.Fatalf("unexpected order: %v", order)
	}
	if got := len(stmts["animaltypes"]); got != 3 {
		t.Fatalf("animaltypes count = %d, want 3", got)
	}
	if got := len(stmts["species"]); got != 2 {
		t.Fatalf("species count = %d, want 2", got)
	}
	if stmts["animaltypes"][2] != "INSERT INTO `animaltypes` VALUES (3,'Reptile');" {
		t.Fatalf("unexpected 3rd animaltypes stmt: %q", stmts["animaltypes"][2])
	}
}

func TestExtractInsertStatements_UnterminatedAtEOF(t *testing.T) {
	dump := "INSERT INTO `species` VALUES (1,'Fox'\n"
	_, _, err := ExtractInsertStatements(strings.NewReader(dump))
	if err == nil {
		t.Fatal("expected error for unterminated statement, got nil")
	}
	if !strings.Contains(err.Error(), "unterminated") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestExtractInsertStatements_EmptyInput(t *testing.T) {
	order, stmts, err := ExtractInsertStatements(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 0 {
		t.Fatalf("order not empty: %v", order)
	}
	if len(stmts) != 0 {
		t.Fatalf("stmts not empty: %v", stmts)
	}
}
