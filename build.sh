#!/bin/sh

docker build . -t muaddib/creaves-update-xmas \
    && docker push muaddib/creaves-update-xmas
