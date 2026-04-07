package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"elderly-fitness/config"

	"github.com/gin-gonic/gin"
	"github.com/tencentyun/cos-go-sdk-v5"
)

type UploadHandler struct {
	cosClient *cos.Client
	bucketURL string
}

func NewUploadHandler() *UploadHandler {
	cosConfig := config.AppConfig.COS
	bucketURL := cosConfig.BucketURL

	// 解析 bucket URL
	u, _ := url.Parse(bucketURL)
	b := &cos.BaseURL{BucketURL: u}

	client := cos.NewClient(b, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  cosConfig.SecretID,
			SecretKey: cosConfig.SecretKey,
		},
	})

	return &UploadHandler{
		cosClient: client,
		bucketURL: bucketURL,
	}
}

// UploadFile 上传文件
func (h *UploadHandler) UploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "请选择要上传的文件",
		})
		return
	}
	defer file.Close()

	// 获取文件类型（image/video）
	fileType := c.DefaultQuery("type", "image")

	// 验证文件类型
	ext := filepath.Ext(header.Filename)
	allowedImageExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
	allowedVideoExts := map[string]bool{".mp4": true, ".mov": true, ".avi": true, ".mkv": true}

	if fileType == "image" && !allowedImageExts[ext] {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "图片仅支持 jpg、png、gif、webp 格式",
		})
		return
	}
	if fileType == "video" && !allowedVideoExts[ext] {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "视频仅支持 mp4、mov、avi、mkv 格式",
		})
		return
	}

	// 生成文件路径：uploads/images/2024/01/01/xxx.jpg
	datePath := time.Now().Format("2006/01/02")
	fileName := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	objectKey := fmt.Sprintf("uploads/%ss/%s/%s", fileType, datePath, fileName)

	// 上传到COS
	_, err = h.cosClient.Object.Put(c, objectKey, file, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "上传失败：" + err.Error(),
		})
		return
	}

	// 返回完整URL（公有读模式，直接拼接URL）
	fileURL := fmt.Sprintf("%s/%s", h.bucketURL, objectKey)

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "上传成功",
		Data: gin.H{
			"url":      fileURL,
			"filename": header.Filename,
			"size":     header.Size,
		},
	})
}

// UploadImage 上传图片（简化接口）
func (h *UploadHandler) UploadImage(c *gin.Context) {
	c.Request.URL.RawQuery = "type=image"
	h.UploadFile(c)
}

// UploadVideo 上传视频（简化接口）
func (h *UploadHandler) UploadVideo(c *gin.Context) {
	c.Request.URL.RawQuery = "type=video"
	h.UploadFile(c)
}
