package actions

import (
	"testing"

	"creaves/models"
)

// Fixture design (all marker values prefixed RAFT_ to isolate from seed data).
//
// Reference rows:
//   animalages:    RAFT_A1, RAFT_A2
//   animaltypes:   RAFT_AT
//   outtaketypes:  RAFT_REL (rating 1, alive), RAFT_DCD (rating -1, dead),
//                  RAFT_ERR (error type -> excluded everywhere)
//   subside_groups: RAFT_SG
//   native_statuses: RAFT_NS
//   entry_causes:  RAFT_EC1 (N1/C1/D1), RAFT_EC2 (N1/C2/no detail)
//   species:       RAFT_Hedgehog (Mammalia, G1, SG, NS),
//                  RAFT_Sparrow (Aves, G2, no subside, no native),
//                  RAFT_Newt (Aves, G1, SG, NS)
//
// Animals (intake/discovery/discoverer rows created per animal):
//   year 1998:
//     1 Hedgehog  A1 EC1 outtake REL
//     2 Hedgehog  A1 EC1 outtake DCD
//     3 Sparrow   A2 EC2 outtake REL
//     4 Sparrow   A2 EC2 no outtake
//     5 Newt      A1 EC1 outtake ERR (excluded from all tables)
//     6 RAFT_NOSPEC (species not in species table) A1 EC1 no outtake
//     8 Newt      A2 EC2 no outtake
//   year 1997:
//     7 Hedgehog  A1 EC1 no outtake (other year -> never counted)
//
// In-scope 1998 animals: 1,2,3,4,6,8 => T=6 for non-outtake tables, T=3 for
// outtake tables (REL x2, DCD x1).

const raftYear = "1998"

func raftExec(t *testing.T, q string, args ...interface{}) {
	t.Helper()
	if err := models.DB.RawQuery(q, args...).Exec(); err != nil {
		t.Fatalf("fixture insert failed: %v\nquery: %s", err, q)
	}
}

