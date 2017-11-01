package stupid

import (
	"fmt"
	"github.com/tealeg/xlsx"
	"strings"
)

type Position struct {
	Name  string
	Sheet string
	Col   string
	Row   int
}

var (
	layout    [100]Position
	Positions int
)

func AddPosition(name string, sheet string, col string, row int) {
	layout[Positions] = Position{
		Name:  name,
		Sheet: sheet,
		Col:   col,
		Row:   row,
	}
	Positions++
}
func ColIDX(name string) int {
	var res int
	l := strings.ToUpper(name)
	b := []byte(l)
	res = int(b[0])
	res = res - 65
	return res
}

func (*RFC) Set(file *xlsx.File, name string, value string) {
	for _, pos := range layout {
		if pos.Name == "" {
			continue
		}
		if pos.Name == name {
			colpos := ColIDX(pos.Col)
			fmt.Printf("Found %s in %s %d,%d\n", name, pos.Col, colpos, pos.Row)
			sheet := file.Sheet[pos.Sheet]
			cell := sheet.Cell(pos.Row-1, colpos)
			cell.SetValue(value)
			fmt.Println("Cell:", cell)
			return
		}
	}
	fmt.Println("Not found: %s\n", name)
}

type RFC struct {
	file *xlsx.File
}

func CreateNew() *RFC {
	var err error

	AddPosition("customer", "RFC Form", "A", 4)
	AddPosition("requestor.name", "RFC Form", "D", 4)
	AddPosition("requestor.email", "RFC Form", "D", 5)
	AddPosition("requestor.telephone", "RFC Form", "D", 6)
	AddPosition("approver.name", "RFC Form", "I", 4)
	AddPosition("approver.email", "RFC Form", "I", 5)
	AddPosition("approver.telephone", "RFC Form", "I", 6)
	AddPosition("changetype", "RFC Form", "A", 9)
	AddPosition("configitem", "RFC Form", "A", 12)
	AddPosition("discipline", "RFC Form", "C", 9)
	AddPosition("changewindow.start", "RFC Form", "I", 9)
	AddPosition("changewindow.end", "RFC Form", "I", 10)
	AddPosition("changewindow.tz", "RFC Form", "I", 11)
	AddPosition("changetitle", "RFC Form", "A", 15)
	AddPosition("customer.reference", "RFC Form", "C", 12)
	AddPosition("rolloutplan", "RFC Form", "A", 25)
	AddPosition("rollbackplan", "RFC Form", "A", 29)
	AddPosition("testplan", "RFC Form", "A", 33)

	AddPosition("acl.type", "Network RFC Details", "B", 7)
	AddPosition("acl.device", "Network RFC Details", "C", 7)
	AddPosition("acl.interface", "Network RFC Details", "D", 7)
	AddPosition("acl.comment", "Network RFC Details", "E", 7)
	AddPosition("acl.purpose", "Network RFC Details", "F", 7)
	AddPosition("acl.source", "Network RFC Details", "G", 7)
	AddPosition("acl.destination", "Network RFC Details", "H", 7)
	AddPosition("acl.port", "Network RFC Details", "I", 7)
	AddPosition("acl.proto", "Network RFC Details", "J", 7)
	AddPosition("acl.action", "Network RFC Details", "K", 7)
	AddPosition("acl.bidirectional", "Network RFC Details", "L", 7)

	stupidTemplate := "rfcmaster.xlsx"
	rfc := new(RFC)
	rfc.file, err = xlsx.OpenFile(stupidTemplate)
	if err != nil {
		fmt.Printf("Failed to open %s: %s", stupidTemplate, err)
		return nil
	}
	return rfc
	err = rfc.file.Save("new-rfc.xlsx")
	if err != nil {
		fmt.Printf(err.Error())
	}
	return nil
}
