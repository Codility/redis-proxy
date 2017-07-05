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

type AddrSpec struct {
	Addr string `json:"addr"`
	Pass string `json:"pass"`
	TLS  bool   `json:"tls"`
	Unix bool   `json:"unix"`

	CertFile   string `json:"certfile"`
	KeyFile    string `json:"keyfile"`
	CACertFile string `json:"cacertfile"`
}

func (as *AddrSpec) AsJSON() string {
	res, err := json.Marshal(as)
	if err != nil {
		return ""
	}
	return string(res)
}

func (as *AddrSpec) Dial() (net.Conn, error) {
	network := "tcp"
	if as.Unix {
		network = "unix"
	}

	if !as.TLS {
		return net.Dial(network, as.Addr)
	}

	// TODO: read the PEM once, not at every accept
	certPEM, err := ioutil.ReadFile(as.CACertFile)
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

	return tls.Dial(network, as.Addr, &tls.Config{
		RootCAs: roots,
	})
}

// Returns:
// - top-level generic net.Listener
// - the underlying net.TCPListener (different from the first listener in case of TLS)
// - effective address
// - error, if any
//
// The reason for explicitely returning net.TCPListener is that the
// proxy needs it to set deadlines on accept operations, but the
// listener from tls package does not support them, and does not
// provide any way to get to the underlying TCPListener.
func (as *AddrSpec) Listen() (net.Listener, *net.TCPListener, *net.Addr, error) {
	ln, err := net.Listen("tcp", as.Addr)
	if err != nil {
		log.Fatalf("Could not listen: %s", err)
		return nil, nil, nil, err
	}
	addr := ln.(*net.TCPListener).Addr()

	if !as.TLS {
		return ln, ln.(*net.TCPListener), &addr, nil
	}

	cer, err := tls.LoadX509KeyPair(as.CertFile, as.KeyFile)
	if err != nil {
		log.Fatalf("Could not load key pair (%s, %s): %s",
			as.CertFile, as.KeyFile, err)
		return nil, nil, nil, err
	}
	tlsLn := tls.NewListener(ln, &tls.Config{
		Certificates: []tls.Certificate{cer},
	})
	return tlsLn, ln.(*net.TCPListener), &addr, nil
}

func (as *AddrSpec) Prepare(name string, server bool) ErrorList {
	if as.Addr == "" {
		return ErrorList{[]string{"Missing " + name + " address"}}
	}

	var err error
	errors := ErrorList{}

	pemFileReadable := func(name string) bool {
		_, err = ioutil.ReadFile(name)
		if err != nil {
			log.Print(err)
		}
		return err == nil
	}

	if as.TLS {
		if server {
			if as.CertFile == "" {
				errors.Add(name + ".tls requires certfile")
			} else if !pemFileReadable(as.CertFile) {
				errors.Add("could not load " + name + ".certfile: " + as.CertFile)
			}

			if as.KeyFile == "" {
				errors.Add(name + ".tls requires keyfile")
			} else if !pemFileReadable(as.KeyFile) {
				errors.Add("could not load " + name + ".keyfile: " + as.KeyFile)
			}
		} else {
			if as.CACertFile == "" {
				errors.Add("uplink.tls requires cacertfile")
			} else if !pemFileReadable(as.CACertFile) {
				errors.Add("could not load " + name + ".cacertfile: " + as.CACertFile)
			}
		}
	}

	if errors.Ok() && !server {
		conn, err := as.Dial()
		if err != nil {
			log.Print(err)
			tlsStr := "(non-TLS)"
			if as.TLS {
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

func (c *Config) ValidateSwitchTo(new *Config) error {
	if c.Listen != new.Listen {
		return errors.New("New config must have the same `listen` block as the old one.")
	}
	if c.Admin != new.Admin {
		return errors.New("New config must have the same `admin` block as the old one.")
	}
	return nil
}

func (c *Config) AsJSON() string {
	res, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return string(res)
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
