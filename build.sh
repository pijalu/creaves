#!/bin/sh

docker build . -t muaddib/creaves-upgrade-2 \
    && docker push muaddib/creaves-upgrade-2
