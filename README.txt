A collection of services to make up an infrastructure:

* Service Registry
* Client-side service discovery stub
* Authentication (server & client side stub)

planned:
* central logging (unless systemd has a good way of doing this)

it's basic, it's only a few days work...


The layout of this repository is such that one can point GOPATH at the root of
this repositry. it includes all dependencies (on purposes)

Getting started:

1) run ./autobuild.sh from the toplevel directory. This creates binaries under dist/

2) start dist/registrar-server

3) start dist/keyvalueserver-server
( you should see a message in the registrar service that it registers)

4) start dist/keyvalueserver-client
( you should see a messsage in the registrar service that it looksup the
address of the keyvalueserver)

5) it should deny access. The "authentication" service isn't implemented yet :)





