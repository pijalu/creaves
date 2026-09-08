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
// bugs.md datatable item: DataTables for the dynamic export view
// (sort/filter/paginate on a data cache instead of a 10k-row DOM).
// Must load AFTER jQuery is exposed (line 1) so it attaches to window.jQuery.
// Webpack/CJS: require('datatables.net-bs4') returns a constructor detached
// from the jQuery-wired one — the live namespace (defaults/ext/type) is
// jQuery.fn.dataTable. Expose THAT as window.DataTable for template scripts.
require('datatables.net-bs4');
window.DataTable = jQuery.fn.dataTable;
// Official i18n packs (CJS, each returns the language object); keyed by the
// normalized lang cookie values used below and in the export view template.
window.CREAVES_DT_LANG = {
    'fr': require('datatables.net-plugins/i18n/fr-FR.js'),
    'de': require('datatables.net-plugins/i18n/de-DE.js'),
    'nl': require('datatables.net-plugins/i18n/nl-NL.js'),
    'en': {} // DataTables' built-in UI strings are English
};
// Custom type: French decimal-comma numbers ("1,5") sort numerically and
// right-align. Ported from the removed sortExportTable() comparison logic.
DataTable.type('num-comma', {
    detect: function (d) {
        return /^-?\d+([.,]\d+)?$/.test((d || '').trim()) ? 'num-comma' : null;
    },
    order: {
        pre: function (d) {
            return parseFloat((d || '').trim().replace(',', '.')) || 0;
        }
    },
    className: 'dt-body-right'
});
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
    // bugs.md datatable item: default language for DataTables (export view).
    if (window.DataTable) {
        DataTable.defaults.language = window.CREAVES_DT_LANG[lang] || window.CREAVES_DT_LANG.fr;
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
