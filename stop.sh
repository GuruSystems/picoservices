#!/bin/sh
dist/amd64/registrar-client shutdown auth.AuthenticationService
dist/amd64/registrar-client shutdown keyvalueserver.KeyValueService
killall registrar-server
