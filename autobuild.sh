#!/bin/sh
# go insists on absolute path.
export GOBIN=`pwd`/dist
export GOPATH=`pwd`
echo "GOPATH=$GOPATH"
mkdir $GOBIN
MYSRC=src/golang.conradwood.net/softcat-rfc-creator
( cd ${MYSRC} && make proto ) || exit 10
( cd ${MYSRC} && make client ) || exit 10
( cd ${MYSRC} && make server ) || exit 10
cp -rvf ${MYSRC}/proto dist/
exit 0
