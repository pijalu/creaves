#!/bin/sh

for f in `find . type f -name *.plush.html`
do
    rpl -i 'body: "View"' 'title: "View", body:"<i class='\''fa fa-eye'\''></i>"' $f
    rpl -i 'body: "Edit"' 'title: "Edit", body:"<i class='\''fa fa-edit'\''></i>"' $f
    rpl -i 'body: "Destroy"' 'title: "Destroy", body:"<i class='\''fa fa-trash'\''></i>", style:"width:44px"' $f
done