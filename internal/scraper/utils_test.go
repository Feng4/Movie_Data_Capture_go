package scraper

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// TestIsSameNumber 验证番号比对：容忍站点的合理规范化，但识破无关作品。
func TestIsSameNumber(t *testing.T) {
	tests := []struct {
		name      string
		requested string
		got       string
		want      bool
	}{
		// 应视为同一作品：站点常见的规范化差异
		{"完全一致", "MDX-0212", "MDX-0212", true},
		{"大小写不同", "mdx-0212", "MDX-0212", true},
		{"站点去掉横线", "MDX-0212", "MDX0212", true},
		{"站点补上横线", "MDX0212", "MDX-0212", true},
		{"下划线与横线互换", "MKY_TN_003", "MKY-TN-003", true},
		{"含空格", "MDX 0212", "MDX-0212", true},

		// 必须判定为不同作品——这是本次修复的核心场景
		{"完全无关的番号（xcity 返回推荐位）", "MD-0265", "UE195R", false},
		{"同前缀不同编号", "MDX-0212", "MDX-0213", false},
		{"编号相同前缀不同", "MDX-0212", "ABC-0212", false},
		{"请求为空", "", "UE195R", false},
		{"结果为空", "MD-0265", "", false},
		{"归一化后为空", "---", "MD-0265", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSameNumber(tt.requested, tt.got); got != tt.want {
				t.Errorf("isSameNumber(%q, %q) = %v, want %v",
					tt.requested, tt.got, got, tt.want)
			}
		})
	}
}

// TestExtractDate 验证从混杂文本中抽取日期。
// 输入取自实际日志中被污染的 release 字段。
func TestExtractDate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "日志中的实际污染文本",
			in:   "発売日\n\t\t\t2026-07-31\n\t\tメーカー品番UE195R",
			want: "2026-07-31",
		},
		{"斜杠分隔", "発売日 2021/11/14", "2021-11-14"},
		{"中文年月日", "发布时间：2021年1月3日", "2021-01-03"},
		{"单位数月日补零", "2021-1-3", "2021-01-03"},
		{"无日期返回空", "メーカー品番UE195R", ""},
		{"空输入", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractDate(tt.in); got != tt.want {
				t.Errorf("extractDate(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestNormalizeXCityURL 验证协议相对地址被补全
func TestNormalizeXCityURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "日志中的实际封面地址",
			in:   "//faws.xcity.jp/package-b/large/image/maker/leo/ue195r/b_1780041165_1.jpg",
			want: "https://faws.xcity.jp/package-b/large/image/maker/leo/ue195r/b_1780041165_1.jpg",
		},
		{"站内绝对路径", "/avod/detail/?id=209751", "https://xcity.jp/avod/detail/?id=209751"},
		{"已含协议保持原样", "https://faws.xcity.jp/a.jpg", "https://faws.xcity.jp/a.jpg"},
		{"空值", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeXCityURL(tt.in); got != tt.want {
				t.Errorf("normalizeXCityURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestFindXCityDetailURL 验证只选取番号相符的搜索结果。
// 模拟站点无请求番号时返回推荐位条目的情形。
func TestFindXCityDetailURL(t *testing.T) {
	const searchHTML = `<html><body>
		<div class="recommend">
			<a href="/avod/detail/?id=209751"><img alt="UE195R" src="/img/ue195r.jpg"></a>
			<a href="/avod/detail/?id=100001"><img alt="ABCD-123" src="/img/abcd123.jpg"></a>
		</div>
		<div class="result">
			<a href="/avod/detail/?id=555555"><img alt="MDX-0212" src="/img/mdx0212.jpg"></a>
		</div>
	</body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(searchHTML))
	if err != nil {
		t.Fatalf("failed to parse html: %v", err)
	}

	t.Run("命中相符番号而非首个链接", func(t *testing.T) {
		got := findXCityDetailURL(doc, "MDX-0212")
		want := "https://xcity.jp/avod/detail/?id=555555"
		if got != want {
			t.Errorf("findXCityDetailURL = %q, want %q", got, want)
		}
	})

	t.Run("番号不存在时返回空而非误取推荐位", func(t *testing.T) {
		// 这正是原 bug：请求 MD-0265，站点无此番号，
		// 旧实现会返回推荐位的 UE195R 详情页
		if got := findXCityDetailURL(doc, "MD-0265"); got != "" {
			t.Errorf("findXCityDetailURL = %q, want empty (no match)", got)
		}
	})

	t.Run("去横线形式也能命中", func(t *testing.T) {
		got := findXCityDetailURL(doc, "MDX0212")
		want := "https://xcity.jp/avod/detail/?id=555555"
		if got != want {
			t.Errorf("findXCityDetailURL = %q, want %q", got, want)
		}
	})

	t.Run("空番号返回空", func(t *testing.T) {
		if got := findXCityDetailURL(doc, ""); got != "" {
			t.Errorf("findXCityDetailURL = %q, want empty", got)
		}
	})
}
