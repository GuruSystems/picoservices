#!/bin/sh
if [ ! -x dist/amd64/registrar-server ]; then
./autobuild.sh || exit 10
fi

GURUPATH="local/testing/picoservices/1"
dist/amd64/registrar-server &
dist/amd64/auth-server -deployment_gurupath=${GURUPATH} -backend=any &
dist/amd64/keyvalueserver-server -deployment_gurupath=${GURUPATH} &
