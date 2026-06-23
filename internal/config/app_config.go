package config

import (
	"net"
	"net/url"
	"strconv"
)

type AppConfig struct {
	WEB      WEBConfig      `yaml:"web"`
	Database DatabaseConfig `yaml:"database"`
}
type WEBConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

func (w WEBConfig) ListenAddress() string {
	return net.JoinHostPort(w.Host, strconv.Itoa(w.Port))
}

func (d DatabaseConfig) ConnectionString() string {
	var user *url.Userinfo
	if d.Password == "" {
		user = url.User(d.User)
	} else {
		user = url.UserPassword(d.User, d.Password)
	}

	return (&url.URL{
		Scheme: "postgres",
		User:   user,
		Host:   net.JoinHostPort(d.Host, strconv.Itoa(d.Port)),
		Path:   d.Database,
	}).String()
}
