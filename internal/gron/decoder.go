package gron

import (
	"encoding/json"
	"io"

	"gopkg.in/yaml.v3"
)

// an ActionFn represents a main action of the program, it accepts
// an input, output and a bitfield of options; returning an exit
// code and any error that occurred
type ActionFn func(io.Reader, io.Writer, int) (int, error)

type Decoder interface {
	Decode(interface{}) error
}

func MakeDecoder(r io.Reader, asYaml bool) Decoder {
	if asYaml {
		return yaml.NewDecoder(r)
	} else {
		d := json.NewDecoder(r)
		d.UseNumber()
		return d
	}
}