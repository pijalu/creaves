#!/bin/sh

# Wait for the database to be ready
echo "Waiting for database..."
while ! nc -z db 3306; do
  sleep 1
done
echo "Database is ready."

# Run migrations, seed the database, and start the app
echo "Starting creaves (development)"
/bin/app migrate && \
    /bin/app task db:seed && \
    exec /bin/app
