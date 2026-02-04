package main

import (
	"fmt"

	"github.com/MetalBlockchain/metalgo/ids"
)

var vmID ids.ID

const vmName = "btcvm"

func main() {
	b := make([]byte, 32)
	copy(b, []byte(vmName))
	var err error
	vmID, err = ids.ToID(b)
	if err != nil {
		panic(err)
	}

	fmt.Println(vmID)
}
