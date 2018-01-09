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

pwd
echo "Copying repo $SRC to $VENDOR"

cp -rv $SRC/src/* ${VENDOR}
