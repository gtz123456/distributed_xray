package db

func Sync() {
	DB.AutoMigrate(&User{})
}
