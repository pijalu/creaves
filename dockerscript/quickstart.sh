#!/bin/sh

echo "Quick start creaves (dev)"
unset GO_ENV
/bin/app migrate && \
    /bin/app task db:seed && \
    /bin/app
