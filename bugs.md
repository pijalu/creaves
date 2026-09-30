# Type/Species Suggestions & Validation — Bug Follow-up

Tracking document for the reception/new suggestion-cache bug and the type/species
mismatch handling changes. All work is in the `creaves/` project.

**Guideline** (same as `/Users/muaddib/dev/creaves.project/bugs.md`):
1. Create a detailed fix plan for each bug - the plan must contain test approach and validation steps - execute the plan and validate the fix when all elements are in place.
2. Any issues found must be fixed and the fix plan must be updated accordingly.
3. Issues found during testing must be fixed and the fix plan must be updated accordingly.
4. Each bug should be moved to docs/archive when tested and closed as the associated plan.
5. **All changes must be tested, including e2e testing using the [agent-browser skill](../.agents/skills/agent-browser/SKILL.md)** — no fix is complete without e2e evidence (commands, URL, captured output).
6. Use interactive shell/filmstrip to validate the output of the tool - you must verify the actual terminal output using agent-browser skill
7. Check code quality with each tool run separately (do not chain them with `;` or `&&`):
- `go vet ./...`
- `staticcheck ./...`
- `gocognit -over 15 .`
- `gocyclo -over 12 .`
- `go test -count=1 -race -cover ./...`
Fix any issues.
8. Commit each fix with a clear and descriptive commit message

### Session constraints
- **creaves contains real production data**: never drop/destroy data; schema changes must be additive/backward compatible (no destructive migrations).

At the end of the session - the bug list must be empty, all changes committed and resolved entries archived in `docs/archive/`. If new items are added, restart the process.

---
# TODO
## Treatment view in animal
1/ Treatment view on animal *must* be based on protocol/actions - currently it seems to be based on obsolete treatment record
2/ The view still show the 3 time icons but should show the time series (instead of time icons morning/noon/evening it should be the actual time label - 3 per line - following a pseudo-group (morning/noon/evening)))

## Dashboard: medication view
- The table should use striped rows (.table-striped)
- The animal button should follow the same format as the other tables in the dashboard (only number ) 
- No need to have a number badge on the table
- the treatment per animal view should use a dedicated background (white or inverse stripped) to underline the group of treatments for that animal
- The treatment does not have to show a time if the time is part of the button
- the treatment does not need to have 2 lines:
```
Citramox L.A. (48H) — 0.06 ml IM
Traitement — Citramox L.A. (48H) · 12:00
```
=> The "Traitement — Citramox L.A. (48H) · 12:00" subline is not needed
- The treatment view should open the view + popup

- The treatment should group same treatments together (same name/dose) and show the serie of time buttons - following the "Treatment view in animal" 3 items per line todo.

## Care plan view
The plan should follow general style (dashboard/treatment view) for medications
=> A clear list - grouped time buttons list for same treatments

Filter approach is not working - selecting/unselection of filters do not show correct details

The view does not make sense: Some items are in the accordion - some are not (the accordion should be used for grouping all elements)

Only 1 bagdes - eg: currently , following the filter there can be "0 future" followed by "12:00 (17)" - seems like the second badge is the global count... but is meaning less in filtered view

Compact/detailled show different list of element - unclear - eg: currently detailed will show
```

Détails de l'entrée
×

Animal
    130/26 · Renard roux · Enclos Renards
Protocole
    —
Source
    Canidés bébé — nourrissage groupe
Type d'action
    Nourrissage
Échéance
    08:00
Statut
    En retard
```

When it's 07:54 so... not really late 
=> the view should be follow some rules to not confuse "late" with "current" and apply a clear "filter" to not complain on late items if there is an upcoming action. This need reviews

Feeding cards are unclear: http://localhost:3000/care_plan?view=compact&kind=feeding
=> It will show 
```
B1 ACCUEIL graines poules, herbe, eau // mettre l’eau à côté de la nourriture
● 1904/26
En retard
```
No action possible - no clear indication of what to do next
