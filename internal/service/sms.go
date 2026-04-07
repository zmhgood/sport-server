package service

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"elderly-fitness/config"
	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"

	"gorm.io/gorm"
)

type SMSService struct {
	smsCodeRepo *repository.SMSCodeRepository
}

func NewSMSService(smsCodeRepo *repository.SMSCodeRepository) *SMSService {
	return &SMSService{
		smsCodeRepo: smsCodeRepo,
	}
}

// SendCode 发送验证码
func (s *SMSService) SendCode(phone, purpose string) error {
	// 检查发送频率限制（1分钟内只能发1次）
	lastCode, err := s.smsCodeRepo.FindLatestByPhone(phone, purpose)
	if err == nil && lastCode != nil {
		if time.Since(lastCode.CreatedAt) < time.Minute {
			return errors.New("发送太频繁，请1分钟后再试")
		}
	}

	// 生成6位验证码
	code := s.generateCode()

	// 创建验证码记录
	smsCode := &model.SMSCode{
		Phone:     phone,
		Code:      code,
		Purpose:   purpose,
		ExpiresAt: time.Now().Add(time.Duration(config.AppConfig.SMS.ExpireTime) * time.Minute),
	}

	if err := s.smsCodeRepo.Create(smsCode); err != nil {
		return errors.New("创建验证码失败")
	}

	// 发送短信（实际项目中接入短信服务商）
	if err := s.sendSMS(phone, code); err != nil {
		return errors.New("发送短信失败: " + err.Error())
	}

	return nil
}

// VerifyCode 验证验证码
func (s *SMSService) VerifyCode(phone, code, purpose string) (bool, error) {
	smsCode, err := s.smsCodeRepo.FindLatestByPhone(phone, purpose)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, errors.New("验证码不存在")
		}
		return false, err
	}

	if !smsCode.IsValid() {
		return false, errors.New("验证码已过期或已使用")
	}

	if smsCode.Code != code {
		return false, errors.New("验证码错误")
	}

	// 标记为已使用
	smsCode.Used = true
	if err := s.smsCodeRepo.Update(smsCode); err != nil {
		return false, errors.New("更新验证码状态失败")
	}

	return true, nil
}

// generateCode 生成6位验证码
func (s *SMSService) generateCode() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("%06d", r.Intn(1000000))
}

// sendSMS 发送短信（实际项目中接入短信服务商API）
func (s *SMSService) sendSMS(phone, code string) error {
	// 开发环境：打印验证码到控制台
	fmt.Printf("\n========================================\n")
	fmt.Printf("📱 短信验证码\n")
	fmt.Printf("手机号: %s\n", phone)
	fmt.Printf("验证码: %s\n", code)
	fmt.Printf("有效期: %d 分钟\n", config.AppConfig.SMS.ExpireTime)
	fmt.Printf("========================================\n\n")

	// TODO: 生产环境接入短信服务商API
	// if config.AppConfig.SMS.Provider == "tencent" {
	//     return s.sendTencentSMS(phone, code)
	// } else if config.AppConfig.SMS.Provider == "aliyun" {
	//     return s.sendAliyunSMS(phone, code)
	// }

	return nil
}

// sendTencentSMS 发送腾讯云短信
// func (s *SMSService) sendTencentSMS(phone, code string) error {
//     // 实现腾讯云短信API调用
//     return nil
// }

// sendAliyunSMS 发送阿里云短信
// func (s *SMSService) sendAliyunSMS(phone, code string) error {
//     // 实现阿里云短信API调用
//     return nil
// }
