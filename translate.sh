#!/bin/bash

# Usage:
#   ./translate.sh        legacy behavior: copy missing locales/X.en-us.yaml -> X.fr.yaml
#   ./translate.sh de     copy missing locales/X.en-us.yaml -> X.de.yaml
#   ./translate.sh nl     copy missing locales/X.en-us.yaml -> X.nl.yaml

target="$1"

case "$target" in
"" )
	suffix="fr"
	;;
de | nl )
	suffix="$target"
	;;
* )
	echo "unsupported locale: $target (expected de or nl)" >&2
	exit 1
	;;
esac

for l in `find locales -name \*.en-us.yaml`
do
	newfile=`echo $l | sed "s/en-us\.yaml/${suffix}\.yaml/g"`
	if [[ ! -f $newfile ]]
	then
		cp $l $newfile
		echo "created $newfile"
	fi
done
