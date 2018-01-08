package main

// maintain prometheus yaml files
// for the targets

import (
	"flag"
	"fmt"
	pb "golang.conradwood.net/registrar/proto"
	"io/ioutil"
	"strings"
	"sync"
)

const (
	YAML_ID = "# this yaml file was written by the registry"
)

var (
	targetsdir = flag.String("prometheus_targets", "", "Directory to store targets for prometheus in. (empty==no targetfiles are maintained)")
	targets    []*target
	promlock   sync.Mutex
)

type target struct {
	name string
	addr []string
}

func UpdateTargets() {
	if *targetsdir == "" {
		return
	}

	// we don't want to run multi-threaded, we're writing files!
	promlock.Lock()
	defer promlock.Unlock()

	targets = targets[:0] // clear targets

	fmt.Printf("Creating list of prometheus targets...\n")

	for e := services.Front(); e != nil; e = e.Next() {
		se := e.Value.(*serviceEntry)
		for _, si := range se.instances {
			if !si.hasApi(pb.Apitype_status) {
				continue
			}
			tname := targetName(se.desc.Name)
			tg := getTargetByName(tname)
			addr := fmt.Sprintf("%s:%d", si.address.Host, si.address.Port)
			tg.addr = append(tg.addr, addr)
			// fmt.Printf("  %s (%s)\n", tname, addr)
		}
	}

	err := writeTargets()
	if err != nil {
		fmt.Printf("Failed to write targets: %s\n", err)
	}
}

func writeTargets() error {
	var err error
	for _, t := range targets {
		fname := fmt.Sprintf("%s/%s.yaml", *targetsdir, t.name)
		s := fmt.Sprintf("%s\n- targets:\n", YAML_ID)
		for _, adr := range t.addr {
			s = fmt.Sprintf("%s   - %s\n", s, adr)
		}
		e := ioutil.WriteFile(fname, []byte(s), 0644)
		if e != nil {
			err = e
		}
		fmt.Println(s)
	}

	// delete yaml files which should not be in there
	if err == nil {
		e := DeleteOldTargets()
		if e != nil {
			err = e
		}
	}

	return err
}

func DeleteOldTargets() error {
	fis, err := ioutil.ReadDir(*targetsdir)
	if err != nil {
		fmt.Println(err)
		return err
	}
	for _, fi := range fis {
		fname := fi.Name()
		if !strings.HasSuffix(fname, ".yaml") {
			continue
		}
		ffname := fmt.Sprintf("%s/%s", *targetsdir, fname)
		if !isOurFile(ffname) {
			continue
		}

		tname := fname[:len(fname)-5]
		fmt.Printf("File: %s, target: \"%s\"\n", fname, tname)
		targets := getTargetByName(tname)
		if len(targets.addr) == 0 {
			WriteEmptyFile(ffname)
			fmt.Printf("Removed file %s\n", ffname)
		}
	}
	return nil
}

func WriteEmptyFile(fname string) {
	s := fmt.Sprintf("%s\n", YAML_ID)
	ioutil.WriteFile(fname, []byte(s), 0644)
}
func isOurFile(fname string) bool {
	bs, err := ioutil.ReadFile(fname)
	if err != nil {
		fmt.Println(err)
		return false
	}
	s := string(bs)
	sx := strings.SplitN(s, "\n", 2)
	if len(sx) < 1 {
		return false
	}
	fmt.Printf("ID: \"%s\" in %s\n", sx[0], fname)
	if sx[0] == YAML_ID {
		return true
	}
	return false
}

func getTargetByName(name string) *target {
	for _, t := range targets {
		if t.name == name {
			return t
		}
	}
	t := &target{name: name}
	targets = append(targets, t)
	return t
}
func targetName(name string) string {
	x := strings.Split(name, ".")
	return x[0]
}
