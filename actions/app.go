package actions

import (
	"fmt"
	"net/http"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo-pop/v3/pop/popmw"
	"github.com/gobuffalo/buffalo/binding"
	"github.com/gobuffalo/envy"
	forcessl "github.com/gobuffalo/mw-forcessl"
	i18n "github.com/gobuffalo/mw-i18n/v2"
	paramlogger "github.com/gobuffalo/mw-paramlogger"
	"github.com/unrolled/secure"

	"creaves/locales"
	"creaves/models"
	"creaves/public"

	csrf "github.com/gobuffalo/mw-csrf"

	"sync"
)

// ENV is used to help switch settings based on where the
// application is being run. Default is "development".
var ENV = envy.Get("GO_ENV", "development")
var app *buffalo.App

// T is translator
var T *i18n.Translator
var appOnce sync.Once

// App is where all routes and middleware for buffalo
// should be defined. This is the nerve center of your
// application.
//
// Routing, middleware, groups, etc... are declared TOP -> DOWN.
// This means if you add a middleware to `app` *after* declaring a
// group, that group will NOT have that new middleware. The same
// is true of resource declarations as well.
//
// It also means that routes are checked in the order they are declared.
// `ServeFiles` is a CATCH-ALL route, so it should always be
// placed last in the route declarations, as it will prevent routes
// declared after it to never be called.
func App() *buffalo.App {
	appOnce.Do(func() {
		app = buffalo.New(buffalo.Options{
			Env:         ENV,
			SessionName: "_creaves_session",
		})

		// Register our datetime format
		binding.RegisterTimeFormats(models.DateTimeFormat, models.DateFormat)
		// Automatically redirect to SSL
		// app.Use(forceSSL())

		// Log request parameters (filters apply).
		app.Use(paramlogger.ParameterLogger)

		// Protect against CSRF attacks. https://www.owasp.org/index.php/Cross-Site_Request_Forgery_(CSRF)
		// Remove to disable this.
		app.Use(csrf.New)

		// Wraps each request in a transaction.
		//  c.Value("tx").(*pop.Connection)
		// Remove to disable this.
		app.Use(popmw.Transaction(models.DB))

		// Setup and use translations:
		app.Use(translations())

		app.GET("/paths", PathHandler)
		app.GET("/", LandingIndex)

		//AuthMiddlewares
		app.Use(SetCurrentUser)
		app.Use(Authorize)

		//Routes for Auth
		auth := app.Group("/auth")
		auth.GET("/", AuthLanding)
		auth.GET("/new", AuthNew)
		auth.POST("/", AuthCreate)
		auth.DELETE("/", AuthDestroy)
		auth.Middleware.Skip(Authorize, AuthLanding, AuthNew, AuthCreate)

		//Routes for languages
		language := app.Group("/lang")
		language.GET("/", SwitchLanguage)
		language.POST("/", SwitchLanguagePost)
		language.Middleware.Remove(Authorize)

		//Routes for User registration
		registrations := app.Group("/registration")
		registrations.GET("/new", UsersNew)
		registrations.POST("/", UsersCreate)
		registrations.Middleware.Remove(Authorize)

		// Routes for users management
		app.Resource("/users", UsersResource{})

		app.Resource("/logentries", LogentriesResource{})
		app.Resource("/discoverers", DiscoverersResource{})
		app.Resource("/discoveries", DiscoveriesResource{})
		app.Resource("/animaltypes", AnimaltypesResource{})
		app.Resource("/intakes", IntakesResource{})
		app.Resource("/outtaketypes", OuttaketypesResource{})
		app.Resource("/outtakes", OuttakesResource{})
		app.Resource("/animals", AnimalsResource{})
		app.GET("/reception/new", ReceptionNew)
		app.Resource("/animalages", AnimalagesResource{})
		app.Resource("/caretypes", CaretypesResource{})
		app.Resource("/cares", CaresResource{})
		app.Resource("/veterinaryvisits", VeterinaryvisitsResource{})
		app.Resource("/treatments", TreatmentsResource{})
		app.PUT("/treatmentschedule", TreatmentUpdateSchedule)

		app.GET("/landing/index", LandingIndex)
		app.GET("/suggestions/animal_species", SuggestionsAnimalSpecies)
		app.GET("/suggestions/discovery_location", SuggestionsDiscoveryLocation)
		app.GET("/suggestions/outtake_location", SuggestionsOuttakeLocation)
		app.GET("/suggestions/discoverer_city", SuggestionsDiscovererCity)
		app.GET("/suggestions/discoverer_country", SuggestionsDiscovererCountry)
		app.GET("/suggestions/animal_in_care", SuggestionsAnimalInCare)
		app.GET("/suggestions/treatment_drug", SuggestionsTreatmentDrug)
		app.GET("/suggestions/treatment_drug_dosage", SuggestionsTreatmentDrugDosage)
		app.GET("/suggestions/CageWithAnimalInCare", SuggestionsCageWithAnimalInCare)
		app.GET("/suggestions/animaltype_species", SuggestionsAnimalTypeDefaultSpecies)
		app.GET("/suggestions/postal_code", SuggestionsPostalCode)
		app.GET("/suggestions/locality", SuggestionsLocality)
		app.GET("/suggestions/discoverer", SuggestionsDiscoverer)

		app.GET("/crash", func(c buffalo.Context) error {
			return fmt.Errorf("Crash me !")
		})
		app.Resource("/traveltypes", TraveltypesResource{})
		app.Resource("/travels", TravelsResource{})

		if ENV != "development" {
			// Custom error handler
			app.ErrorHandlers[500] = func(status int, err error, c buffalo.Context) error {
				c.Flash().Add("danger", err.Error())
				return c.Render(status, r.HTML("/oops/oops.plush.html"))
			}
		}
		app.GET("/dashboard", DashboardIndex)
		app.Resource("/drugs", DrugsResource{})
		app.GET("/registertable", RegistertableIndex)
		app.GET("/registertable/ExportCSV", RegistertableIndexCSV)

		app.GET("/registersnapshot", RegistersnapshotIndex)
		app.GET("/registersnapshot/ExportCSV", RegistersnapshotIndexCSV)

		maintenance := app.Group("/maintenance")
		maintenance.GET("/", MaintenanceIndex)
		maintenance.GET("/renumber", MaintenanceRenumber)

		app.Resource("/species", SpeciesResource{})
		app.GET("/export/csv", ExportCsv)
		app.GET("/export/excel", ExportExcel)

		app.Resource("/localities", LocalitiesResource{})
		app.Resource("/zones", ZonesResource{})
		app.GET("/feeding", FeedingIndex)
		app.GET("/feeding/close", FeedingClose)

		app.Resource("/native_statuses", NativeStatusesResource{})
		app.Resource("/subside_groups", SubsideGroupsResource{})

		// Hints
		app.GET("/hint/speciesDetails", HintSpeciesDetails)

		app.Resource("/entry_causes", EntryCausesResource{})
		app.ServeFiles("/", http.FS(public.FS())) // serve files from the public directory
	})

	return app
}

// translations will load locale files, set up the translator `actions.T`,
// and will return a middleware to use to load the correct locale for each
// request.
// for more information: https://gobuffalo.io/en/docs/localization
func translations() buffalo.MiddlewareFunc {
	var err error
	if T, err = i18n.New(locales.FS(), "en-US"); err != nil {
		app.Stop(err)
	}
	return T.Middleware()
}

// forceSSL will return a middleware that will redirect an incoming request
// if it is not HTTPS. "http://example.com" => "https://example.com".
// This middleware does **not** enable SSL. for your application. To do that
// we recommend using a proxy: https://gobuffalo.io/en/docs/proxy
// for more information: https://github.com/unrolled/secure/
func forceSSL() buffalo.MiddlewareFunc {
	return forcessl.Middleware(secure.Options{
		SSLRedirect:     ENV == "production",
		SSLProxyHeaders: map[string]string{"X-Forwarded-Proto": "https"},
	})
}
