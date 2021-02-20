#!/bin/sh

echo "Quick start creaves (dev) - admin/admin to login"
unset GO_ENV
/bin/app migrate && \
    /bin/app task db:create_admin && \
    /bin/app
