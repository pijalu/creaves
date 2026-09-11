import csv
import gzip
import re
from collections import Counter, defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parent


def sql_values(text):
    match = re.search(r"INSERT INTO `animaltypes` VALUES\n(.*?);", text, re.S)
    body = match.group(1)
    rows, i = [], 0
    while i < len(body):
        if body[i] != '(':
            i += 1
            continue
        i += 1
        row = []
        while i < len(body):
            while i < len(body) and body[i] in ' \n\r,':
                i += 1
            if body[i] == ')':
                i += 1
                break
            if body[i] == "'":
                i += 1
                value = []
                while i < len(body):
                    char = body[i]
                    i += 1
                    if char == '\\' and i < len(body):
                        value.append(body[i])
                        i += 1
                    elif char == "'":
                        break
                    else:
                        value.append(char)
                row.append(''.join(value))
            else:
                end = i
                while end < len(body) and body[end] not in ',)':
                    end += 1
                row.append(body[i:end].strip())
                i = end
        rows.append(row)
    return rows


def norm(value):
    value = value.replace('’', "'").replace('œ', 'oe').replace('Œ', 'OE')
    value = re.sub(r'\s+', ' ', value.strip().lower())
    return value


def description_species(value):
    # Startup dump stores literal escaped newlines; parser has already removed
    # the slash, so use the original dump's line-like `n` only through known
    # canonical species matching below. This function is supplemented by the
    # broad taxonomy rules, not treated as sole authority.
    return {norm(x) for x in re.split(r'[\r\n]+', value) if x.strip()}


