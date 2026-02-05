package north2md_test

import (
	"bytes"
	_ "embed"
	"fmt"
	"reflect"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/r3labs/diff/v3"

	main "github.com/fdkevin0/north2md"
)

var (

	//go:embed tid-2636739.html
	sourcePostHTML []byte

	//go:embed tid-2636739.toml
	expectedPostTOML []byte
)

func TestExtractPostDataFromHTML(t *testing.T) {
	parser := main.NewPostParser(&main.HTMLSelectors{
		Title:       "h1#subject_tpc",
		Forum:       "#breadcrumbs .crumbs-item.gray3:nth-child(3)",
		PostTable:   "table.js-post",
		AuthorName:  "strong",
		PostTime:    ".tiptop .gray",
		PostContent: "div[id^='read_']",
		Floor:       ".tiptop .fl a",
		AuthorInfo:  ".tiptop .tar",
		Avatar:      "img[src*=\"avatar\"]",
		Images:      "img",
	})
	parser.LoadFromReader(bytes.NewBuffer(sourcePostHTML))

	resultPost, err := parser.ExtractPost()
	if err != nil {
		t.Errorf("Failed to extract post data: %v", err)
	}

	wantPost := &main.Post{}
	_, err = toml.Decode(string(expectedPostTOML), wantPost)
	if err != nil {
		t.Errorf("Failed to decode expected post data: %v", err)
	}

	// 如果仍有差异，显示详细信息
	if !reflect.DeepEqual(*resultPost, *wantPost) {
		changes, err := diff.Diff(*resultPost, *wantPost)
		if err != nil {
			t.Error(err)
		}
		for _, change := range changes {
			fmt.Printf("Field: %s, From: %v, To: %v\n", change.Path, change.From, change.To)
		}

		t.Errorf("Extracted post data does not match expected data")
	}
}
