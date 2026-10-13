package handler

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
)

func validateUpload(file io.ReadSeeker, size int64, kind, ext string) error {
	limit := int64(5 << 20)
	if kind == "video" {
		limit = 100 << 20
	}
	if kind != "image" && kind != "video" {
		return errors.New("文件类型无效")
	}
	if size <= 0 || size > limit {
		return errors.New("文件为空或超过大小限制")
	}
	data := make([]byte, 512)
	n, err := file.Read(data)
	if err != nil && err != io.EOF {
		return errors.New("无法读取文件")
	}
	data = data[:n]
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return errors.New("无法读取文件")
	}
	mime := http.DetectContentType(data)
	valid := false
	if kind == "image" {
		expected := map[string]string{".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".gif": "image/gif", ".webp": "image/webp"}
		valid = expected[ext] != "" && mime == expected[ext]
	} else {
		switch ext {
		case ".mp4", ".mov":
			valid = len(data) >= 12 && string(data[4:8]) == "ftyp"
		case ".avi":
			valid = len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "AVI "
		case ".mkv":
			valid = bytes.HasPrefix(data, []byte{0x1a, 0x45, 0xdf, 0xa3})
		}
	}
	if !valid || strings.HasPrefix(mime, "text/") {
		return errors.New("文件内容与扩展名不一致")
	}
	return nil
}
