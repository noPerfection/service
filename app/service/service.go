package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/security/vault"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	zmq "github.com/pebbe/zmq4"
)

// Environment variables for each SDS Service
type Service struct {
	Name               string // Service name
	PublicKey          string // The Curve key of the service
	SecretKey          string // The Curve secret key of the service
	BroadcastPublicKey string
	BroadcastSecretKey string
	inproc             bool
	url                string
	broadcast_url      string
}

func (p *Service) set_curve_key(secret_key string) error {
	p.SecretKey = secret_key

	pub_key, err := zmq.AuthCurvePublic(secret_key)
	if err != nil {
		return fmt.Errorf("zmq.Convert Secret to Pub: %w", err)
	}

	p.PublicKey = pub_key

	return nil
}

func (p *Service) set_broadcast_curve_key(secret_key string) error {
	p.BroadcastSecretKey = secret_key

	pub_key, err := zmq.AuthCurvePublic(secret_key)
	if err != nil {
		return fmt.Errorf("zmq.Convert Secret to Pub: %w", err)
	}

	p.BroadcastPublicKey = pub_key

	return nil
}

// for example service.NewInternal(service.SPAGHETTI, service.REMOTE, service.SUBSCRIBE)
// Creates the service with the parameters but without any information
func Inprocess(service_type ServiceType) (*Service, error) {
	name := string(service_type)

	s := Service{
		Name:               name,
		inproc:             true,
		url:                "inproc://reply_" + name,
		broadcast_url:      "inproc://pub_" + name,
		PublicKey:          "",
		SecretKey:          "",
		BroadcastPublicKey: "",
		BroadcastSecretKey: "",
	}

	return &s, nil
}

// for example service.NewInternal(service.SPAGHETTI, service.REMOTE, service.SUBSCRIBE)
// Creates the service with the parameters but without any information
func NewExternal(service_type ServiceType, limits ...Limit) (*Service, error) {
	default_configuration := DefaultConfiguration(service_type)
	app_config := configuration.NewService(default_configuration)

	name := string(service_type)
	host_env := name + "_HOST"
	port_env := name + "_PORT"
	broadcast_host_env := name + "_BROADCAST_HOST"
	broadcast_port_env := name + "_BROADCAST_PORT"

	s := Service{
		Name:               name,
		inproc:             false,
		PublicKey:          "",
		SecretKey:          "",
		BroadcastPublicKey: "",
		BroadcastSecretKey: "",
	}

	for _, limit := range limits {
		switch limit {
		case REMOTE:
			s.url = fmt.Sprintf("tcp://%s:%s", app_config.GetString(host_env), app_config.GetString(port_env))
		case THIS:
			s.url = fmt.Sprintf("tcp://*:%s", app_config.GetString(port_env))
		case SUBSCRIBE:
			s.broadcast_url = fmt.Sprintf("tcp://%s:%s", app_config.GetString(broadcast_host_env), app_config.GetString(broadcast_port_env))
		case BROADCAST:
			s.broadcast_url = fmt.Sprintf("tcp://*:%s", app_config.GetString(broadcast_port_env))
		}
	}

	return &s, nil
}

// Creates the service with the parameters that includes
// private and private key
func NewSecure(service_type ServiceType, limits ...Limit) (*Service, error) {
	default_configuration := DefaultConfiguration(service_type)
	app_config := configuration.NewService(default_configuration)

	name := string(service_type)
	public_key := name + "_PUBLIC_KEY"
	broadcast_public_key := name + "_BROADCAST_PUBLIC_KEY"

	s, err := NewExternal(service_type, limits...)
	if err != nil {
		return nil, fmt.Errorf("service.New: %w", err)
	}

	for _, limit := range limits {
		switch limit {
		case REMOTE:
			if !app_config.Exist(public_key) {
				return nil, fmt.Errorf("security enabled, but missing %s", s.Name)
			}
			s.PublicKey = app_config.GetString(public_key)
		case THIS:
			bucket, key_name := s.SecretKeyVariable()

			SecretKey, err := vault.GetStringFromVault(bucket, key_name)
			if err != nil {
				return nil, fmt.Errorf("vault.GetString for %s service secret key: %w", s.Name, err)
			}

			if err := s.set_curve_key(SecretKey); err != nil {
				return nil, fmt.Errorf("socket.set_curve_key %s: %w", s.Name, err)
			}
		case SUBSCRIBE:
			if !app_config.Exist(broadcast_public_key) {
				return nil, fmt.Errorf("security enabled, but missing %s", s.Name)
			}
			s.BroadcastPublicKey = app_config.GetString(broadcast_public_key)
		case BROADCAST:
			bucket, key_name := s.BroadcastSecretKeyVariable()
			SecretKey, err := vault.GetStringFromVault(bucket, key_name)
			if err != nil {
				return nil, fmt.Errorf("vault.GetString for %s service broadcast secret key: %w", s.Name, err)

			}

			if err := s.set_broadcast_curve_key(SecretKey); err != nil {
				return nil, fmt.Errorf("socket.set_broadcast_curve_key %s: %w", s.Name, err)
			}
		}
	}

	return s, nil
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
		service, err := NewSecure(service_type, THIS)
		if err != nil {
			return nil, fmt.Errorf("service.New(`%s`): %w", service_type, err)
		}
		if service != nil && service.PublicKey == PublicKey {
			return service, nil
		}
	}

	return nil, fmt.Errorf("public key '%s' not matches to any service", PublicKey)
}

// Returns the Service Name
func (e *Service) ServiceName() string {
	caser := cases.Title(language.AmericanEnglish)
	return "SDS " + caser.String(strings.ToLower(e.Name))
}

// Returns the request-reply url as a host:port
func (e *Service) Url() string {
	return e.url
}

// Returns the broadcast url as a host:port
func (e *Service) BroadcastUrl() string {
	return e.broadcast_url
}

func NewDeveloper(public_key string, secret_key string) *Service {
	return &Service{
		Name:               "developer",
		url:                "",
		broadcast_url:      "",
		inproc:             false,
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
