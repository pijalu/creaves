#!/bin/sh

docker build . -t muaddib/creaves \
    && docker push muaddib/creaves
