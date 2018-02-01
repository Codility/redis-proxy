package rproxy

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net"
	"strings"
)

const (
	SanitizedPass = "[removed]"
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
	Addr    string `json:"addr"`
	Pass    string `json:"pass"`
	TLS     bool   `json:"tls"`
	Network string `json:"network"`

	CertFile   string `json:"certfile"`
	KeyFile    string `json:"keyfile"`
	CACertFile string `json:"cacertfile"`
	SkipVerify bool   `json:"skipverify,omitempty"`
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
	if as.Network != "" {
		network = as.Network
	}
	if !(network == "tcp" || network == "unix") {
		return nil, errors.New("Unsupported network for dialing: " + network)
	}

	if !as.TLS {
		return net.Dial(network, as.Addr)
	}

	roots := x509.NewCertPool()
	if !as.SkipVerify {
		// TODO: read the PEM once, not at every accept
		certPEM, err := ioutil.ReadFile(as.CACertFile)
		if err != nil {
			log.Print("Could not load cert: " + err.Error())
			return nil, err
		}

		if !roots.AppendCertsFromPEM(certPEM) {
			err := errors.New("Could not add cert to pool")
			log.Fatal(err)
			return nil, err
		}
	}

	return tls.Dial(network, as.Addr, &tls.Config{
		RootCAs:            roots,
		InsecureSkipVerify: as.SkipVerify,
	})
}

func (as *AddrSpec) Listen() (*Listener, error) {
	network := "tcp"
	if as.Network != "" {
		network = as.Network
	}

	if !(network == "tcp" || network == "unix") {
		return nil, errors.New("Unsupported network for listening: " + network)
	}

	ln, err := net.Listen(network, as.Addr)
	if err != nil {
		log.Fatalf("Could not listen: %s", err)
		return nil, err
	}

	// AddrDeadliner requires funcs that are implemented on both
	// net.TCPListener and net.UnixListener.  We limit the values
	// for `network` above, so those should be the only cases, and
	// so it's okay to assume it will crash otherwise.
	if !as.TLS {
		return &Listener{ln, ln.(AddrDeadliner)}, nil
	}
	tlsLn := tls.NewListener(ln, as.GetTLSConfig())
	return &Listener{tlsLn, ln.(AddrDeadliner)}, nil
}

func (as *AddrSpec) GetTLSConfig() *tls.Config {
	if !as.TLS {
		return nil
	}
	cer, err := tls.LoadX509KeyPair(as.CertFile, as.KeyFile)
	if err != nil {
		log.Fatalf("Could not load key pair (%s, %s): %s",
			as.CertFile, as.KeyFile, err)
		return nil
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cer},
	}
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
			if !as.SkipVerify {
				if as.CACertFile == "" {
					errors.Add("uplink.tls requires cacertfile")
				} else if !pemFileReadable(as.CACertFile) {
					errors.Add("could not load " + name + ".cacertfile: " + as.CACertFile)
				}
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

func (a *AddrSpec) SanitizedForPublication() *AddrSpec {
	return &AddrSpec{
		Addr:       a.Addr,
		Pass:       SanitizedPass,
		TLS:        a.TLS,
		Network:    a.Network,
		CertFile:   a.CertFile,
		KeyFile:    a.KeyFile,
		CACertFile: a.CACertFile,
	}
}

////////////////////////////////////////
// Config

type Config struct {
	Uplink          AddrSpec `json:"uplink"`
	Listen          AddrSpec `json:"listen"`
	ListenRaw       AddrSpec `json:"listen_raw"`
	Admin           AddrSpec `json:"admin"`
	ReadTimeLimitMs int64    `json:"read_time_limit_ms"`
	LogMessages     bool     `json:"log_messages"`
}

type ConfigLoader interface {
	Load() (*Config, error)
}

func (c *Config) Prepare() ErrorList {
	errList := ErrorList{}

	if c.Admin.Addr != "" {
		errList.Append(c.Admin.Prepare("admin", true))
	}
	errList.Append(c.Listen.Prepare("listen", true))
	errList.Append(c.Uplink.Prepare("uplink", false))

	if c.ListenRaw.Addr != "" {
		if c.ListenRaw.Pass != "" {
			errList.Add("listen_raw does not support in-proxy authentication")
		}
	}

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

func (c *Config) SanitizedForPublication() *Config {
	return &Config{
		Uplink:          *c.Uplink.SanitizedForPublication(),
		Listen:          *c.Listen.SanitizedForPublication(),
		ListenRaw:       *c.ListenRaw.SanitizedForPublication(),
		Admin:           *c.Admin.SanitizedForPublication(),
		ReadTimeLimitMs: c.ReadTimeLimitMs,
		LogMessages:     c.LogMessages,
	}
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
// InputConfigLoader

type InputConfigLoader struct {
	reader io.Reader
	loaded bool
}

func NewInputConfigLoader(reader io.Reader) *InputConfigLoader {
	return &InputConfigLoader{reader: reader, loaded: false}
}

func (c *InputConfigLoader) Load() (*Config, error) {
	if c.loaded {
		return nil, errors.New("Cannot reload config when it's read from input.")
	}
	c.loaded = true
	configJson, err := ioutil.ReadAll(c.reader)
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
		},
	}
}

func (c *TestConfigLoader) Load() (*Config, error) {
	return c.conf, c.err
}

func (c *TestConfigLoader) Replace(conf *Config) {
	c.conf = conf
}
