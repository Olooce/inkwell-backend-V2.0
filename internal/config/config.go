package config

import (
	"encoding/xml"
	"io"
	"os"
	"sync"
)

var (
	cfg  *APIConfig
	once sync.Once
)

// APIConfig represents the root element.
type APIConfig struct {
	XMLName        xml.Name             `xml:"API"`
	RequestDump    bool                 `xml:"REQUEST_DUMP,attr"`
	Context        ContextConfig        `xml:"CONTEXT"`
	Authentication AuthenticationConfig `xml:"AUTHENTICATION"`
	Pagination     PaginationConfig     `xml:"PAGINATION"`
	DB             DBConfig             `xml:"DB"`
	THIRD_PARTY    ThirdPartyConfig     `xml:"THIRD_PARTY"`
}

// ContextConfig holds basic server settings.
type ContextConfig struct {
	Port            int    `xml:"PORT"`
	Host            string `xml:"HOST"`
	Path            string `xml:"PATH"`
	TimeZone        string `xml:"TIME_ZONE"`
	EnableBasicAuth bool   `xml:"ENABLE_BASIC_AUTH"`
}

type ThirdPartyConfig struct {
	HFToken string `xml:"HF_TOKEN"`
}

// AuthenticationConfig holds authentication settings.
type AuthenticationConfig struct {
	MultipleSameUserSessions bool `xml:"MULTIPLE_SAME_USER_SESSIONS,attr"`
	EnableTokenAuth          bool `xml:"ENABLE_TOKEN_AUTH"`
	SessionTimeout           int  `xml:"SESSION_TIMEOUT"`
}

// PaginationConfig holds pagination settings.
type PaginationConfig struct {
	PageSize int `xml:"PAGE_SIZE"`
}

// DBConfig holds database connection settings.
type DBConfig struct {
	Initialize bool         `xml:"INITIALIZE"`
	Server     string       `xml:"SERVER"`
	Host       string       `xml:"HOST"`
	Port       int          `xml:"PORT"`
	Driver     string       `xml:"DRIVER"`
	SSLMode    string       `xml:"SSL_MODE"`
	Names      DBNames      `xml:"NAMES"`
	Username   string       `xml:"USERNAME"`
	Password   DBPassword   `xml:"PASSWORD"`
	Pool       DBPoolConfig `xml:"POOL"`
}

// DBNames holds the names defined in the DB section.
type DBNames struct {
	INKWELL string `xml:"INKWELL,attr"`
}

// DBPassword holds password details.
type DBPassword struct {
	Type  string `xml:"TYPE,attr"`
	Value string `xml:",chardata"`
}

// DBPoolConfig holds database connection pooling settings.
type DBPoolConfig struct {
	MaxOpenConns    int `xml:"MAX_OPEN_CONNS"`
	MaxIdleConns    int `xml:"MAX_IDLE_CONNS"`
	ConnMaxLifetime int `xml:"CONN_MAX_LIFETIME"`
}

// LoadConfig loads and parses the XML configuration from the given file.
func LoadConfig(xmlPath string) (*APIConfig, error) {
	once.Do(func() {
		f, err := os.Open(xmlPath)
		if err != nil {
			return
		}
		defer func(f *os.File) {
			err := f.Close()
			if err != nil {

			}
		}(f)

		data, err := io.ReadAll(f)
		if err != nil {
			return
		}

		var newCfg APIConfig
		if err := xml.Unmarshal(data, &newCfg); err != nil {
			return
		}

		cfg = &newCfg
	})

	if cfg == nil {
		return nil, os.ErrInvalid
	}
	return cfg, nil
}

// GetConfig returns the loaded configuration.
func GetConfig() *APIConfig {
	return cfg
}
