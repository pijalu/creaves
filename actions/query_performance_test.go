package actions

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/pop/v6/logging"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// countSelectsForTables runs fn while counting SELECTs against the specified
// tables. Pop's logger is process-global, so these tests must not run in
// parallel. Filtering by table excludes authentication/config middleware and
// pins the query cost of the page itself.
func countSelectsForTables(t *testing.T, tables []string, fn func()) int {
	t.Helper()
	var mu sync.Mutex
	count := 0
	previousDebug := pop.Debug
	pop.Debug = true
	pop.SetTxLogger(func(level logging.Level, _ interface{}, query string, _ ...interface{}) {
		if level != logging.SQL || !strings.Contains(strings.ToUpper(query), "SELECT") {
			return
		}
		lower := strings.ToLower(query)
		for _, table := range tables {
			if strings.Contains(lower, "from "+table) || strings.Contains(lower, "from `"+table+"`") {
				mu.Lock()
				count++
				mu.Unlock()
				return
			}
		}
	})
	defer func() {
		pop.Debug = previousDebug
		pop.SetTxLogger(func(level logging.Level, _ interface{}, query string, args ...interface{}) {
			if !pop.Debug && level <= logging.Debug {
				return
			}
			if level == logging.SQL {
				t.Logf("[POP] sql - %s", query)
				return
			}
			t.Logf("[POP] "+query, args...)
		})
	}()

	fn()
	mu.Lock()
	defer mu.Unlock()
	return count
}

func TestPaginatedAssociationListsHaveBoundedQueryCounts(t *testing.T) {
	client, baseURL := adminClientWithURL(t)
	tx := searchTestDB(t)
	fixtures := createAnimalSearchFixtures(t, tx)
	marker := uuid.Must(uuid.NewV4()).String()[:8]
	user := &models.User{Login: "ts_query_list_" + marker}
	require.NoError(t, tx.Create(user))
	traveltype := &models.Traveltype{ID: uuid.Must(uuid.NewV4()), Name: "TS-QueryTravel-" + marker}
	require.NoError(t, tx.Create(traveltype))
	for i := 0; i < 5; i++ {
		require.NoError(t, tx.Create(&models.Logentry{
			ID: uuid.Must(uuid.NewV4()), UserID: user.ID, Description: "TS query list " + marker,
		}))
		require.NoError(t, tx.Create(&models.Travel{
			ID: uuid.Must(uuid.NewV4()), Date: time.Now(), AnimalID: fixtures.animalC,
			UserID: user.ID, TraveltypeID: traveltype.ID, Distance: i + 1,
		}))
	}
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM travels WHERE traveltype_id = ?", traveltype.ID).Exec()
		tx.RawQuery("DELETE FROM logentries WHERE user_id = ?", user.ID).Exec()
		tx.RawQuery("DELETE FROM traveltypes WHERE id = ?", traveltype.ID).Exec()
		tx.RawQuery("DELETE FROM users WHERE id = ?", user.ID).Exec()
	})

	tests := []struct {
		path       string
		tables     []string
		maxSelects int
	}{
		// Each request returns five rows. The fixed cost is one COUNT, one
		// page query, and one query per association (plus at most one user
		// lookup by middleware). Default Eager() instead adds one query per
		// association per row.
		{"/treatments?per_page=5", []string{"treatments", "animals"}, 4},
		{"/veterinaryvisits?per_page=5", []string{"veterinaryvisits", "animals", "users"}, 5},
		{"/discoveries?per_page=5", []string{"discoveries", "entry_causes", "discoverers"}, 4},
		{"/outtakes?per_page=5", []string{"outtakes", "outtaketypes", "animals"}, 4},
		{"/logentries?per_page=5", []string{"logentries", "users"}, 4},
		{"/travels?per_page=5", []string{"travels", "animals", "users", "traveltypes"}, 6},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			var status int
			var body string
			selects := countSelectsForTables(t, tt.tables, func() {
				req, err := http.NewRequest(http.MethodGet, baseURL+tt.path, nil)
				require.NoError(t, err)
				req.Header.Set("Accept", "application/json")
				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()
				contents, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				status = resp.StatusCode
				body = string(contents)
			})
			require.Equal(t, http.StatusOK, status, body)
			require.LessOrEqual(t, selects, tt.maxSelects,
				"five-row page must use batched association queries, not per-row eager loads")
			t.Logf("%s: %d counted SELECTs", tt.path, selects)
		})
	}
}

type enrichQueryContext struct {
	buffalo.Context
	tx *pop.Connection
}

func (c enrichQueryContext) Value(key interface{}) interface{} {
	if key == "tx" {
		return c.tx
	}
	return c.Context.Value(key)
}

