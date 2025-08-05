package lib

import (
	"encoding/json"
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type DBConfig struct {
	User     string
	Password string
	Addr     string
	DBName   string
}

func (dbConfig DBConfig) getDSN() string {
	// dsn := "user:password@tcp(127.0.0.1:3306)/test_db?charset=utf8mb4&parseTime=True&loc=Local"

	return fmt.Sprintf("%v:%v@tcp(%v)/%v?charset=utf8mb4&parseTime=True&loc=Local", dbConfig.User, dbConfig.Password, dbConfig.Addr, dbConfig.DBName)
}
func DBConfigFrom(val string) (*DBConfig, error) {
	dbConfig := &DBConfig{}
	err := json.Unmarshal([]byte(val), dbConfig)
	if err != nil {
		return nil, err
	}
	return  dbConfig, nil
}

var dbConfig DBConfig

func RegisterDBConfig(addr string, user string, password string, dbName string) error {
	dbConfig = DBConfig{Addr: addr, User: user, Password: password, DBName: dbName}
	// fmt.Printf("mysql dsn: %v", dbConfig.getDSN())
	_, err := GetDB()
	return err
}

func GetDB() (*gorm.DB, error) {
	// 连接数据库
	db, err := gorm.Open(mysql.Open(dbConfig.getDSN()), &gorm.Config{})
	if err != nil {
		return nil, NewXError(err, "连接数据库失败")
	}
	return db, nil
}
