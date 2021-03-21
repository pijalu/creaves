#!/bin/bash


for l in `find locales -name \*.en-us.yaml`
do
		newfile=`echo $l | sed 's/en-us\.yaml/fr\.yaml/g'`
		if [[ ! -f $newfile ]]
		then
				cp $l $newfile
		fi
done
