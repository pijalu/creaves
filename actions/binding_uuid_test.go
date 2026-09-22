package actions

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gobuffalo/buffalo/binding"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

type uuidBindDiscoverer struct {
	ID uuid.UUID
}

type uuidBindDiscovery struct {
	Discoverer uuidBindDiscoverer
}

type uuidBindAnimal struct {
	Discovery uuidBindDiscovery
}

func bindForm(t *testing.T, params map[string]string, dst interface{}) error {
	t.Helper()
	vals := url.Values{}
	for k, v := range params {
		vals.Set(k, v)
	}
	req := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return binding.BaseRequestBinder.Exec(req, dst)
}

// Regression: an empty Discovery.Discoverer.ID (discoverer picker with no
// selection) must bind to uuid.Nil instead of failing the whole request
// with "uuid: incorrect UUID length 0 in string \"\"" (HTTP 500 on animal
// save).
func TestBindEmptyDiscovererID(t *testing.T) {
	registerUUIDDecoder()
	a := &uuidBindAnimal{}
	err := bindForm(t, map[string]string{
		"Discovery.Discoverer.ID": "",
	}, a)
	require.NoError(t, err)
	require.Equal(t, uuid.Nil, a.Discovery.Discoverer.ID)
}

// A filled picker value must still bind to the real UUID.
func TestBindValidDiscovererID(t *testing.T) {
	registerUUIDDecoder()
	id := uuid.Must(uuid.NewV4())
	a := &uuidBindAnimal{}
	err := bindForm(t, map[string]string{
		"Discovery.Discoverer.ID": id.String(),
	}, a)
	require.NoError(t, err)
	require.Equal(t, id, a.Discovery.Discoverer.ID)
}

// A malformed (non-empty) UUID must still be rejected.
func TestBindInvalidDiscovererID(t *testing.T) {
	registerUUIDDecoder()
	a := &uuidBindAnimal{}
	err := bindForm(t, map[string]string{
		"Discovery.Discoverer.ID": "not-a-uuid",
	}, a)
	require.Error(t, err)
}
