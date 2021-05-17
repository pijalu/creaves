function submitfix() {
     jQuery('form').submit(function(){
          $(this).find(':submit').attr( 'disabled','disabled' );
     });
}

// Export as global
global.submitfix = submitfix;