package rproxy

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"strings"
)

////////////////////////////////////////
// ErrorList

type ErrorList struct {
	errors []string
}

func (l *ErrorList) Errors() []string {
	if l.errors == nil {
		return []string{}
	} else {
		return l.errors
	}
}

func (l *ErrorList) Add(error string) {
	l.errors = append(l.errors, error)
}

func (l *ErrorList) Ok() bool {
	return l.errors == nil || len(l.errors) == 0
}

func (l *ErrorList) Append(other ErrorList) {
	l.errors = append(l.errors, other.errors...)
}

func (l *ErrorList) AsError() error {
	if l.Ok() {
		return nil
	}
	return errors.New(strings.Join(l.Errors(), ", "))
}

////////////////////////////////////////
// AddrSpec

type TLSSpec struct {
	CertFile   string `json:"certfile"`
	KeyFile    string `json:"keyfile"`
	CACertFile string `json:"cacertfile"`

	CertFilePEM   []byte `json:"-"`
	KeyFilePEM    []byte `json:"-"`
	CACertFilePEM []byte `json:"-"`
}

type AddrSpec struct {
	Addr string   `json:"addr"`
	Pass string   `json:"pass"`
	TLS  *TLSSpec `json:"tls"`
}

func (as *AddrSpec) Equal(other *AddrSpec) bool {
	return (as.Addr == other.Addr) &&
		(as.Pass == other.Pass)
}

func (as *AddrSpec) Dial() (net.Conn, error) {
	if as.TLS == nil {
		return net.Dial("tcp", as.Addr)
	}

	// TODO: read the PEM once, not at every accept
	certPEM, err := ioutil.ReadFile(as.TLS.CACertFile)
	if err != nil {
		log.Print("Could not load cert: " + err.Error())
		return nil, err
	}

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certPEM) {
		err := errors.New("Could not add cert to pool")
		log.Fatal(err)
		return nil, err
	}

	return tls.Dial("tcp", as.Addr, &tls.Config{
		RootCAs: roots,
	})
}

func (as *AddrSpec) Prepare(name string, server bool) ErrorList {
	if as.Addr == "" {
		return ErrorList{[]string{"Missing " + name + " address"}}
	}

	var err error
	errors := ErrorList{}

	if as.TLS != nil {
		if server {
			if as.TLS.CertFile == "" {
				errors.Add(name + ".tls requires certfile")
			} else {
				as.TLS.CertFilePEM, err = ioutil.ReadFile(as.TLS.CertFile)
				if err != nil {
					log.Print(err)
					errors.Add("could not load " + name + ".tls.certfile: " + as.TLS.CertFile)
				}
			}

			if as.TLS.KeyFile == "" {
				errors.Add(name + ".tls requires keyfile")
			} else {
				as.TLS.KeyFilePEM, err = ioutil.ReadFile(as.TLS.KeyFile)
				if err != nil {
					log.Print(err)
					errors.Add("could not load " + name + ".tls.keyfile: " + as.TLS.KeyFile)
				}
			}
		} else {
			if as.TLS.CACertFile == "" {
				errors.Add("uplink.tls requires cacertfile")
			} else {
				as.TLS.CACertFilePEM, err = ioutil.ReadFile(as.TLS.CACertFile)
				if err != nil {
					log.Print(err)
					errors.Add("could not load " + name + ".tls.cacertfile: " + as.TLS.CACertFile)
				}
			}
		}
	}

	if errors.Ok() && !server {
		conn, err := as.Dial()
		if err != nil {
			log.Print(err)
			tlsStr := "(non-TLS)"
			if as.TLS != nil {
				tlsStr = "(TLS)"
			}
			errors.Add("could not connect to " + name + ": " + as.Addr + " " + tlsStr)
		} else {
			conn.Close()
		}
	}
	return errors
}

////////////////////////////////////////
// Config

type Config struct {
	Uplink          AddrSpec `json:"uplink"`
	Listen          AddrSpec `json:"listen"`
	Admin           AddrSpec `json:"admin"`
	ReadTimeLimitMs int64    `json:"read_time_limit_ms"`
	LogMessages     bool     `json:"log_messages"`
}

type ConfigLoader interface {
	Load() (*Config, error)
}

func (c *Config) Prepare() ErrorList {
	errList := ErrorList{}

	errList.Append(c.Admin.Prepare("admin", true))
	errList.Append(c.Listen.Prepare("listen", true))
	errList.Append(c.Uplink.Prepare("uplink", false))

	return errList
}

////////////////////////////////////////
// FileConfigLoader

type FileConfigLoader struct {
	fileName string
}

func NewFileConfigLoader(name string) *FileConfigLoader {
	return &FileConfigLoader{name}
}

func (f *FileConfigLoader) Load() (*Config, error) {
	configJson, err := ioutil.ReadFile(f.fileName)
	if err != nil {
		return nil, err
	}
	var config Config
	return &config, json.Unmarshal(configJson, &config)
}

////////////////////////////////////////
// TestConfigLoader

type TestConfigLoader struct {
	conf *Config
	err  error
}

func NewTestConfigLoader(uplinkAddr string) *TestConfigLoader {
	return &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: uplinkAddr},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
			Admin:  AddrSpec{Addr: "127.0.0.1:0"},
		},
	}
}

func (c *TestConfigLoader) Load() (*Config, error) {
	return c.conf, c.err
}

func (c *TestConfigLoader) Replace(conf *Config) {
	c.conf = conf
}
