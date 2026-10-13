package service

import (
	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type syntheticTransport func(*http.Request) (*http.Response, error)

func (f syntheticTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWeChatFailureAndConcurrentAccountCreation(t *testing.T) {
	f := newFixture(t)
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	service := NewAuthService(repository.NewUserRepository(f.db), nil)
	for _, body := range []string{`{"errcode":40029,"errmsg":"invalid synthetic code"}`, `{}`, `invalid JSON`} {
		http.DefaultTransport = syntheticTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})
		if _, err := service.WeChatLogin("synthetic-code"); err == nil {
			t.Fatal("invalid response accepted")
		}
	}
	http.DefaultTransport = syntheticTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New("synthetic unavailable") })
	if _, err := service.WeChatLogin("synthetic-code"); err == nil {
		t.Fatal("transport failure accepted")
	}
	openID := fmt.Sprintf("synthetic-login-%d", time.Now().UnixNano())
	http.DefaultTransport = syntheticTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"openid":%q}`, openID))), Header: make(http.Header)}, nil
	})
	var wg sync.WaitGroup
	ids := make(chan uint, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := service.WeChatLogin("synthetic-code")
			if err != nil {
				t.Error(err)
				return
			}
			ids <- result.UserInfo.ID
		}()
	}
	wg.Wait()
	close(ids)
	var id uint
	for actual := range ids {
		if id != 0 && actual != id {
			t.Fatal("duplicate account")
		}
		id = actual
	}
	var count int64
	if err := f.db.Model(&model.User{}).Where("openid=?", openID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("accounts=%d error=%v", count, err)
	}
	if err := f.db.Model(&model.User{}).Where("id=?", id).Update("nick_name", "Existing nickname").Error; err != nil {
		t.Fatal(err)
	}
	result, err := service.WeChatLogin("synthetic-code")
	if err != nil || result.UserInfo.NickName != "Existing nickname" {
		t.Fatal("existing profile lost")
	}
	sqlDB, _ := f.db.DB()
	sqlDB.Close()
	if _, err = service.WeChatLogin("synthetic-code"); err == nil || !strings.Contains(err.Error(), "读取失败") {
		t.Fatal("database error treated as new account")
	}
}
