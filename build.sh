#!/bin/sh
IMG=muaddib/creaves-ng

docker buildx build --platform linux/amd64,linux/arm64 --push -t $IMG .
