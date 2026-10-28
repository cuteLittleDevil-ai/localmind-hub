package config

import "github.com/zeromicro/go-zero/rest"

type Config struct {
	rest.RestConf

	Auth  AuthConf
	Data  DataConf
	Redis RedisConf
	Mongo MongoConf
	Mqtt  MqttConf
}

type AuthConf struct {
	DemoToken string `json:",default=localmind-demo-token"`
}

type DataConf struct {
	SqlitePath string `json:",default=data/localmind.db"`
}

type RedisConf struct {
	Addr string `json:",default=127.0.0.1:6379"`
}

type MongoConf struct {
	URI      string `json:",default=mongodb://127.0.0.1:27017"`
	Database string `json:",default=localmind"`
}

type MqttConf struct {
	Broker string `json:",default=tcp://127.0.0.1:1883"`
}
