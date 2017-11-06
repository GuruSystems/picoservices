#!/bin/sh
# go insists on absolute path.
export GOBIN=`pwd`/dist
export GOPATH=`pwd`
echo "GOPATH=$GOPATH"
mkdir $GOBIN
BUILD() {
    echo
    echo "building $1"
MYSRC=src/golang.conradwood.net/$1
( cd ${MYSRC} && make proto ) || exit 10
( cd ${MYSRC} && make client ) || exit 10
( cd ${MYSRC} && make server ) || exit 10
cp -rvf ${MYSRC}/proto dist/
}

BUILD auth
BUILD authserver
BUILD registrar
BUILD keyvalueserver

env

build-repo-client -branch=${GIT_BRANCH} -build=${BUILD_NUMBER} -commitid=${COMMIT_ID} -commitmsg="commit msg unknown" -repository=${PROJECT_NAME} -server_addr=apps.gurusys.co.uk:5004

exit 0
