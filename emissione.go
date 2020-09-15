package turtleware

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/kernle32dll/emissione-go"
)

var (
	// use a custom json writer, which uses jsoniter.
	jsonWriter = emissione.NewJSONWriter(emissione.MarshallMethod(func(v interface{}) ([]byte, error) {
		return jsoniter.MarshalIndent(v, "", "  ")
	}))

	xmlWriter = emissione.NewXmlIndentWriter("", "  ")

	// EmissioneWriter is the globally used writer for writing out response bodies.
	EmissioneWriter = emissione.New(jsonWriter, emissione.WriterMapping{
		"application/json":                jsonWriter,
		"application/json;charset=utf-8":  jsonWriter,
		"application/json; charset=utf-8": jsonWriter,
		"application/xml":                 xmlWriter,
		"application/xml;charset=utf-8":   xmlWriter,
		"application/xml; charset=utf-8":  xmlWriter,
	})
)