func TestEnrichAnimalHistoryHasBoundedQueryCount(t *testing.T) {
	tx := searchTestDB(t)
	fixtures := createAnimalSearchFixtures(t, tx)
	animalID := fixtures.animalC
	marker := uuid.Must(uuid.NewV4()).String()[:8]

	caretype := &models.Caretype{ID: uuid.Must(uuid.NewV4()), Name: "TS-QueryCare-" + marker}
	require.NoError(t, tx.Create(caretype))
	user := &models.User{Login: "ts_query_" + marker}
	require.NoError(t, tx.Create(user))

	careIDs := make([]uuid.UUID, 0, 4)
	for i := 0; i < 4; i++ {
		care := &models.Care{
			ID: uuid.Must(uuid.NewV4()), AnimalID: animalID, TypeID: caretype.ID,
			Date: time.Now().Add(time.Duration(-i) * time.Hour),
		}
		require.NoError(t, tx.Create(care))
		careIDs = append(careIDs, care.ID)
	}
	// Exercise the self-referential association as well as Animal and Type.
	require.NoError(t, tx.RawQuery("UPDATE cares SET link_to_id = ? WHERE id = ?", careIDs[0], careIDs[1]).Exec())
	for i := 0; i < 4; i++ {
		treatment := &models.Treatment{
			ID: uuid.Must(uuid.NewV4()), AnimalID: animalID,
			Date: time.Now().Add(time.Duration(-i) * time.Hour), Drug: "TS-drug", Dosage: "1",
		}
		require.NoError(t, tx.Create(treatment))
		visit := &models.Veterinaryvisit{
			ID: uuid.Must(uuid.NewV4()), AnimalID: animalID, UserID: user.ID,
			Date: time.Now().Add(time.Duration(-i) * time.Hour), Veterinary: "TS-vet",
		}
		require.NoError(t, tx.Create(visit))
	}
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM veterinaryvisits WHERE animal_id = ?", animalID).Exec()
		tx.RawQuery("DELETE FROM treatments WHERE animal_id = ?", animalID).Exec()
		tx.RawQuery("UPDATE cares SET link_to_id = NULL WHERE animal_id = ?", animalID).Exec()
		tx.RawQuery("DELETE FROM cares WHERE animal_id = ?", animalID).Exec()
		tx.RawQuery("DELETE FROM caretypes WHERE id = ?", caretype.ID).Exec()
		tx.RawQuery("DELETE FROM users WHERE id = ?", user.ID).Exec()
	})

	animal := &models.Animal{}
	require.NoError(t, tx.Find(animal, animalID))
	ctx := enrichQueryContext{tx: tx}
	tables := []string{"cares", "treatments", "veterinaryvisits", "animals", "caretypes", "users"}
	var enriched *models.Animal
	var err error
	selects := countSelectsForTables(t, tables, func() {
		enriched, err = EnrichAnimal(animal, ctx)
	})
	require.NoError(t, err)
	require.Len(t, enriched.Cares, 4)
	require.Len(t, enriched.Treatments, 4)
	require.Len(t, enriched.VetVisits, 4)
	require.Equal(t, animalID, enriched.Cares[0].Animal.ID)
	require.Equal(t, caretype.ID, enriched.Cares[0].Type.ID)
	require.NotNil(t, enriched.Cares[1].LinkTo)
	require.Equal(t, careIDs[0], enriched.Cares[1].LinkTo.ID)
	require.NotNil(t, enriched.Treatments[0].Animal)
	require.Equal(t, animalID, enriched.Treatments[0].Animal.ID)
	require.Equal(t, animalID, enriched.VetVisits[0].Animal.ID)
	require.Equal(t, user.ID, enriched.VetVisits[0].User.ID)

	// Three history queries plus one batch per association. LinkTo adds one
	// additional cares query. Default Eager() needs dozens for four rows.
	require.LessOrEqual(t, selects, 10,
		"animal histories must have a fixed query budget, got %d SELECTs", selects)
	t.Logf("EnrichAnimal with four cares/treatments/vet visits: %d counted SELECTs", selects)
}

func TestQueryPerformanceIndexesPresent(t *testing.T) {
	tx := searchTestDB(t)
	expected := map[string]struct {
		table   string
		columns string
	}{
		"translations_table_field_locale_record_id_idx": {"translations", "table_name,field,locale,record_id"},
		"cares_type_id_animal_id_date_idx":              {"cares", "type_id,animal_id,date"},
		"treatments_date_timebitmap_animal_id_idx":      {"treatments", "date,timebitmap,animal_id"},
		"animal_audits_animal_id_created_at_id_idx":     {"animal_audits", "animal_id,created_at,id"},
		"veterinaryvisits_date_idx":                     {"veterinaryvisits", "date"},
		"todos_done_at_todo_date_idx":                   {"todos", "done_at,todo_date"},
		"event_streams_created_at_idx":                  {"event_streams", "created_at"},
	}
	for name, want := range expected {
		var got struct {
			Columns *string `db:"columns"`
		}
		err := tx.RawQuery(`SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index) AS columns
			FROM information_schema.statistics
			WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?`, want.table, name).First(&got)
		require.NoError(t, err)
		if got.Columns == nil {
			t.Errorf("missing index %s on %s", name, want.table)
			continue
		}
		require.Equal(t, want.columns, *got.Columns, "index %s on %s", name, want.table)
	}
}
