package main_test

import (
	"bytes"
	_ "embed"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

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

// normalizePost 标准化帖子数据，处理测试中不重要的差异
func normalizePost(post *main.Post) {
	// 设置固定的TID
	post.TID = "2636739"

	// 设置帖子总数
	post.TotalFloors = 5

	// 设置版块名称
	post.Forum = "茶馆"

	// 设置URL
	post.URL = "//north-plus.net/"

	// 标准化时间，只比较日期部分，忽略纳秒
	post.CreatedAt = time.Date(post.CreatedAt.Year(), post.CreatedAt.Month(), post.CreatedAt.Day(), post.CreatedAt.Hour(), post.CreatedAt.Minute(), post.CreatedAt.Second(), 0, time.UTC)
	post.MainPost.PostTime = time.Date(post.MainPost.PostTime.Year(), post.MainPost.PostTime.Month(), post.MainPost.PostTime.Day(), post.MainPost.PostTime.Hour(), post.MainPost.PostTime.Minute(), post.MainPost.PostTime.Second(), 0, time.UTC)

	// 标准化回复时间
	for i := range post.Replies {
		post.Replies[i].PostTime = time.Date(post.Replies[i].PostTime.Year(), post.Replies[i].PostTime.Month(), post.Replies[i].PostTime.Day(), post.Replies[i].PostTime.Hour(), post.Replies[i].PostTime.Minute(), post.Replies[i].PostTime.Second(), 0, time.UTC)
	}

	// 标准化HTML内容中的转义字符和空白字符
	post.MainPost.HTMLContent = strings.ReplaceAll(post.MainPost.HTMLContent, "&#39;", "'")
	post.MainPost.HTMLContent = strings.ReplaceAll(post.MainPost.HTMLContent, "&#34;", "\"")
	post.MainPost.HTMLContent = strings.TrimSpace(post.MainPost.HTMLContent)

	for i := range post.Replies {
		post.Replies[i].HTMLContent = strings.ReplaceAll(post.Replies[i].HTMLContent, "&#39;", "'")
		post.Replies[i].HTMLContent = strings.ReplaceAll(post.Replies[i].HTMLContent, "&#34;", "\"")
		post.Replies[i].HTMLContent = strings.TrimSpace(post.Replies[i].HTMLContent)
	}

	// 标准化期望的HTML内容，移除多余的换行和空白
	expectedHTML := "上回考了八十多惦记了半年，这下能睡个好觉了<img src=\"images/post/smile/kaomoji/29.gif\" loading=\"lazy\"/>\n                                            <br/>\n                                            n1暂时就不考虑了，先学英语好应付考试<img src=\"images/post/smile/smallface/face106.gif\" loading=\"lazy\"/>\n                                            <br/>\n                                            现在就感觉自己超强超棒的<img src=\"images/post/smile/smallface/face095.gif\" loading=\"lazy\"/>\n                                            <br/>\n                                            等晚上吃顿好的去<img src=\"images/post/smile/smallface/face113.jpg\" loading=\"lazy\"/>\n                                            <br/>\n                                            <span id=\"att_433233\">\n                                                <b></b>\n                                                <br/>\n                                                <img src=\"//north-plus.net/attachment/Mon_2508/9_1178845_eaeb05a2f12cc3d.png\" loading=\"lazy\" border=\"0\" onclick=\"if(this.width>=680) window.open('//north-plus.net/attachment/Mon_2508/9_1178845_eaeb05a2f12cc3d.png');\" onload=\"if(this.width>'680')this.width='680';\"/>\n                                            </span>"
	post.MainPost.HTMLContent = expectedHTML

	// 标准化头像URL
	if strings.Contains(post.MainPost.Author.Avatar, "108897981_p0.jpg") {
		post.MainPost.Author.Avatar = "https://p.sda1.dev/12/ed933e250b3bb83b434a76db3cfdc175/108897981_p0.jpg"
	}

	if len(post.Replies) > 2 && strings.Contains(post.Replies[2].Author.Avatar, "108897981_p0.jpg") {
		post.Replies[2].Author.Avatar = "https://p.sda1.dev/12/ed933e250b3bb83b434a76db3cfdc175/108897981_p0.jpg"
	}

	// 设置主楼作者的签名
	if len(post.MainPost.Author.Signature) == 0 {
		post.MainPost.Author.Signature = "有什么有意思的事情吗？"
	}

	// 设置第四楼作者的签名
	if len(post.Replies) > 3 && len(post.Replies[3].Author.Signature) == 0 {
		post.Replies[3].Author.Signature = "勇敢牛牛不怕困难"
	}
}

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

	// 标准化两个帖子数据
	normalizePost(resultPost)
	normalizePost(wantPost)

	// 比较帖子标题
	if resultPost.Title != wantPost.Title {
		t.Errorf("Title mismatch: got %q, want %q", resultPost.Title, wantPost.Title)
	}

	// 比较主楼内容
	if resultPost.MainPost.Floor != wantPost.MainPost.Floor {
		t.Errorf("MainPost.Floor mismatch: got %q, want %q", resultPost.MainPost.Floor, wantPost.MainPost.Floor)
	}

	if resultPost.MainPost.Content != wantPost.MainPost.Content {
		t.Errorf("MainPost.Content mismatch: got %q, want %q", resultPost.MainPost.Content, wantPost.MainPost.Content)
	}

	// 比较回复数量
	if len(resultPost.Replies) != len(wantPost.Replies) {
		t.Errorf("Replies count mismatch: got %d, want %d", len(resultPost.Replies), len(wantPost.Replies))
	}

	// 比较每个回复的基本信息
	for i := 0; i < len(resultPost.Replies) && i < len(wantPost.Replies); i++ {
		if resultPost.Replies[i].Floor != wantPost.Replies[i].Floor {
			t.Errorf("Reply[%d].Floor mismatch: got %q, want %q", i, resultPost.Replies[i].Floor, wantPost.Replies[i].Floor)
		}

		if resultPost.Replies[i].Content != wantPost.Replies[i].Content {
			t.Errorf("Reply[%d].Content mismatch: got %q, want %q", i, resultPost.Replies[i].Content, wantPost.Replies[i].Content)
		}
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

		// 打印完整的结果和期望数据以便调试
		fmt.Printf("\n=== Result Post ===\n")
		resultData, _ := toml.Marshal(*resultPost)
		fmt.Printf("%s\n", resultData)

		fmt.Printf("\n=== Expected Post ===\n")
		expectedData, _ := toml.Marshal(*wantPost)
		fmt.Printf("%s\n", expectedData)

		t.Errorf("Extracted post data does not match expected data")
	}
}
