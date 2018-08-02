package goplugin

import (
	"testing"

	"github.com/insolar/insolar/logicrunner"

	"bytes"

	"github.com/2tvenom/cbor"
)

type HelloWorlder struct {
	Greeted int
}

func TestHelloWorld(t *testing.T) {
	gp, err := NewGoPlugin("127.0.0.1:7777", "127.0.0.1:7778")
	defer gp.Stop()
	if err != nil {
		t.Fatal(err)
	}
	var buff bytes.Buffer
	e := cbor.NewEncoder(&buff)
	e.Marshal(HelloWorlder{77})

	obj := logicrunner.Object{
		MachineType: logicrunner.MachineTypeGoPlugin,
		Reference:   "reference",
		Data:        buff.Bytes(),
	}

	data, ret, err := gp.Exec(obj, "Hello", logicrunner.Arguments{})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("len of data == 0")
	}
	//	if ret == logicrunner.Arguments{} // IDK, lets decide what must be here
	t.Log(ret)
}
