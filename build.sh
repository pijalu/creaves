#!/bin/sh
IMG=muaddib/creaves-test

docker buildx build --platform linux/amd64,linux/arm64 --push -t $IMG .
