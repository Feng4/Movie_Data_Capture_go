package scraper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// loadMadouQuFixture 从 testdata 载入离线 HTML 样本
func loadMadouQuFixture(t *testing.T, name string) *goquery.Document {
	t.Helper()

	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("failed to open fixture %s: %v", name, err)
	}
	defer f.Close()

	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		t.Fatalf("failed to parse fixture %s: %v", name, err)
	}
	return doc
}

// assertStringsEqual 比较字符串切片，逐项报告差异
func assertStringsEqual(t *testing.T, label string, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s = %v (len %d), want %v (len %d)", label, got, len(got), want, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s[%d] = %q, want %q", label, i, got[i], want[i])
		}
	}
}

func TestMadouQuNumberVariants(t *testing.T) {
	tests := []struct {
		name   string
		number string
		want   []string
	}{
		{
			name:   "带横线番号生成去横线变体",
			number: "MDX-0212",
			want:   []string{"MDX-0212", "MDX0212"},
		},
		{
			name:   "无横线番号生成加横线变体",
			number: "MDX0212",
			want:   []string{"MDX0212", "MDX-0212"},
		},
		{
			name:   "多段番号去除所有分隔符后再补横线",
			number: "MKY-TN-003",
			want:   []string{"MKY-TN-003", "MKYTN003", "MKYTN-003"},
		},
		{
			name:   "纯空白返回空",
			number: "   ",
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertStringsEqual(t, "variants", madouquNumberVariants(tt.number), tt.want)
		})
	}
}

