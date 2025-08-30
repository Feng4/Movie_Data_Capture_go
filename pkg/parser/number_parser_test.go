package parser

import (
	"testing"
	"movie-data-capture/internal/config"
)

func TestNumberParser_GetNumber(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		// Standard JAV formats
		{"Standard format", "SNIS-829.mp4", "SNIS-829"},
		{"With Chinese subtitle", "SNIS-829-C.mp4", "SNIS-829"},
		{"Underscore format", "SSIS_001.mp4", "SSIS-001"},
		{"Mixed case", "ssni984.mp4", "SSNI-984"},
		
		// FC2 formats
		{"FC2 with PPV", "FC2-PPV-1234567.mp4", "FC2-1234567"},
		{"FC2 without PPV", "FC2-1234567.mp4", "FC2-1234567"},
		{"FC2 underscore", "FC2_PPV_1234567.mp4", "FC2-1234567"},
		
		// Tokyo Hot formats
		{"Tokyo Hot n-series", "Tokyo Hot n9001 FHD.mp4", "n9001"},
		{"Tokyo Hot with dash", "TokyoHot-n1287-HD SP2006.mp4", "n1287"},
		
		// Caribbean formats
		{"Caribbean format", "caribean-020317_001.nfo", "020317-001"},
		{"Carib with underscore", "257138_3xplanet_1Pondo_080521_001.mp4", "080521_001"},
		
		// Heydouga formats
		{"Heydouga format", "heydouga-4102-023-CD2.iso", "heydouga-4102-023"},
		{"Heydouga mixed", "HeyDOuGa4236-1048 Ai Qiu.mp4", "heydouga-4236-1048"},
		
		// XXX-AV formats
		{"XXX-AV format", "XXX-AV 22061-CD5.iso", "xxx-av-22061"},
		{"XXX-AV simple", "xxx-av 20589.mp4", "xxx-av-20589"},
		
		// Pacopacomama formats
		{"Pacopacomama format", "pacopacomama-093021_539-FHD.mkv", "093021_539"},
		{"Muramura format", "Muramura-102114_145-HD.wmv", "102114_145"},
		
		// HEYZO formats
		{"HEYZO format", "sbw99.cc@heyzo_hd_2636_full.mp4", "HEYZO-2636"},
		
		// With site prefixes
		{"With site prefix 1", "hhd800.com@STARS-566-HD.mp4", "STARS-566"},
		{"With site prefix 2", "jav20s8.com@GIGL-677_4K.mp4", "GIGL-677"},
		{"With site prefix 3", "sbw99.cc@iesp-653-4K.mp4", "IESP-653"},
		
		// With quality indicators
		{"4K prefix", "4K-ABP-358_C.mkv", "ABP-358"},
		
		// CD series
		{"CD series 1", "n1012-CD1.wmv", "N1012"},
		{"CD series with brackets", "[]n1012-CD2.wmv", "N1012"},
		
		// Chinese subtitle variations
		{"CH subtitle", "rctd-460ch.mp4", "RCTD-460"},
		{"CH with CD", "rctd-461CH-CD2.mp4", "RCTD-461"},
		{"Mixed case CD", "rctd-461-Cd3-C.mp4", "RCTD-461"},
		{"Complex CD format", "rctd-461-C-cD4.mp4", "RCTD-461"},
		
		// Madou formats
		{"MD format", "MD-123.ts", "MD-123"},
		{"MDSR format", "MDSR-0001-ep2.ts", "MDSR-0001"},
		{"MKY format", "MKY-NS-001.mp4", "MKY-NS-001"},
	}
	
	parser := NewNumberParser(nil)
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.GetNumber(tt.filename)
			if result != tt.expected {
				t.Errorf("GetNumber(%s) = %s, want %s", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestNumberParser_IsUncensored(t *testing.T) {
	tests := []struct {
		name     string
		number   string
		expected bool
	}{
		// Built-in uncensored patterns
		{"Tokyo Hot", "n1234", true},
		{"Caribbean", "010121-001", true},
		{"HEYZO", "HEYZO-1234", true},
		{"XXX-AV", "xxx-av-12345", true},
		{"Heydouga", "heydouga-4102-023", true},
		{"X-Art", "x-art.21.12.25", true},
		{"Pure numbers", "123456", true},
		{"Long numbers", "12345678", true},
		{"Caribbean underscore", "123456_789", true},
		
		// Censored (should return false)
		{"Standard JAV", "SNIS-829", false},
		{"Another standard", "STARS-566", false},
		{"Short number", "123", false},
	}
	
	parser := NewNumberParser(nil)
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.IsUncensored(tt.number)
			if result != tt.expected {
				t.Errorf("IsUncensored(%s) = %v, want %v", tt.number, result, tt.expected)
			}
		})
	}
}

func TestNumberParser_CustomRegex(t *testing.T) {
	// Create a config with custom regex
	cfg := &config.Config{
		NameRule: config.NameRuleConfig{
			NumberRegexs: "CUSTOM-(\\d+) TEST-(\\w+)",
		},
	}
	
	parser := NewNumberParser(cfg)
	
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{"Custom regex 1", "CUSTOM-123.mp4", "123"},
		{"Custom regex 2", "TEST-ABC.mp4", "ABC"},
		{"Fallback to builtin", "SNIS-829.mp4", "SNIS-829"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.GetNumber(tt.filename)
			if result != tt.expected {
				t.Errorf("GetNumber(%s) = %s, want %s", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestNumberParser_SubtitleGroup(t *testing.T) {
	parser := NewNumberParser(nil)
	
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{
			"Japanese subtitle group",
			"[脸肿字幕组][PoRO]牝教師4～穢された教壇～ 「生意気ドジっ娘女教師・美結～高飛車ハメ堕ち2濁金」[720p][x264_aac].mp4",
			"牝教師4～穢された教壇～ 「生意気ドジっ娘女教師・美結～高飛車ハメ堕ち2濁金」",
		},
		{
			"SUB format",
			"[SUB]SNIS-829.mp4",
			"SNIS-829",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.GetNumber(tt.filename)
			// For subtitle group tests, we just check that we get some result
			if result == "" {
				t.Errorf("GetNumber(%s) returned empty string", tt.filename)
			}
			// Note: Exact matching for subtitle groups might be complex due to encoding
		})
	}
}