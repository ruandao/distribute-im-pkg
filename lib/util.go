package lib

import (
	"bytes"
	"encoding/json"

	"github.com/spf13/viper"
)

func ReadFromJSON(dataBytes []byte, store any) error {
	v := viper.New()
	v.SetConfigType("json")

	// 从字节数组读取配置
	if err := v.ReadConfig(bytes.NewBuffer(dataBytes)); err != nil {
		return NewXError(err, "读取配置失败")
	}

	// 初始化配置结构体

	// 将配置解析到结构体
	if err := v.Unmarshal(&store); err != nil {
		return NewXError(err, "解析配置失败")
	}
	return nil
}

func WriteIntoJSONIndent(val any) string {
	jsonData, _ := json.MarshalIndent(val, "", " ")
	return string(jsonData)
}
