package repository

import (
	"lab-teaching/backend/internal/model"
)

// ============ V1.x-21 系统设置 ============
func (r *Repo) GetSetting(key string) (string, bool) {
	var s model.SystemSetting
	if err := r.DB.Where("key = ?", key).First(&s).Error; err != nil {
		return "", false
	}
	return s.Value, true
}

func (r *Repo) UpsertSetting(key, value string) error {
	var s model.SystemSetting
	if err := r.DB.Where("key = ?", key).First(&s).Error; err != nil {
		return r.DB.Create(&model.SystemSetting{Key: key, Value: value}).Error
	}
	return r.DB.Model(&s).Update("value", value).Error
}

func (r *Repo) ListSettings() ([]model.SystemSetting, error) {
	var list []model.SystemSetting
	err := r.DB.Order("key").Find(&list).Error
	return list, err
}

// ============ V1.x-21 用户分组 ============
func (r *Repo) CreateGroup(g *model.UserGroup) error { return r.DB.Create(g).Error }

func (r *Repo) ListGroups() ([]model.GroupMemberCount, error) {
	var groups []model.UserGroup
	if err := r.DB.Order("id").Find(&groups).Error; err != nil {
		return nil, err
	}
	out := make([]model.GroupMemberCount, 0, len(groups))
	for _, g := range groups {
		var n int64
		r.DB.Model(&model.User{}).Where("group_id = ?", g.ID).Count(&n)
		out = append(out, model.GroupMemberCount{UserGroup: g, MemberCount: n})
	}
	return out, nil
}

func (r *Repo) GetGroup(id uint) (*model.UserGroup, error) {
	var g model.UserGroup
	err := r.DB.First(&g, id).Error
	return &g, err
}

func (r *Repo) UpdateGroup(g *model.UserGroup) error { return r.DB.Save(g).Error }

func (r *Repo) DeleteGroup(id uint) error {
	// 先解除用户关联，再删除分组
	if err := r.DB.Model(&model.User{}).Where("group_id = ?", id).Update("group_id", 0).Error; err != nil {
		return err
	}
	return r.DB.Delete(&model.UserGroup{}, id).Error
}

// ============ V1.x-21 用户管理 ============
func (r *Repo) ListUsers() ([]model.User, error) {
	var users []model.User
	err := r.DB.Order("id").Find(&users).Error
	return users, err
}

func (r *Repo) GetUser(id uint) (*model.User, error) {
	var u model.User
	err := r.DB.First(&u, id).Error
	return &u, err
}

func (r *Repo) UpdateUser(u *model.User) error { return r.DB.Save(u).Error }

func (r *Repo) DeleteUser(id uint) error { return r.DB.Delete(&model.User{}, id).Error }
