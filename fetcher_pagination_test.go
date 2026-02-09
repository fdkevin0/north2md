package south2md

import (
	"strings"
	"testing"
)

func TestResolvePageFetchResultsStrictModeReturnsError(t *testing.T) {
	page1 := NewPostParser()
	page2 := NewPostParser()

	_, err := resolvePageFetchResults([]*PostParser{page1, page2, nil}, []int{3}, true)
	if err == nil {
		t.Fatal("expected strict pagination error")
	}
	if !strings.Contains(err.Error(), "缺失页: [3]") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolvePageFetchResultsNonStrictModeSkipsFailedPages(t *testing.T) {
	page1 := NewPostParser()
	page2 := NewPostParser()

	parsers, err := resolvePageFetchResults([]*PostParser{page1, page2, nil}, []int{3}, false)
	if err != nil {
		t.Fatalf("unexpected error in non-strict mode: %v", err)
	}
	if len(parsers) != 2 {
		t.Fatalf("expected 2 parsers, got %d", len(parsers))
	}
}