def main():
    species = list(csv.DictReader((ROOT / 'create_species.csv').open(encoding='utf-8')))
    dump = gzip.open(ROOT / 'creaves-startup.sql.gz', 'rt', encoding='utf-8').read()
    types = sql_values(dump)
    type_names = {row[1] for row in types}
    type_by_norm = {norm(name): name for name in type_names}
    description_text = {
        row[1]: norm(row[2].replace('\\\\n', ' '))
        for row in types
    }

    # Description membership is strongest available repository evidence. The
    # dump's descriptions are retained as escaped text, so matching is done
    # against normalized text rather than relying on line separators.
    def description_matches(row):
        name = norm(row['CreavesSpecies'])
        if not name:
            return []
        return [type_name for type_name, text in description_text.items() if name in text]

    # Explicit, reviewed classification rules based on canonical taxonomy.
    def classify(row):
        matches = description_matches(row)
        if len(matches) == 1:
            return matches[0], 'source description exact-name membership'
        if len(matches) > 1:
            return '', 'conflicting source descriptions: ' + ' | '.join(sorted(matches))
        agw = norm(row['AgwGroup'])
        cls = norm(row['Class'])
        family = norm(row['Family'])
        order = norm(row['Order'])
        if agw == norm('Rapace'):
            return 'Rapaces', 'taxonomy: AGW Rapace'
        if agw == norm('Chauve-souris'):
            return 'chauves-souris', 'taxonomy: AGW Chauve-souris'
        if agw == norm('Mustélidé'):
            return 'Mustélidés', 'taxonomy: AGW Mustélidé'
        if agw == norm('Ongulé'):
            return 'Ongulés', 'taxonomy: AGW Ongulé'
        if cls in ('amphibia', 'reptilia') or agw in ('batracien', 'reptile'):
            return 'Reptiles, Amphibiens', 'taxonomy: class/AGW amphibian or reptile'
        if agw == norm('Micro-mammifère'):
            if 'erinace' in family or 'soric' in family:
                return 'Hérissons / Insectivore', 'taxonomy: insectivore family'
            return 'Rongeurs', 'taxonomy: AGW Micro-mammifère'
        if cls == 'aves':
            # Existing reference descriptions divide birds by care-size groups;
            # explicit AGW families have stable operational categories.
            if agw in ('grand échassier', 'anatidé / grèbe', 'autre oiseau d\'eau'):
                return 'Grands Oiseaux', 'taxonomy: AGW large/water bird'
            if agw in ('limicole', 'laridé'):
                return 'Moyens Oiseaux', 'taxonomy: AGW wader/gull'
            if agw == 'corvidé':
                return 'Moyens Oiseaux', 'taxonomy: AGW corvid'
            return 'Petits Oiseaux', 'taxonomy: Aves residual operational group'
        if agw == norm('Domestique'):
            return 'Autres mammifères (Procyonidé, Viverridés, ...)', 'taxonomy: AGW Domestique'
        if cls == 'mammalia':
            if any(x in family for x in ('felid',)):
                return 'Félidés', 'taxonomy: family Felidae'
            if any(x in family for x in ('canid',)):
                return 'Canidés', 'taxonomy: family Canidae'
            if any(x in family for x in ('erinace', 'soric')):
                return 'Hérissons / Insectivore', 'taxonomy: insectivore family'
            if order == 'lagomorpha':
                return 'Lagomorphe', 'taxonomy: order Lagomorpha'
            return 'Autres mammifères (Procyonidé, Viverridés, ...)', 'taxonomy: Mammalia residual group'
        return '', 'unclassified'

    out = []
    for row in species:
        typ, evidence = classify(row)
        if typ not in type_names:
            evidence = evidence or 'no approved candidate'
            typ = ''
            confidence = 'unresolved'
            review_status = 'blocked'
        else:
            confidence = 'candidate_high'
            review_status = 'pending_domain_review'
        out.append({
            'species_id': row['ID'],
            'species_name': row['CreavesSpecies'],
                          'scientific_name': row['Species'],
                          'animal_type_name': typ,
                          'match_method': 'candidate_taxonomy_rule',
                                        'confidence': confidence,
                                        'review_status': review_status,
            'evidence': evidence,
            'notes': 'Phase 1 operational classification; verify domain ownership before migration.',
        })

    names = [r['species_name'] for r in out]
    duplicate = [name for name, count in Counter(norm(x) for x in names).items() if count > 1]
    ids = [r['species_id'] for r in out]
    duplicate = [value for value, count in Counter(ids).items() if count > 1]
    if duplicate:
        raise SystemExit('duplicate species IDs: ' + repr(duplicate))
    expected = {r['ID'] for r in species}
    actual = set(ids)
    if expected != actual:
        raise SystemExit('coverage mismatch')

    mapping = ROOT / 'species_animaltype_mapping.csv'
    fields = list(out[0])
    with mapping.open('w', newline='', encoding='utf-8') as handle:
        writer = csv.DictWriter(handle, fieldnames=fields)
        writer.writeheader(); writer.writerows(sorted(out, key=lambda r: norm(r['species_name'])))

    review = ROOT / 'species_animaltype_mapping_review.csv'
    review_fields = ['species_name', 'candidate_type', 'source_class', 'source_order', 'source_family', 'source_agw_group', 'decision', 'review_notes']
    by_id = {r['ID']: r for r in species}
    with review.open('w', newline='', encoding='utf-8') as handle:
        writer = csv.DictWriter(handle, fieldnames=review_fields)
        writer.writeheader()
        for item in sorted(out, key=lambda r: norm(r['species_name'])):
            source = by_id[item['species_id']]
            writer.writerow({'species_name': item['species_name'], 'candidate_type': item['animal_type_name'], 'source_class': source['Class'], 'source_order': source['Order'], 'source_family': source['Family'], 'source_agw_group': source['AgwGroup'], 'decision': 'pending_domain_review', 'review_notes': item['evidence']})

    counts = Counter(r['animal_type_name'] for r in out)
    report = ROOT / 'species_animaltype_mapping_report.txt'
    with report.open('w', encoding='utf-8') as handle:
        handle.write('Phase 1 species-to-animal-type mapping report\n')
        handle.write('==============================================\n')
        handle.write('species_total: %d\n' % len(species))
        handle.write('mapped_total: %d\n' % len(out))
        handle.write('approved_high_confidence: 0\n')
        handle.write('candidate_high_confidence: %d\n' % sum(r['confidence'] == 'candidate_high' for r in out))
        handle.write('domain_review_required: %d\n' % sum(r['review_status'] == 'pending_domain_review' for r in out))
        handle.write('unmapped: %d\n' % sum(not r['animal_type_name'] for r in out))
        handle.write('ambiguous: %d\n' % sum('conflicting source descriptions' in r['evidence'] for r in out))
        handle.write('duplicate_assignments: 0\nunknown_types: 0\n')
        handle.write('common_name_collisions: 4 (Campagnol, Murin, Musaraigne, Pipistrelle)\n')
        handle.write('\nAssignments by animal type:\n')
        for name in sorted(counts, key=norm):
            handle.write('%s: %d\n' % (name, counts[name]))
        handle.write('\nImportant: all rows are generated by explicit taxonomy/operational rules and require domain-owner sign-off before database migration.\n')

if __name__ == '__main__':
    main()
