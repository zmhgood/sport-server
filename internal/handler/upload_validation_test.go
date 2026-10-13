package handler

import (
	"bytes"
	"testing"
)

func TestUploadValidation(t *testing.T) {
	gif := []byte("GIF89a synthetic content")
	for _, tc := range []struct {
		name      string
		data      []byte
		size      int64
		kind, ext string
		ok        bool
	}{
		{"gif", gif, int64(len(gif)), "image", ".gif", true},
		{"renamed text", []byte("hello"), 5, "image", ".jpg", false},
		{"empty", nil, 0, "image", ".png", false},
		{"over image limit", gif, 5<<20 + 1, "image", ".gif", false},
		{"at image limit", gif, 5 << 20, "image", ".gif", true},
		{"over video limit", gif, 100<<20 + 1, "video", ".mp4", false},
		{"wrong type", gif, 10, "other", ".gif", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := bytes.NewReader(tc.data)
			err := validateUpload(file, tc.size, tc.kind, tc.ext)
			if (err == nil) != tc.ok {
				t.Fatal("unexpected validation result")
			}
			position, _ := file.Seek(0, 1)
			if tc.ok && position != 0 {
				t.Fatal("stream not rewound")
			}
		})
	}
}
