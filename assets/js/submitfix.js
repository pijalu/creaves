function submitfix() {
    $('form').on("submit", function (e) {
        var v = $(this).isValid();
        if (v) {
             $('button[type=submit], input[type=submit]').attr('disabled',"disabled");
        }
        return v;
     });
}

// Export as global
global.submitfix = submitfix;