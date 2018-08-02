package main

import (
	"bytes"
	"log"
	"net"
	"net/http"
	"net/rpc"

	"io/ioutil"
	"os"

	"plugin"
	"reflect"

	"github.com/2tvenom/cbor"
	"github.com/insolar/insolar/logicrunner"
	"github.com/insolar/insolar/logicrunner/goplugin"
)

type GoInsider struct {
	dir string
}

func NewGoInsider(path string) *GoInsider {
	//TODO: check that path exist, it's a directory and writable
	return &GoInsider{path}
}

func (t *GoInsider) Call(args goplugin.CallReq, reply *goplugin.CallResp) error {
	path, err := t.ObtainCode(args.Object)
	check(err)

	p, err := plugin.Open(path)
	check(err)

	export, err := p.Lookup("INSEXPORT")
	check(err)

	var data_buf bytes.Buffer
	cbor := cbor.NewEncoder(&data_buf)
	_, err = cbor.Unmarshal(args.Object.Data, export)
	check(err)

	method := reflect.ValueOf(export).MethodByName("INSMETHOD__" + args.Method)
	if !method.IsValid() {
		panic("wtf, no method " + args.Method + "in the plugin")
	}

	res := method.Call([]reflect.Value{})

	cbor.Marshal(export)
	reply.Data = data_buf.Bytes()

	log.Printf("res: %+v\n", res)

	return nil
}

func (t *GoInsider) ObtainCode(obj logicrunner.Object) (string, error) {
	path := t.dir + "/" + obj.Reference
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := ioutil.WriteFile(path, obj.Code, 0666)
		check(err)
	} else {
		check(err)
	}
	return path, nil
}

var PATH = "/Users/ruz/go/src/github.com/insolar/insolar/tmp"

func main() {
	log.Print("ginsider launched")
	insider := GoInsider{PATH}
	rpc.Register(insider)
	rpc.HandleHTTP()
	l, e := net.Listen("tcp", ":7777")
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
	<-make(chan byte)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
