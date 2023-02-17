package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/blocklords/gosds/app/argument"
	"github.com/blocklords/gosds/app/env"
	"github.com/blocklords/gosds/security/vault"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	zmq "github.com/pebbe/zmq4"
)

// Environment variables for each SDS Service
type Service struct {
	Name               string // Service name
	broadcast_host     string // Broadcasting host
	broadcast_port     string // Broadcasting port
	host               string // request-reply host
	port               string // request-reply port
	PublicKey          string // The Curve key of the service
	SecretKey          string // The Curve secret key of the service
	BroadcastPublicKey string
	BroadcastSecretKey string
}

func (p *Service) set_curve_key(secret_key string) error {
	p.SecretKey = secret_key

	pub_key, err := zmq.AuthCurvePublic(secret_key)
	if err != nil {
		return err
	}

	p.PublicKey = pub_key

	return nil
}

func (p *Service) set_broadcast_curve_key(secret_key string) error {
	p.BroadcastSecretKey = secret_key

	pub_key, err := zmq.AuthCurvePublic(secret_key)
	if err != nil {
		return err
	}

	p.BroadcastPublicKey = pub_key

	return nil
}

// for example service.New(service.SPAGHETTI, service.REMOTE, service.SUBSCRIBE)
func New(service_type ServiceType, limits ...Limit) (*Service, error) {
	name := string(service_type)
	host_env := name + "_HOST"
	port_env := name + "_PORT"
	broadcast_host_env := name + "_BROADCAST_HOST"
	broadcast_port_env := name + "_BROADCAST_PORT"

	s := Service{
		Name:               name,
		host:               "",
		port:               "",
		broadcast_host:     "",
		broadcast_port:     "",
		PublicKey:          "",
		SecretKey:          "",
		BroadcastPublicKey: "",
		BroadcastSecretKey: "",
	}

	var v *vault.Vault
	exist, err := argument.Exist(argument.PLAIN)
	if err != nil {
		return nil, err
	} else if !exist {
		v, _, err = vault.New()
		if err != nil {
			return nil, err
		}
	}

	for _, limit := range limits {
		switch limit {
		case REMOTE:
			if !env.Exists(port_env) && !env.Exists(host_env) {
				return nil, fmt.Errorf("missing PORT AND HOST environment variables of SDS %s", s.Name)
			}
			s.host = env.GetString(host_env)
			s.port = env.GetString(port_env)

			if !exist {
				s.PublicKey = s.GetPublicKey()
			}
		case THIS:
			if !env.Exists(port_env) {
				return nil, fmt.Errorf("missing PORT environment variable of SDS %s", s.Name)
			}
			s.port = env.GetString(port_env)

			if !exist {
				bucket, key_name := s.SecretKeyVariable()
				SecretKey, err := v.GetString(bucket, key_name)
				if err != nil {
					return nil, err
				}

				if err := s.set_curve_key(SecretKey); err != nil {
					return nil, err
				}
			}
		case SUBSCRIBE:
			if !env.Exists(broadcast_host_env) && !env.Exists(broadcast_port_env) {
				return nil, fmt.Errorf("missing BROADCAST PORT and BROADCAST HOST environment variables of SDS %s", s.Name)
			}
			if !exist {
				s.BroadcastPublicKey = s.GetBroadcastPublicKey()
			}
		case BROADCAST:
			if !env.Exists(broadcast_port_env) {
				return nil, fmt.Errorf("missing BROADCAST PORT environment vairable of SDS %s", s.Name)
			}
			if !exist {
				bucket, key_name := s.BroadcastSecretKeyVariable()
				SecretKey, err := v.GetString(bucket, key_name)
				if err != nil {
					return nil, err
				}

				if err := s.set_broadcast_curve_key(SecretKey); err != nil {
					return nil, err
				}
			}
		}
	}

	return &s, nil
}

// Returns the public key from the environment
func (s *Service) GetPublicKey() string {
	return env.GetString(s.Name + "_PUBLIC_KEY")
}

// Returns the broadcasting public key from the environment
func (s *Service) GetBroadcastPublicKey() string {
	return env.GetString(s.Name + "_BROADCAST_PUBLIC_KEY")
}

// Returns the Vault secret storage and the key for curve private part.
func (s *Service) SecretKeyVariable() (string, string) {
	return "SDS_SERVICES", s.Name + "_SECRET_KEY"
}

// Returns the Vault secret storage and the key for curve private part for broadcaster.
func (s *Service) BroadcastSecretKeyVariable() (string, string) {
	return "SDS_SERVICES", s.Name + "_BROADCAST_SECRET_KEY"
}

// Returns the service environment parameters by its Public Key
func GetByPublicKey(PublicKey string) (*Service, error) {
	for _, service_type := range service_types() {
		service, err := New(service_type)
		if err != nil {
			return nil, err
		}
		if service != nil && service.PublicKey == PublicKey {
			return service, nil
		}
	}

	return nil, errors.New("the service wasn't found for a given public key")
}

// Returns the Service Name
func (e *Service) ServiceName() string {
	caser := cases.Title(language.AmericanEnglish)
	return "SDS " + caser.String(strings.ToLower(e.Name))
}

// Returns the request-reply url as a host:port
func (e *Service) Url() string {
	return e.host + ":" + e.port
}

// Returns the broadcast url as a host:port
func (e *Service) BroadcastUrl() string {
	return e.broadcast_host + ":" + e.broadcast_port
}

// returns the request-reply port
func (e *Service) Port() string {
	return e.port
}

// Returns the broadcast port
func (e *Service) BroadcastPort() string {
	return e.broadcast_port
}

func NewDeveloper(public_key string, secret_key string) *Service {
	return &Service{
		Name:               "developer",
		host:               "",
		port:               "",
		broadcast_host:     "",
		broadcast_port:     "",
		PublicKey:          public_key,
		SecretKey:          secret_key,
		BroadcastPublicKey: public_key,
		BroadcastSecretKey: secret_key,
	}
}

func Developer() (*Service, error) {
	exist, err := argument.Exist(argument.PLAIN)
	if err != nil {
		return nil, err
	}
	if exist {
		return NewDeveloper("", ""), nil
	}
	if !env.Exists("DEVELOPER_PUBLIC_KEY") || !env.Exists("DEVELOPER_SECRET_KEY") {
		return nil, errors.New("missing 'DEVELOPER_PUBLIC_KEY' or 'DEVELOPER_SECRET_KEY'")
	}
	public_key := env.GetString("DEVELOPER_PUBLIC_KEY")
	secret_key := env.GetString("DEVELOPER_SECRET_KEY")

	return NewDeveloper(public_key, secret_key), nil
}
