package userdao

import (
	"context"
	"errors"
	"time"

	"vesko/auth"

	"gorm.io/gorm"
)

type UserModel struct {
	ID                uint    `gorm:"primaryKey;autoIncrement"`
	Username          string  `gorm:"size:255;uniqueIndex;not null"`
	Email             string  `gorm:"size:255;uniqueIndex;not null"`
	Mobile            *string `gorm:"size:20;uniqueIndex"`
	MobileVerified    bool    `gorm:"not null;default:false"`
	Role              string  `gorm:"size:20;not null;default:'customer'"`
	IsProfileComplete bool    `gorm:"not null;default:true"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         gorm.DeletedAt `gorm:"index"`
}

type PasswordModel struct {
	UserID       uint   `gorm:"primaryKey"`
	PasswordHash string `gorm:"size:255;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type SSOAccountModel struct {
	ID         uint   `gorm:"primaryKey;autoIncrement"`
	UserID     uint   `gorm:"index;not null"`
	Provider   string `gorm:"size:50;not null"`
	ProviderID string `gorm:"size:255;not null"`
	Email      string `gorm:"size:255;not null"`
	Data       []byte `gorm:"type:jsonb"`
	CreatedAt  time.Time
	UpdatedAt  time.Time

	// Unique constraint on provider + provider_id
	_ struct{} `gorm:"uniqueIndex:idx_provider_id,composite:provider,provider_id"`
}

type PostgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) AutoMigrate(ctx context.Context) error {
	return r.db.WithContext(ctx).AutoMigrate(&UserModel{}, &PasswordModel{}, &SSOAccountModel{})
}

func (r *PostgresRepository) GetUserDetailsByUsername(ctx context.Context, username string) (auth.User, error) {
	var model UserModel
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return auth.User{}, auth.ErrUserNotFound
	}
	if err != nil {
		return auth.User{}, err
	}

	return toDomainUser(model), nil
}

func (r *PostgresRepository) GetUserByEmail(ctx context.Context, email string) (auth.User, error) {
	var model UserModel
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return auth.User{}, auth.ErrUserNotFound
	}
	if err != nil {
		return auth.User{}, err
	}

	return toDomainUser(model), nil
}

func (r *PostgresRepository) GetUserByMobile(ctx context.Context, mobile string) (auth.User, error) {
	var model UserModel
	err := r.db.WithContext(ctx).Where("mobile = ?", mobile).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return auth.User{}, auth.ErrUserNotFound
	}
	if err != nil {
		return auth.User{}, err
	}
	return toDomainUser(model), nil
}

func (r *PostgresRepository) GetUserByID(ctx context.Context, id uint) (auth.User, error) {
	var model UserModel
	err := r.db.WithContext(ctx).First(&model, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return auth.User{}, auth.ErrUserNotFound
	}
	if err != nil {
		return auth.User{}, err
	}
	return toDomainUser(model), nil
}

func (r *PostgresRepository) RegisterUser(ctx context.Context, user auth.User, passwordHash string) (auth.User, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var mobile *string
		if user.Mobile != "" {
			m := user.Mobile
			mobile = &m
		}

		model := UserModel{
			Username:          user.Username,
			Email:             user.Email,
			Mobile:            mobile,
			MobileVerified:    user.MobileVerified,
			Role:              string(user.Role),
			IsProfileComplete: user.IsProfileComplete,
		}

		if err := tx.Create(&model).Error; err != nil {
			return err
		}

		if passwordHash != "" {
			password := PasswordModel{
				UserID:       model.ID,
				PasswordHash: passwordHash,
			}
			if err := tx.Create(&password).Error; err != nil {
				return err
			}
		}

		user = toDomainUser(model)
		return nil
	})

	if err == nil {
		return user, nil
	}

	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return auth.User{}, auth.ErrUserAlreadyExists
	}

	// Detailed error checking (could be improved with DB specific error codes)
	var existing UserModel
	switch {
	case r.db.WithContext(ctx).Where("username = ?", user.Username).First(&existing).Error == nil:
		return auth.User{}, auth.ErrUsernameAlreadyExists
	case r.db.WithContext(ctx).Where("email = ?", user.Email).First(&existing).Error == nil:
		return auth.User{}, auth.ErrEmailAlreadyExists
	case user.Mobile != "" && r.db.WithContext(ctx).Where("mobile = ?", user.Mobile).First(&existing).Error == nil:
		return auth.User{}, auth.ErrMobileAlreadyExists
	}

	return auth.User{}, err
}

