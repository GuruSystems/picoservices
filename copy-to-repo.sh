#!/bin/sh

# this script will copy the files required for this repo
# to another repos 'vendor' folder.

# requires vendor folder as parameter

VENDOR=$1
if [ -z "$VENDOR" ]; then
    echo Missing vendor parameter as first argument
    exit 10
fi

SRC=`dirname $0`

if [ ! -d $VENDOR ]; then
    echo $VENDOR does not exit
    exit 10
fi

VENDOR="`pwd`/$VENDOR"
pwd
echo "Copying repo $SRC to $VENDOR"

(cd ${SRC} ; git diff --quiet)
CODE=$?
if [ ${CODE} -ne 0 ]; then
    echo "Refusing to patch from a locally modified version"
    exit 10
fi

cd ${SRC}/src
find . | grep -v vendor | grep -v '*'|grep -v '~$'| grep -v '/.git'|grep -v 'dist'|grep -v 'vendor'|cpio -pav --make-directories --unconditional ${VENDOR}
