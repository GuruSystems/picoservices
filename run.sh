#!/bin/sh
if [ ! -x dist/amd64/registrar-server ]; then
./autobuild.sh || exit 10
fi

dist/amd64/registrar-server &
dist/amd64/auth-server -backend=any &
dist/amd64/keyvalueserver-server &
