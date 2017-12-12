#!/bin/sh

# dodgy script, but see readme on target

TARGET=master.fra
ssh ${TARGET} "sudo systemctl stop auth-server registrar "
rsync --inplace -pvra dist/amd64/*-server ${TARGET}:/srv/deployments
ssh ${TARGET} "sudo systemctl start auth-server registrar "
