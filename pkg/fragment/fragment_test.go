package fragment

import (
	"testing"
)

func TestFragmentManager_IsFragmentFile(t *testing.T) {
	fm := NewFragmentManager()
	
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"CD format lowercase", "movie-cd1.mp4", true},
		{"CD format uppercase", "MOVIE-CD2.AVI", true},
		{"CD format underscore", "movie_cd3.mkv", true},
		{"PART format", "movie-part1.mp4", true},
		{"DISC format", "movie-disc2.mp4", true},
		{"Simple number", "movie-1.mp4", true},
		{"Simple number underscore", "movie_2.mp4", true},
		{"Not fragment", "movie.mp4", false},
		{"Not fragment with number in name", "movie2021.mp4", false},
		{"Complex name with CD", "ABC-123-cd1.mp4", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fm.IsFragmentFile(tt.filename); got != tt.want {
				t.Errorf("IsFragmentFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFragmentManager_ParseFragmentInfo(t *testing.T) {
	fm := NewFragmentManager()
	
	tests := []struct {
		name         string
		filePath     string
		wantBaseName string
		wantPartNum  int
		wantSuffix   string
	}{
		{
			name:         "CD format",
			filePath:     "/path/to/ABC-123-cd1.mp4",
			wantBaseName: "ABC-123",
			wantPartNum:  1,
			wantSuffix:   "-cd1",
		},
		{
			name:         "CD format uppercase",
			filePath:     "/path/to/MOVIE-CD2.AVI",
			wantBaseName: "MOVIE",
			wantPartNum:  2,
			wantSuffix:   "-CD2",
		},
		{
			name:         "PART format",
			filePath:     "/path/to/movie-part3.mkv",
			wantBaseName: "movie",
			wantPartNum:  3,
			wantSuffix:   "-part3",
		},
		{
			name:         "Simple number",
			filePath:     "/path/to/movie-1.mp4",
			wantBaseName: "movie",
			wantPartNum:  1,
			wantSuffix:   "-1",
		},
		{
			name:         "Not fragment",
			filePath:     "/path/to/movie.mp4",
			wantBaseName: "movie",
			wantPartNum:  0,
			wantSuffix:   "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := fm.ParseFragmentInfo(tt.filePath)
			if err != nil {
				t.Errorf("ParseFragmentInfo() error = %v", err)
				return
			}
			
			if info.BaseName != tt.wantBaseName {
				t.Errorf("BaseName = %v, want %v", info.BaseName, tt.wantBaseName)
			}
			if info.PartNumber != tt.wantPartNum {
				t.Errorf("PartNumber = %v, want %v", info.PartNumber, tt.wantPartNum)
			}
			if info.PartSuffix != tt.wantSuffix {
				t.Errorf("PartSuffix = %v, want %v", info.PartSuffix, tt.wantSuffix)
			}
		})
	}
}

func TestFragmentManager_GroupFragmentFiles(t *testing.T) {
	fm := NewFragmentManager()
	
	filePaths := []string{
		"/path/to/ABC-123-cd1.mp4",
		"/path/to/ABC-123-cd2.mp4",
		"/path/to/ABC-123-cd3.mp4",
		"/path/to/XYZ-456.mp4",           // 非分片文件
		"/path/to/DEF-789-part1.mkv",
		"/path/to/DEF-789-part2.mkv",
		"/path/to/single-movie.avi",      // 非分片文件
	}
	
	fragmentGroups, nonFragmentFiles := fm.GroupFragmentFiles(filePaths)
	
	// 检查分片组数量
	if len(fragmentGroups) != 2 {
		t.Errorf("Expected 2 fragment groups, got %d", len(fragmentGroups))
	}
	
	// 检查非分片文件数量
	if len(nonFragmentFiles) != 2 {
		t.Errorf("Expected 2 non-fragment files, got %d", len(nonFragmentFiles))
	}
	
	// 检查第一个分片组
	for _, group := range fragmentGroups {
		if group.BaseName == "abc-123.mp4" {
			if len(group.Fragments) != 3 {
				t.Errorf("Expected 3 fragments in ABC-123 group, got %d", len(group.Fragments))
			}
			if group.MainFile != "/path/to/ABC-123-cd1.mp4" {
				t.Errorf("Expected main file to be cd1, got %s", group.MainFile)
			}
		} else if group.BaseName == "def-789.mkv" {
			if len(group.Fragments) != 2 {
				t.Errorf("Expected 2 fragments in DEF-789 group, got %d", len(group.Fragments))
			}
		}
	}
}

func TestFragmentGroup_HasMissingParts(t *testing.T) {
	tests := []struct {
		name      string
		fragments []FragmentInfo
		want      bool
	}{
		{
			name: "Complete sequence",
			fragments: []FragmentInfo{
				{PartNumber: 1},
				{PartNumber: 2},
				{PartNumber: 3},
			},
			want: false,
		},
		{
			name: "Missing part 2",
			fragments: []FragmentInfo{
				{PartNumber: 1},
				{PartNumber: 3},
			},
			want: true,
		},
		{
			name: "Starts from 2",
			fragments: []FragmentInfo{
				{PartNumber: 2},
				{PartNumber: 3},
			},
			want: true,
		},
		{
			name:      "Empty",
			fragments: []FragmentInfo{},
			want:      false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := &FragmentGroup{Fragments: tt.fragments}
			if got := group.HasMissingParts(); got != tt.want {
				t.Errorf("HasMissingParts() = %v, want %v", got, tt.want)
			}
		})
	}
}