#!/bin/sh

echo "Starting creaves (production)"
export GO_ENV=production
/bin/app migrate && \
    /bin/app task db:seed && \
    exec /bin/app
