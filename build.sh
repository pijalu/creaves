#!/bin/sh

docker build . -t muaddib/creaves-upgrade \
    && docker push muaddib/creaves-upgrade
