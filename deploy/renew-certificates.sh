#!/bin/sh
set -eu

cd /opt/urock
docker compose --profile tools run --rm certbot renew --quiet
docker compose exec -T nginx nginx -s reload
