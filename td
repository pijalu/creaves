diff --git a/templates/treatments/new.plush.html b/templates/treatments/new.plush.html
index eb2fad8..2f9ba95 100644
--- a/templates/treatments/new.plush.fr.html
+++ b/templates/treatments/new.plush.fr.html
@@ -16,9 +16,13 @@
   <h3 class="d-inline-block">New Treatment for animal <%= treatmentTemplate.AnimalID %></h3>
 </div>
 
+
+<%= if (params["back"]) { %>
+<%= formFor(treatmentTemplate, {action: treatmentsPath({back:params["back"]}), method: "POST"}) { %>
+<% } else {%>
 <%= formFor(treatmentTemplate, {action: treatmentsPath(), method: "POST"}) { %>
+<%}%>
   <input type="hidden" name="AnimalID" value="<%= treatmentTemplate.AnimalID %>" />
-
   <div class="container p-3 my-3 border">
         <div class="form-row">
           <div class="form-group col-md-12">
@@ -77,6 +81,12 @@
     </div>
   
   <button class="btn btn-success" role="submit">Save</button>
+
+  
+  <%= if (params["back"]) { %>
+  <%= linkTo(params["back"], {class: "btn btn-warning", "data-confirm": "Are you sure?", body: "Cancel"}) %>
+  <% } else { %>
   <%= linkTo(treatmentsPath(), {class: "btn btn-warning", "data-confirm": "Are you sure?", body: "Cancel"}) %>
+  <% } %>
 <% } %>
 <% } %>
\ No newline at end of file
