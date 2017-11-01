/*
Package compound provides a common startup/teardown code for Go based 
microservices.
It's intent is to provide an easy upgrade path when the common
infrastructure needs change.
This package, whilst incomplete, is intented to handle
* service discovery
* monitoring setup (exposing metrics)
* fail-over and load-balancing
* authentication of services & users

TODO(cnw): currently it's one big library, most of this stuff should
probably be moved out into its own service (especially db access)
*/
