#!/bin/sh

docker build . -t muaddib/vica \
    && docker push muaddib/vica
