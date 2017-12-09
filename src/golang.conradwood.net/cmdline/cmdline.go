package cmdline

import (
	"flag"
	"fmt"
	"strings"
)

var (
	registry = flag.String("registry", "localhost:5000", "address of the registrar server (for lookups)")
)

func GetRegistryAddress() string {
	res := *registry
	if !strings.Contains(res, ":") {
		res = fmt.Sprintf("%s:5000", res)
	}
	return res
}
