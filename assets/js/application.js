require("expose-loader?exposes=$,jQuery!jquery");
require("./wizard.js");
require("./submitfix.js")
require("./jquery.auto-complete.js");
require("bootstrap/dist/js/bootstrap.bundle.js");
require("@fortawesome/fontawesome-free/js/all.js");
require("flatpickr");
require('jquery-ujs');
require('bootstrap-table/dist/bootstrap-table.js');
require('bootstrap-table/dist/bootstrap-table-locale-all.js');
require('select2/dist/js/select2.full.js');
// D2 (I18N_UI_LOCALIZATION_FIX_PLAN.md Phase 4): select2 widget chrome
// (placeholder/"No results found"/"Searching…") follows the lang cookie.
// The i18n packs self-register on select2's AMD registry when required
// AFTER select2.full.js; the default language must be set at module scope,
// before the inline template scripts run their $('select').select2(...) init.
require('select2/dist/js/i18n/en.js');
require('select2/dist/js/i18n/fr.js');
require('select2/dist/js/i18n/de.js');
require('select2/dist/js/i18n/nl.js');
(function () {
    var m = /(?:^|;\s*)lang=([^;]*)/.exec(document.cookie);
    var lang = m ? decodeURIComponent(m[1]) : '';
    // "" (absent) and "fr" both mean the base/canonical French UI
    // (actions.SwitchLanguage cookie; normalizeUILang domain).
    switch (lang) {
        case 'de': lang = 'de'; break;
        case 'nl': lang = 'nl'; break;
        case 'en-US': lang = 'en'; break;
        default: lang = 'fr';
    }
    if (window.jQuery && jQuery.fn && jQuery.fn.select2) {
        jQuery.fn.select2.defaults.set('language', lang);
    }
})();
//require('bootstrap-table/dist/extensions/filter-control/bootstrap-table-filter-control.js');

$(() => {
    // Close autocomplete suggestion lists on page scroll: they are absolutely
    // positioned and would otherwise float over the sticky navbar once the
    // input scrolls underneath it.
    $(window).on('scroll', () => {
        $('.autocomplete-suggestions').hide();
    });
});
