package db

func Sync() {
	DB.AutoMigrate(&Order{})
}
