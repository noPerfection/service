package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/blocklords/gosds/app/configuration"
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
	default_configuration := DefaultConfiguration(service_type)
	app_config, err := configuration.NewService(default_configuration)
	if err != nil {
		return nil, err
	}

	name := string(service_type)
	host_env := name + "_HOST"
	port_env := name + "_PORT"
	broadcast_host_env := name + "_BROADCAST_HOST"
	broadcast_port_env := name + "_BROADCAST_PORT"
	public_key := name + "_PUBLIC_KEY"
	broadcast_public_key := name + "_BROADCAST_PUBLIC_KEY"

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

	if !app_config.Plain {
		new_vault, _, err := vault.New(app_config)
		if err != nil {
			return nil, err
		} else {
			v = new_vault
		}
	}

	for _, limit := range limits {
		switch limit {
		case REMOTE:
			s.host = app_config.GetString(host_env)
			s.port = app_config.GetString(port_env)

			if !app_config.Plain {
				if !app_config.Exist(public_key) {
					return nil, fmt.Errorf("security enabled, but missing %s", s.Name)
				}
				s.PublicKey = app_config.GetString(public_key)
			}
		case THIS:
			s.port = app_config.GetString(port_env)

			if !app_config.Plain {
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
			s.broadcast_host = app_config.GetString(broadcast_host_env)
			s.broadcast_port = app_config.GetString(broadcast_port_env)

			if !app_config.Plain {
				if !app_config.Exist(broadcast_public_key) {
					return nil, fmt.Errorf("security enabled, but missing %s", s.Name)
				}
				s.BroadcastPublicKey = app_config.GetString(broadcast_public_key)
			}
		case BROADCAST:
			s.port = app_config.GetString(broadcast_port_env)

			if !app_config.Plain {
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

func Developer(app_config *configuration.Config) (*Service, error) {
	if app_config.Plain {
		return NewDeveloper("", ""), nil
	}
	if !app_config.Exist("DEVELOPER_PUBLIC_KEY") || !app_config.Exist("DEVELOPER_SECRET_KEY") {
		return nil, errors.New("missing 'DEVELOPER_PUBLIC_KEY' or 'DEVELOPER_SECRET_KEY'")
	}
	public_key := app_config.GetString("DEVELOPER_PUBLIC_KEY")
	secret_key := app_config.GetString("DEVELOPER_SECRET_KEY")

	return NewDeveloper(public_key, secret_key), nil
}
