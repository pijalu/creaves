function setupWizard() {
    console.log("Wizard setup");

    var navListItems = $('div.setup-panel div a'),
        allWells = $('.setup-content'),
        allNextBtn = $('.nextBtn'),
        allPrevBtn = $('.prevBtn'),
        allForms = $('form');

    allWells.hide();

    allForms.on("submit", function (e) {
       var v = $(this).isValid();
       if (v) {
            $('button[type=submit], input[type=submit]').attr('disabled',"disabled");
       }
       return v;
    });

    navListItems.on("click", function (e) {
        e.preventDefault();
        var $target = $($(this).attr('href')),
            $item = $(this);

        if (!$item.hasClass('disabled')) {
            navListItems.removeClass('btn-primary').addClass('btn-default');
            $item.addClass('btn-primary');
            allWells.hide();
            $target.show();
            $target.find('input:eq(0)').focus();
        }
    });

    allPrevBtn.on("click", function () {
        var curStep = $(this).closest(".setup-content"),
            curStepBtn = curStep.attr("id"),
            prevStepWizard = $('div.setup-panel div a[href="#' + curStepBtn + '"]').parent().prev().children("a");

        prevStepWizard.removeClass("disabled").trigger("click");
    });

    allNextBtn.on("click", function () {
        var curStep = $(this).closest(".setup-content"),
            curStepBtn = curStep.attr("id"),
            nextStepWizard = $('div.setup-panel div a[href="#' + curStepBtn + '"]').parent().next().children("a"),
            curInputs = curStep.find("input[type='text'],input[type='url'],select"),
            isValid = true;

        for (var i = 0; i < curInputs.length; i++) {
            if (!curInputs[i].validity.valid) {
                console.log(curInputs[i], "is not valid");
                isValid = false;
                console.log("Close:",  $(curInputs[i]).closest(".form-group"));
                $(curInputs[i]).closest(".form-group").addClass("is-invalid");
                $(curInputs[i]).addClass("is-invalid").removeClass("is-valid");
            } else {
                $(curInputs[i]).closest(".form-group").addClass("is-valid");
                $(curInputs[i]).addClass("is-valid").removeClass("is-invalid");
            }
        }

        if (isValid) {
            nextStepWizard.removeClass("disabled").trigger("click");
            //nextStepWizard.removeAttr('disabled').trigger('click');
        } else {
            nextStepWizard.addClass("disabled");
            //nextStepWizard.attr('disabled', true);
        }
    });

    $('div.setup-panel div a.btn-primary').trigger('click');
}

// Export as global
global.setupWizard = setupWizard;