func setupReportsAnnualFixtures(t *testing.T) {
	t.Helper()
	now := "NOW()"

	raftExec(t, "INSERT INTO animalages (id, name, `def`, created_at, updated_at) VALUES ('11111111-1111-1111-1111-1111111111a1', 'RAFT Young', 0, "+now+", "+now+")")
	raftExec(t, "INSERT INTO animalages (id, name, `def`, created_at, updated_at) VALUES ('11111111-1111-1111-1111-1111111111a2', 'RAFT Adult', 0, "+now+", "+now+")")
	raftExec(t, "INSERT INTO animaltypes (id, name, `def`, created_at, updated_at) VALUES ('22222222-2222-2222-2222-2222222222a1', 'RAFT Type', 0, "+now+", "+now+")")
	raftExec(t, "INSERT INTO outtaketypes (id, name, `def`, created_at, updated_at, dead, rating, error) VALUES ('33333333-3333-3333-3333-3333333333a1', 'RAFT_REL', 0, "+now+", "+now+", 0, 1, 0)")
	raftExec(t, "INSERT INTO outtaketypes (id, name, `def`, created_at, updated_at, dead, rating, error) VALUES ('33333333-3333-3333-3333-3333333333a2', 'RAFT_DCD', 0, "+now+", "+now+", 1, -1, 0)")
	raftExec(t, "INSERT INTO outtaketypes (id, name, `def`, created_at, updated_at, dead, rating, error) VALUES ('33333333-3333-3333-3333-3333333333a3', 'RAFT_ERR', 0, "+now+", "+now+", 0, 0, 1)")
	raftExec(t, "INSERT INTO subside_groups (id, `group`, size, amount, created_at, updated_at) VALUES ('RAFT_SG', 'RAFT Group A', 1, 50, "+now+", "+now+")")
	raftExec(t, "INSERT INTO native_statuses (id, status, indication, created_at, updated_at, freeable) VALUES ('RAFT_NS', 'RAFT Native', 'x', "+now+", "+now+", 1)")
	raftExec(t, "INSERT INTO entry_causes (id, cause, detail, nature, indication, created_at, updated_at, sort_order) VALUES ('RAFT_EC1', 'RAFT_C1', 'RAFT_D1', 'RAFT_N1', 'x', "+now+", "+now+", 1)")
	raftExec(t, "INSERT INTO entry_causes (id, cause, detail, nature, indication, created_at, updated_at, sort_order) VALUES ('RAFT_EC2', 'RAFT_C2', '', 'RAFT_N1', 'x', "+now+", "+now+", 2)")

	species := []struct{ id, class, subside, agw, native string }{
		{"RAFT_Hedgehog", "RAFT_Mammalia", "RAFT_SG", "RAFT_G1", "RAFT_NS"},
		{"RAFT_Sparrow", "RAFT_Aves", "", "RAFT_G2", ""},
		{"RAFT_Newt", "RAFT_Aves", "RAFT_SG", "RAFT_G1", "RAFT_NS"},
	}
	for _, s := range species {
		raftExec(t, "INSERT INTO species (ID, species, class, family, creaves_species, subside_group, created_at, updated_at, `order`, agw_group, native_status) VALUES (?, 's', ?, 'f', ?, ?, "+now+", "+now+", 'o', ?, ?)",
			s.id, s.class, s.id, s.subside, s.agw, s.native)
	}

	raftExec(t, "INSERT INTO discoverers (id, created_at, updated_at) VALUES ('44444444-4444-4444-4444-4444444444a1', "+now+", "+now+")")

	// animal id, year, species, age, entry cause, outtake type ("" = none)
	animals := []struct {
		id, year         int
		species, age, ec string
		outtake          string
	}{
		{980001, 1998, "RAFT_Hedgehog", "11111111-1111-1111-1111-1111111111a1", "RAFT_EC1", "33333333-3333-3333-3333-3333333333a1"},
		{980002, 1998, "RAFT_Hedgehog", "11111111-1111-1111-1111-1111111111a1", "RAFT_EC1", "33333333-3333-3333-3333-3333333333a2"},
		{980003, 1998, "RAFT_Sparrow", "11111111-1111-1111-1111-1111111111a2", "RAFT_EC2", "33333333-3333-3333-3333-3333333333a1"},
		{980004, 1998, "RAFT_Sparrow", "11111111-1111-1111-1111-1111111111a2", "RAFT_EC2", ""},
		{980005, 1998, "RAFT_Newt", "11111111-1111-1111-1111-1111111111a1", "RAFT_EC1", "33333333-3333-3333-3333-3333333333a3"},
		{980006, 1998, "RAFT_NOSPEC", "11111111-1111-1111-1111-1111111111a1", "RAFT_EC1", ""},
		{980007, 1997, "RAFT_Hedgehog", "11111111-1111-1111-1111-1111111111a1", "RAFT_EC1", ""},
		{980008, 1998, "RAFT_Newt", "11111111-1111-1111-1111-1111111111a2", "RAFT_EC2", ""},
	}

	for i, a := range animals {
		intakeID := uuidLike(i, 5)
		discoveryID := uuidLike(i, 6)
		outtakeID := uuidLike(i, 7)
		raftExec(t, "INSERT INTO intakes (id, date, created_at, updated_at) VALUES (?, ?, "+now+", "+now+")", intakeID, dateForYear(a.year))
		raftExec(t, "INSERT INTO discoveries (id, date, discoverer_id, created_at, updated_at, entry_cause_id) VALUES (?, ?, '44444444-4444-4444-4444-4444444444a1', "+now+", "+now+", ?)", discoveryID, dateForYear(a.year), a.ec)
		if a.outtake != "" {
			raftExec(t, "INSERT INTO outtakes (id, date, outtaketype_id, created_at, updated_at) VALUES (?, ?, ?, "+now+", "+now+")", outtakeID, dateForYear(a.year), a.outtake)
		}
		var outtakeArg interface{}
		if a.outtake != "" {
			outtakeArg = outtakeID
		}
		raftExec(t, "INSERT INTO animals (id, species, animalage_id, animaltype_id, discovery_id, intake_id, outtake_id, created_at, updated_at, year, yearNumber, IntakeDate) VALUES (?, ?, ?, '22222222-2222-2222-2222-2222222222a1', ?, ?, ?, "+now+", "+now+", ?, ?, ?)",
			a.id, a.species, a.age, discoveryID, intakeID, outtakeArg, a.year, a.id%1000, dateForYear(a.year))
	}

	t.Cleanup(func() { teardownReportsAnnualFixtures(t) })
}

func uuidLike(i, prefix int) string {
	// deterministic uuid-shaped ids: <prefix>0000000-0000-0000-0000-00000000000<i>
	return sprintfUUID(prefix, i)
}

func sprintfUUID(prefix, i int) string {
	const digits = "0123456789abcdef"
	p := digits[prefix]
	c := digits[i]
	return string([]byte{p, p, p, p, p, p, p, p}) + "-0000-0000-0000-0000000000" + string([]byte{c, c})
}

func dateForYear(y int) string {
	if y == 1997 {
		return "1997-06-01 10:00:00"
	}
	return "1998-06-01 10:00:00"
}

func teardownReportsAnnualFixtures(t *testing.T) {
	t.Helper()
	exec := func(q string, args ...interface{}) {
		if err := models.DB.RawQuery(q, args...).Exec(); err != nil {
			t.Logf("teardown failed: %v (%s)", err, q)
		}
	}
	exec("DELETE FROM animals WHERE id BETWEEN 980001 AND 980008")
	exec("DELETE FROM outtakes WHERE id LIKE '77777777%'")
	exec("DELETE FROM discoveries WHERE id LIKE '66666666%'")
	exec("DELETE FROM intakes WHERE id LIKE '55555555%'")
	exec("DELETE FROM discoverers WHERE id = '44444444-4444-4444-4444-4444444444a1'")
	exec("DELETE FROM species WHERE ID LIKE 'RAFT_%'")
	exec("DELETE FROM entry_causes WHERE id LIKE 'RAFT_%'")
	exec("DELETE FROM native_statuses WHERE id LIKE 'RAFT_%'")
	exec("DELETE FROM subside_groups WHERE id LIKE 'RAFT_%'")
	exec("DELETE FROM outtaketypes WHERE id LIKE '33333333%'")
	exec("DELETE FROM animaltypes WHERE id LIKE '22222222%'")
	exec("DELETE FROM animalages WHERE id LIKE '11111111%'")
}

