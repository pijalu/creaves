function submitfix() {
     $('form').on("submit", function(){
          $(this).find(':submit').prop('disabled','disabled');
     });
}

// Export as global
global.submitfix = submitfix;