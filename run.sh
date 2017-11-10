#!/bin/sh
if [ ! -x dist/amd64/registrar-server ]; then
./autobuild.sh || exit 10
fi

for thi in `ls dist/amd64/*-server`; do
       $thi &
done