func sectionByID(t *testing.T, sections []annualStatSection, id string) annualStatSection {
	t.Helper()
	for _, s := range sections {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("section %s not found", id)
	return annualStatSection{}
}

func assertRows(t *testing.T, sec annualStatSection, wantTotal int, want [][3]string) {
	t.Helper()
	if sec.Total != wantTotal {
		t.Errorf("section %s: total = %d, want %d", sec.ID, sec.Total, wantTotal)
	}
	if len(sec.Rows) != len(want) {
		t.Fatalf("section %s: %d rows, want %d (rows=%+v)", sec.ID, len(sec.Rows), len(want), sec.Rows)
	}
	for i, w := range want {
		got := sec.Rows[i]
		if got.Category != w[0] {
			t.Errorf("section %s row %d: category = %q, want %q", sec.ID, i, got.Category, w[0])
		}
		if gotCount := itoa(got.Count); gotCount != w[1] {
			t.Errorf("section %s row %d: count = %s, want %s", sec.ID, i, gotCount, w[1])
		}
		if got.Percent != w[2] {
			t.Errorf("section %s row %d: percent = %q, want %q", sec.ID, i, got.Percent, w[2])
		}
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	buf := [8]byte{}
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func TestReportsAnnualStats(t *testing.T) {
	setupReportsAnnualFixtures(t)

	sections, err := runAnnualStats(models.DB, raftYear)
	if err != nil {
		t.Fatalf("runAnnualStats failed: %v", err)
	}
	if len(sections) != 12 {
		t.Fatalf("expected 12 sections, got %d", len(sections))
	}

	assertRows(t, sectionByID(t, sections, "species"), 6, [][3]string{
		{"RAFT_Hedgehog", "2", "33.3"},
		{"RAFT_Sparrow", "2", "33.3"},
		{"RAFT_Newt", "1", "16.7"},
		{"Unknown", "1", "16.7"},
	})
	assertRows(t, sectionByID(t, sections, "class"), 6, [][3]string{
		{"RAFT_Aves", "3", "50.0"},
		{"RAFT_Mammalia", "2", "33.3"},
		{"Unknown", "1", "16.7"},
	})
	assertRows(t, sectionByID(t, sections, "agw_group"), 6, [][3]string{
		{"RAFT_G1", "3", "50.0"},
		{"RAFT_G2", "2", "33.3"},
		{"Unknown", "1", "16.7"},
	})
	assertRows(t, sectionByID(t, sections, "subsidies_group"), 6, [][3]string{
		{"RAFT Group A", "3", "50.0"},
		{"Unknown", "3", "50.0"},
	})
	assertRows(t, sectionByID(t, sections, "native_status"), 6, [][3]string{
		{"RAFT Native", "3", "50.0"},
		{"Unknown", "3", "50.0"},
	})
	assertRows(t, sectionByID(t, sections, "entry_age"), 6, [][3]string{
		{"RAFT Adult", "3", "50.0"},
		{"RAFT Young", "3", "50.0"},
	})
	assertRows(t, sectionByID(t, sections, "outtake_type"), 3, [][3]string{
		{"RAFT_REL", "2", "66.7"},
		{"RAFT_DCD", "1", "33.3"},
	})
	assertRows(t, sectionByID(t, sections, "outtake_rating"), 3, [][3]string{
		{"Dead", "1", "33.3"},
		{"Alive", "2", "66.7"},
	})
	assertRows(t, sectionByID(t, sections, "outtake_dead_released"), 3, [][3]string{
		{"Dead", "1", "33.3"},
		{"Released", "2", "66.7"},
	})
	assertRows(t, sectionByID(t, sections, "entry_cause"), 6, [][3]string{
		{"RAFT_C1", "3", "50.0"},
		{"RAFT_C2", "3", "50.0"},
	})
	assertRows(t, sectionByID(t, sections, "entry_cause_detail"), 6, [][3]string{
		{"RAFT_N1 / RAFT_C1 / RAFT_D1", "3", "50.0"},
		{"RAFT_N1 / RAFT_C2", "3", "50.0"},
	})
	assertRows(t, sectionByID(t, sections, "entry_cause_nature"), 6, [][3]string{
		{"RAFT_N1", "6", "100.0"},
	})
}

func TestAnnualStatPercent(t *testing.T) {
	cases := []struct {
		count, total int
		want         string
	}{
		{1, 3, "33.3"},
		{2, 3, "66.7"},
		{1, 6, "16.7"},
		{3, 6, "50.0"},
		{6, 6, "100.0"},
		{0, 0, "0.0"},
		{0, 5, "0.0"},
	}
	for _, c := range cases {
		if got := annualStatPercent(c.count, c.total); got != c.want {
			t.Errorf("annualStatPercent(%d, %d) = %q, want %q", c.count, c.total, got, c.want)
		}
	}
}
