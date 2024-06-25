package util

import (
    "path/filepath"
    "log"
    "os"

    "gopkg.in/yaml.v3"
)

type ServerConfig struct {
    Address string `yaml:"address"`
    Port int `yaml:"port"`
}
type JFrogConfig struct {
    URL string `yaml:"url"`
    Repository string `yaml:"repository"`
    Token string `yaml:"token"`
}
type Config struct {
    Server ServerConfig `yaml:"server"`
    JFrog JFrogConfig `yaml:"jfrog"`
    CacheTimeout int64 `yaml:"cacheTimeout"`
}

func ReadConfig(path string) *Config {
    var config Config
    data, err := os.ReadFile(filepath.Clean(path))
    if err != nil {
        log.Panicf("Error opening config file at path %v. err=%v", path, err)
    }
    if err := yaml.Unmarshal(data, &config); err != nil {
        log.Panicf("Error unmarshaling config file from YAML at path %v. err=%v", path, err)
    }

    // overwrite secrets from environment variables, if they are set (also overwrite even if the value is an empty string - if the env-variable exists, it will overwrite)
    if envvar, ok := os.LookupEnv("UENV_LIST_JFROG_TOKEN"); ok {
        config.JFrog.Token = envvar
    }

    if config.JFrog.URL == "" || config.JFrog.Repository == "" || config.JFrog.Token == "" {
        log.Fatalf("JFrog config section does not pass sanity checks. URL, Repository and Token must all not be empty")
    }
    return &config
}
