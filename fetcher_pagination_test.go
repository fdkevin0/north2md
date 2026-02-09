package south2md

import (
	"strings"
	"testing"
)

func TestResolvePageFetchResultsStrictModeReturnsError(t *testing.T) {
	selectors := defaultSelectorsForPaginationTest()
	page1 := NewPostParser(selectors)
	page2 := NewPostParser(selectors)

	_, err := resolvePageFetchResults([]*PostParser{page1, page2, nil}, []int{3}, true)
	if err == nil {
		t.Fatal("expected strict pagination error")
	}
	if !strings.Contains(err.Error(), "缺失页: [3]") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolvePageFetchResultsNonStrictModeSkipsFailedPages(t *testing.T) {
	selectors := defaultSelectorsForPaginationTest()
	page1 := NewPostParser(selectors)
	page2 := NewPostParser(selectors)

	parsers, err := resolvePageFetchResults([]*PostParser{page1, page2, nil}, []int{3}, false)
	if err != nil {
		t.Fatalf("unexpected error in non-strict mode: %v", err)
	}
	if len(parsers) != 2 {
		t.Fatalf("expected 2 parsers, got %d", len(parsers))
	}
}

func defaultSelectorsForPaginationTest() *HTMLSelectors {
	cfg := NewDefaultConfig()
	return &HTMLSelectors{
		Title:       cfg.SelectorTitle,
		Forum:       cfg.SelectorForum,
		PostTable:   cfg.SelectorPostTable,
		AuthorName:  cfg.SelectorAuthorName,
		PostTime:    cfg.SelectorPostTime,
		PostContent: cfg.SelectorPostContent,
		Floor:       cfg.SelectorFloor,
		AuthorInfo:  cfg.SelectorAuthorInfo,
		Avatar:      cfg.SelectorAvatar,
		Images:      cfg.SelectorImages,
	}
}