func TestMadouQuFieldValue(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		labels []string
		want   string
	}{
		{"繁体标签全角冒号", "麻豆番號：MDX-0212", madouquFieldPatterns.number, "MDX-0212"},
		{"简体标签", "番号：MD-0269", madouquFieldPatterns.number, "MD-0269"},
		{"半角冒号带空格", "片名: 虞姬嘆", madouquFieldPatterns.title, "虞姬嘆"},
		{"标签同行带值", "麻豆女郎：倪哇哇", madouquFieldPatterns.actor, "倪哇哇"},
		{"无冒号不匹配", "麻豆女郎", madouquFieldPatterns.actor, ""},
		{"标签不存在", "下載地址：Magnet", madouquFieldPatterns.number, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := madouquFieldValue(tt.line, tt.labels); got != tt.want {
				t.Errorf("madouquFieldValue(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestSplitMadouQuActors(t *testing.T) {
	tests := []struct {
		name  string
		actor string
		want  []string
	}{
		{"单个演员", "倪哇哇", []string{"倪哇哇"}},
		{"顿号分隔", "梁佳芯、唐芯", []string{"梁佳芯", "唐芯"}},
		{"半角逗号分隔", "袁子仪,杨柳", []string{"袁子仪", "杨柳"}},
		{"混合分隔并去重", "沈娜娜 / 沈娜娜、林小雨", []string{"沈娜娜", "林小雨"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertStringsEqual(t, "actors", splitMadouQuActors(tt.actor), tt.want)
		})
	}
}

// TestParseMadouQuDetail 用真实详情页样本验证字段提取。
// 样本页面的正文形如：
//
//	<p>麻豆番號：MDX-0212</p>
//	<p>麻豆片名：虞姬嘆</p>
//	<p><a>麻豆女郎</a>：倪哇哇</p>
func TestParseMadouQuDetail(t *testing.T) {
	doc := loadMadouQuFixture(t, "madouqu_detail.html")
	const detailURL = "https://madouqu.com/video/mdx0212/"

	data := parseMadouQuDetail(doc, detailURL, "MDX-0212")

	if data.Number != "MDX-0212" {
		t.Errorf("Number = %q, want %q", data.Number, "MDX-0212")
	}
	if data.Title != "虞姬嘆" {
		t.Errorf("Title = %q, want %q", data.Title, "虞姬嘆")
	}
	// 「麻豆女郎」是 <a> 文本、值紧随其后，goquery 的 Text() 会拼成一行
	if data.Actor != "倪哇哇" {
		t.Errorf("Actor = %q, want %q", data.Actor, "倪哇哇")
	}
	assertStringsEqual(t, "ActorList", data.ActorList, []string{"倪哇哇"})

	if data.Release != "2021-11-14" {
		t.Errorf("Release = %q, want %q", data.Release, "2021-11-14")
	}
	if data.Year != "2021" {
		t.Errorf("Year = %q, want %q", data.Year, "2021")
	}
	if !strings.HasPrefix(data.Cover, "https://") {
		t.Errorf("Cover = %q, want an absolute https URL", data.Cover)
	}
	// 懒加载占位图是 data:image/gif;base64，不得被当成封面
	if strings.HasPrefix(data.Cover, "data:") {
		t.Errorf("Cover picked up a lazy-load placeholder: %q", data.Cover)
	}
	if data.Website != detailURL {
		t.Errorf("Website = %q, want %q", data.Website, detailURL)
	}
	if data.Source != "madouqu" {
		t.Errorf("Source = %q, want %q", data.Source, "madouqu")
	}
}

// TestParseMadouQuDetailFallbacks 验证正文缺字段时的兜底路径
func TestParseMadouQuDetailFallbacks(t *testing.T) {
	t.Run("缺番号时回落到请求番号并从标题剥离", func(t *testing.T) {
		const html = `<html><body>
			<div class="cao_entry_header"><header><h1>ABC-123 测试片名</h1></header></div>
			<div class="entry-content"><p>正文没有番号标签</p></div>
		</body></html>`

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
		if err != nil {
			t.Fatalf("failed to parse html: %v", err)
		}

		data := parseMadouQuDetail(doc, "https://madouqu.com/video/abc123/", "ABC-123")
		if data.Number != "ABC-123" {
			t.Errorf("Number = %q, want fallback %q", data.Number, "ABC-123")
		}
		if data.Title != "测试片名" {
			t.Errorf("Title = %q, want number stripped from header title", data.Title)
		}
	})

	t.Run("标签与值分处两个段落时跨行提取", func(t *testing.T) {
		const html = `<html><body>
			<div class="entry-content">
				<p>麻豆番號：XYZ-001</p>
				<p>麻豆女郎</p>
				<p>：苏语棠、楚梦舒</p>
			</div>
		</body></html>`

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
		if err != nil {
			t.Fatalf("failed to parse html: %v", err)
		}

		data := parseMadouQuDetail(doc, "https://madouqu.com/video/xyz001/", "XYZ-001")
		assertStringsEqual(t, "ActorList", data.ActorList, []string{"苏语棠", "楚梦舒"})
	})

	t.Run("无番号无标题时不产生垃圾数据", func(t *testing.T) {
		const html = `<html><body><div class="entry-content"><p>空</p></div></body></html>`

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
		if err != nil {
			t.Fatalf("failed to parse html: %v", err)
		}

		data := parseMadouQuDetail(doc, "https://madouqu.com/video/none/", "")
		if data.Number != "" || data.Title != "" {
			t.Errorf("expected empty Number/Title, got Number=%q Title=%q", data.Number, data.Title)
		}
	})
}

// TestMatchMadouQuResult 用真实搜索页样本验证结果匹配
func TestMatchMadouQuResult(t *testing.T) {
	doc := loadMadouQuFixture(t, "madouqu_search.html")
	const wantURL = "https://madouqu.com/video/mdx0212/"

	t.Run("命中带横线番号", func(t *testing.T) {
		gotURL, title, ok := matchMadouQuResult(doc, madouquNumberVariants("MDX-0212"))
		if !ok {
			t.Fatal("expected a match for MDX-0212")
		}
		if gotURL != wantURL {
			t.Errorf("url = %q, want %q", gotURL, wantURL)
		}
		if !strings.Contains(title, "MDX0212") {
			t.Errorf("title = %q, want it to contain MDX0212", title)
		}
	})

	t.Run("命中无横线番号", func(t *testing.T) {
		gotURL, _, ok := matchMadouQuResult(doc, madouquNumberVariants("MDX0212"))
		if !ok {
			t.Fatal("expected a match for MDX0212")
		}
		if gotURL != wantURL {
			t.Errorf("url = %q, want %q", gotURL, wantURL)
		}
	})

	t.Run("不存在的番号不匹配", func(t *testing.T) {
		if _, _, ok := matchMadouQuResult(doc, madouquNumberVariants("ZZZ-9999")); ok {
			t.Error("expected no match for ZZZ-9999")
		}
	})
}

func TestNormalizeForMatch(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"MDX-0212", "MDX0212"},
		{"mdx_0212", "MDX0212"},
		// 中文不属于 \w，会被一并剥离，故标题可直接与番号比对
		{"MDX0212 虞姬嘆", "MDX0212"},
		{"https://madouqu.com/video/mdx0212/", "HTTPSMADOUQUCOMVIDEOMDX0212"},
	}

	for _, tt := range tests {
		if got := normalizeForMatch(tt.in); got != tt.want {
			t.Errorf("normalizeForMatch(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMadouQuImageSrc(t *testing.T) {
	const html = `<html><body>
		<img id="lazy" src="data:image/gif;base64,R0lGODlhAQABAAAAACH5BAEKAAEALAAAAAABAAEAAAICTAEAOw=="
		     data-src="https://i0.wp.com/madouqu.com/real.jpg">
		<img id="plain" src="https://i0.wp.com/madouqu.com/plain.jpg">
		<img id="empty" src="">
	</body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("failed to parse html: %v", err)
	}

	tests := []struct {
		sel  string
		want string
	}{
		{"#lazy", "https://i0.wp.com/madouqu.com/real.jpg"},
		{"#plain", "https://i0.wp.com/madouqu.com/plain.jpg"},
		{"#empty", ""},
	}

	for _, tt := range tests {
		if got := madouquImageSrc(doc.Find(tt.sel)); got != tt.want {
			t.Errorf("madouquImageSrc(%s) = %q, want %q", tt.sel, got, tt.want)
		}
	}
}
