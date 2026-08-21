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
// ParseInsertRows maps INSERT ... VALUES tuples to column names. For dumps without
// an explicit column list, columns must be supplied in schema order.
func ParseInsertRows(statement string, columns []string) ([]map[string]string, error) {
	upper := strings.ToUpper(statement)
	v := strings.Index(upper, " VALUES")
	if v < 0 {
		return nil, fmt.Errorf("INSERT has no VALUES clause")
	}
	body := strings.TrimSpace(strings.TrimSuffix(statement[v+7:], ";"))
	rows, err := parseValueTuples(body)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]string, 0, len(rows))
	for _, values := range rows {
		if len(values) != len(columns) {
			return nil, fmt.Errorf("INSERT has %d values, want %d columns", len(values), len(columns))
		}
		row := make(map[string]string, len(columns))
		for i, col := range columns {
			row[col] = values[i]
		}
		out = append(out, row)
	}
	return out, nil
}

func parseValueTuples(s string) ([][]string, error) {
	var rows [][]string
	for i := 0; i < len(s); {
		for i < len(s) && (s[i] == ' ' || s[i] == '\n' || s[i] == '\r' || s[i] == ',') {
			i++
		}
		if i >= len(s) {
			break
		}
		if s[i] != '(' {
			return nil, fmt.Errorf("expected tuple at %d", i)
		}
		i++
		var row []string
		for {
			for i < len(s) && (s[i] == ' ' || s[i] == '\n' || s[i] == '\r') {
				i++
			}
			var b strings.Builder
			quoted := false
			parenDepth := 0
			if i < len(s) && s[i] == '\'' {
				quoted = true
				i++
			}
			for i < len(s) {
				c := s[i]
				if quoted {
					if c == '\\' && i+1 < len(s) {
						i++
						switch s[i] {
						case 'n':
							b.WriteByte('\n')
						case 'r':
							b.WriteByte('\r')
						default:
							b.WriteByte(s[i])
						}
						i++
						continue
					}
					if c == '\'' {
						if i+1 < len(s) && s[i+1] == '\'' {
							b.WriteByte('\'')
							i += 2
							continue
						}
						quoted = false
						i++
						break
					}
					b.WriteByte(c)
					i++
					continue
				}
				if c == '(' {
					parenDepth++
				} else if c == ')' {
					if parenDepth == 0 {
						break
					}
					parenDepth--
				}
				if c == ',' && parenDepth == 0 {
					break
				}
				b.WriteByte(c)
				i++
			}
			row = append(row, strings.TrimSpace(b.String()))
			for i < len(s) && (s[i] == ' ' || s[i] == '\n' || s[i] == '\r') {
				i++
			}
			if i >= len(s) {
				return nil, fmt.Errorf("unterminated tuple")
			}
			if s[i] == ')' {
				i++
				break
			}
			if s[i] != ',' {
				return nil, fmt.Errorf("expected comma at %d", i)
			}
			i++
		}
		rows = append(rows, row)
	}
	return rows, nil
}

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