func (r *PostgresRepository) UpdateUser(ctx context.Context, user auth.User) error {
	var mobile *string
	if user.Mobile != "" {
		m := user.Mobile
		mobile = &m
	}

	model := UserModel{
		ID:                user.ID,
		Username:          user.Username,
		Email:             user.Email,
		Mobile:            mobile,
		MobileVerified:    user.MobileVerified,
		Role:              string(user.Role),
		IsProfileComplete: user.IsProfileComplete,
	}

	return r.db.WithContext(ctx).Model(&model).Updates(map[string]any{
		"username":            model.Username,
		"email":               model.Email,
		"mobile":              model.Mobile,
		"mobile_verified":     model.MobileVerified,
		"role":                model.Role,
		"is_profile_complete": model.IsProfileComplete,
	}).Error
}

func (r *PostgresRepository) GetPasswordHashByUserID(ctx context.Context, userID uint) (string, error) {
	var model PasswordModel
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", errors.New("password not found")
	}
	return model.PasswordHash, err
}

func (r *PostgresRepository) GetSSOAccount(ctx context.Context, provider string, providerID string) (auth.User, bool, error) {
	var ssoModel SSOAccountModel
	err := r.db.WithContext(ctx).Where("provider = ? AND provider_id = ?", provider, providerID).First(&ssoModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return auth.User{}, false, nil
	}
	if err != nil {
		return auth.User{}, false, err
	}

	var userModel UserModel
	err = r.db.WithContext(ctx).First(&userModel, ssoModel.UserID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return auth.User{}, false, nil
	}
	if err != nil {
		return auth.User{}, true, err
	}

	return toDomainUser(userModel), true, nil
}

func (r *PostgresRepository) CreateSSOUser(ctx context.Context, user auth.User, account auth.SSOAccount) (auth.User, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var mobile *string
		if user.Mobile != "" {
			m := user.Mobile
			mobile = &m
		}

		userModel := UserModel{
			Username:          user.Username,
			Email:             user.Email,
			Mobile:            mobile,
			MobileVerified:    user.MobileVerified,
			Role:              string(user.Role),
			IsProfileComplete: user.IsProfileComplete,
		}

		if err := tx.Create(&userModel).Error; err != nil {
			return err
		}

		ssoModel := SSOAccountModel{
			UserID:     userModel.ID,
			Provider:   account.Provider,
			ProviderID: account.ProviderID,
			Email:      account.Email,
		}
		// We could also marshal account.Data if we had a proper JSON handler or if we use []byte
		// For now let's keep it simple.

		if err := tx.Create(&ssoModel).Error; err != nil {
			return err
		}

		user = toDomainUser(userModel)
		return nil
	})

	return user, err
}

func (r *PostgresRepository) LinkSSOAccount(ctx context.Context, userID uint, account auth.SSOAccount) error {
	ssoModel := SSOAccountModel{
		UserID:     userID,
		Provider:   account.Provider,
		ProviderID: account.ProviderID,
		Email:      account.Email,
	}
	return r.db.WithContext(ctx).Create(&ssoModel).Error
}

func toDomainUser(model UserModel) auth.User {
	mobile := ""
	if model.Mobile != nil {
		mobile = *model.Mobile
	}

	return auth.User{
		ID:                model.ID,
		Username:          model.Username,
		Email:             model.Email,
		Mobile:            mobile,
		MobileVerified:    model.MobileVerified,
		Role:              auth.Role(model.Role),
		IsProfileComplete: model.IsProfileComplete,
		CreatedAt:         model.CreatedAt,
		UpdatedAt:         model.UpdatedAt,
	}
}
