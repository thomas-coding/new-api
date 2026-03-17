package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
)

const ManagedDemoUserPassword = "Codexshare123!"

func EnsureManagedDefaults() error {
	if !constant.Setup || !RootUserExists() {
		common.SysLog("skip managed demo user bootstrap before setup completes")
		return nil
	}
	if err := ensureManagedBootstrapOption("RegisterEnabled", "false"); err != nil {
		return err
	}
	if err := ensureManagedBootstrapOption("PasswordRegisterEnabled", "false"); err != nil {
		return err
	}
	for i := 1; i <= 10; i++ {
		if err := ensureManagedDemoUser(fmt.Sprintf("codexshare%04d", i)); err != nil {
			return err
		}
	}
	return nil
}

func ensureManagedBootstrapOption(key string, value string) error {
	var option Option
	err := DB.First(&option, "key = ?", key).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return UpdateOption(key, value)
}

func ensureManagedDemoUser(username string) error {
	var existing User
	err := DB.Where("username = ?", username).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	user := &User{
		Username:    username,
		Password:    ManagedDemoUserPassword,
		DisplayName: username,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		Remark:      "系统预置演示账号",
	}
	if err := user.Insert(0); err != nil {
		return err
	}
	targetQuota := int(100 * common.QuotaPerUnit)
	if err := DB.Model(&User{}).Where("id = ?", user.Id).Update("quota", targetQuota).Error; err != nil {
		return err
	}
	common.SysLog(fmt.Sprintf("managed demo user ensured: %s", username))
	return nil
}
