package user

type User struct {
	ID           string `gorm:"primaryKey;type:text" json:"id"`
	Email        string `gorm:"uniqueIndex;not null;type:text" json:"email"`
	Username     string `gorm:"uniqueIndex;not null;type:text" json:"username"`
	DisplayName  string `gorm:"type:text;default:''" json:"display_name"`
	AvatarURL    string `gorm:"type:text;default:''" json:"avatar_url"`
	PasswordHash string `gorm:"type:text;default:''" json:"-"`
	IsAdmin      bool   `gorm:"default:false" json:"is_admin"`
	IsActive     bool   `gorm:"default:true" json:"is_active"`
	TimeZone     string `gorm:"column:time_zone;default:''" json:"time_zone"`
	Locale       string `gorm:"column:locale;default:''" json:"locale"`
}
