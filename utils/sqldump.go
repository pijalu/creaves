package utils

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// insertStartRe matches the beginning of an INSERT statement for a given
// table, with or without backticks and with or without a column list, e.g.
// "INSERT INTO `animaltypes` VALUES (..." or
// "INSERT INTO translations (id, ...) VALUES (...".
var insertStartRe = regexp.MustCompile("^INSERT INTO `?([a-zA-Z0-9_]+)`?(\\s*\\([^)]*\\))? VALUES")

// maxDumpLineSize allows very long INSERT lines (16MB) found in mysqldump output.
const maxDumpLineSize = 16 * 1024 * 1024

// ExtractInsertStatements reads a SQL dump and groups complete INSERT INTO ...
// VALUES statements per table. It returns the tables in first-seen order plus a
// map of table name -> list of statements. DDL, comments, SET/DROP/CREATE/
// LOCK/ALTER and any other non-INSERT content is skipped. A statement spans
// multiple lines until a trimmed line ends with ';'. An unterminated statement
// at EOF is an error.
func ExtractInsertStatements(r io.Reader) (order []string, stmts map[string][]string, err error) {
	stmts = map[string][]string{}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), maxDumpLineSize)

	var (
		inStmt  bool
		table   string
		current strings.Builder
	)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if !inStmt {
			m := insertStartRe.FindStringSubmatch(trimmed)
			if m == nil {
				continue // DDL, comments, SET, DROP, CREATE, LOCK, ALTER, ...
			}
			inStmt = true
			table = m[1]
			current.Reset()
			current.WriteString(trimmed)
		} else {
			current.WriteString("\n")
			current.WriteString(trimmed)
		}

		if inStmt && strings.HasSuffix(trimmed, ";") {
			if _, seen := stmts[table]; !seen {
				order = append(order, table)
			}
			stmts[table] = append(stmts[table], current.String())
			inStmt = false
			table = ""
		}
	}

	if serr := scanner.Err(); serr != nil {
		return nil, nil, fmt.Errorf("reading SQL dump: %w", serr)
	}

	if inStmt {
		return nil, nil, fmt.Errorf("unterminated INSERT statement for table %q at EOF", table)
	}

	return order, stmts, nil
}
