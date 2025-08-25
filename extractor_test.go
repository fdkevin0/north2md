package main_test

import (
	"bytes"
	_ "embed"
	"reflect"
	"testing"

	"github.com/BurntSushi/toml"
	main "github.com/fdkevin0/north2md"
)

var (

	//go:embed tid-2636739.html
	sourcePostHTML []byte

	//go:embed tid-2636739.toml
	expectedPostTOML []byte
)

func TestExtractPostDataFromHTML(t *testing.T) {
	parser := main.NewHTMLParser()
	parser.LoadFromReader(bytes.NewBuffer(sourcePostHTML))

	config := main.NewDefaultConfig()
	extractor := main.NewDataExtractor(&config.Selectors)

	resultPost, err := extractor.ExtractPost(parser)
	if err != nil {
		t.Errorf("Failed to extract post data: %v", err)
	}

	wantPost := &main.Post{}
	_, err = toml.Decode(string(expectedPostTOML), wantPost)
	if err != nil {
		t.Errorf("Failed to decode expected post data: %v", err)
	}

	if !reflect.DeepEqual(*resultPost, *wantPost) {
		t.Errorf("Extracted post data does not match expected data")
	}
}
