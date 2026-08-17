package manager

import (
	"fmt"
	"time"

	"github.com/noPerfection/service/mushroom"
	"github.com/noPerfection/topology"
)

const (
	CONNECTION_STATUS_NOT_SET     = "not-set"      // 1. as default, no connection established -> request handshake
	CONNECTION_STATUS_NOT_FOUND   = "not-found"    // 1. not found returned by check status to say to initiate
	CONNECTION_STATUS_MISMATCH    = "mismatch"     // 1. if status mismatch when doing a status check
	CONNECTION_STATUS_NOT_CHECKED = "not-checked"  // 2. once handshaked, set not-checked -> is-service-running
	CONNECTION_STATUS_CHECKING    = "checking"     // 3. once handshaked, set checking -> is-service-running
	CONNECTION_STATUS_ACCESS_ERR  = "access-error" // 3. if hmac error -> request handshake
	CONNECTION_STATUS_CURVE_ERR   = "curve-error"  // 4. if curve mismatch -> request handshake
	CONNECTION_STATUS_CONNECTED   = "connected"    // 5. skip manager
	CONNECTION_STATUS_TIMEOUT     = "timeout"      // timeout -> request is-service-running
)

type ConnectionStatus struct {
	status      string
	updatedTime time.Time
	tick        time.Time
}

func NewConnectionStatus(status string) ConnectionStatus {
	c := ConnectionStatus{}
	c.Set(status)
	return c
}

func (c *ConnectionStatus) Set(status string) bool {
	if c.status == status {
		c.updatedTime = time.Now()
		c.tick = time.Now()
		return false
	}
	c.status = status
	c.updatedTime = time.Now()
	c.tick = time.Now()
	return true
}

// Returns status, if not set return CONNECTION_STATUS_NOT_SET ('not-set')
func (c *ConnectionStatus) Get() string {
	if c.status == "" {
		return CONNECTION_STATUS_NOT_SET
	}
	return c.status
}

func (c *ConnectionStatus) Tick() {
	c.tick = time.Now()
}

func (c *ConnectionStatus) String() string {
	if c.status == "" {
		return CONNECTION_STATUS_NOT_SET
	}
	return fmt.Sprintf("%s@%s, tick: %s", c.status, c.updatedTime.Format(time.RFC3339), c.tick.Format(time.RFC3339))
}

type ManagerConnection struct {
	status        ConnectionStatus
	client        *topology.Client
	hmac          string
	hash          string
	lockGoroutine bool
}

type InboundConnection struct {
	status    ConnectionStatus
	depURL    mushroom.TopologyURL
	hmac      string
	cacheHash string
}

type EdgeConnectionMsg struct {
	Status    string `json:"status"`
	DepURL    string `json:"dep-url"`
	CacheHash string `json:"cache-hash,omitempty"`
}

type OutboundConnection struct {
	status    ConnectionStatus
	cacheHash string
	depURL    mushroom.TopologyURL
	hmac      string
}

type ServiceStatus struct {
	serviceURL mushroom.TopologyURL
	// inproc | ipc | tcp
	protocol      string
	managerCon    *ManagerConnection
	inboundCons   map[string]*InboundConnection
	outboundCons  map[string]*OutboundConnection
	inboundEdges  map[string]*InboundConnection
	outboundEdges map[string]*OutboundConnection
	inbounds      map[string]*InboundConnection
	outbounds     map[string]*OutboundConnection
}

type ServiceStatusMsg struct {
	InboundCons   map[string]EdgeConnectionMsg `json:"inbound-cons,omitempty"`
	OutboundCons  map[string]EdgeConnectionMsg `json:"outbound-cons,omitempty"`
	InboundEdges  map[string]EdgeConnectionMsg `json:"inbound-edges,omitempty"`
	OutboundEdges map[string]EdgeConnectionMsg `json:"outbound-edges,omitempty"`
	Inbounds      map[string]EdgeConnectionMsg `json:"inbounds,omitempty"`
	Outbounds     map[string]EdgeConnectionMsg `json:"outbounds,omitempty"`
}

type Context map[string]map[string]*ServiceStatus

// If the dependency status has anything but outbounds to this service,
// then the dependency is not handshakeable, because this service is not connecting to this dependency.
func (s *ServiceStatus) isHandshakeable(contextURL string) bool {
	return contextURL != s.serviceURL.String()
}

func (s *ServiceStatus) toMsg() *ServiceStatusMsg {
	msg := &ServiceStatusMsg{}
	if len(s.inboundCons) > 0 {
		msg.InboundCons = make(map[string]EdgeConnectionMsg)
		for depURL, inboundCon := range s.inboundCons {
			msg.InboundCons[depURL] = EdgeConnectionMsg{
				Status: inboundCon.status.Get(),
				DepURL: inboundCon.depURL.String(),
			}
		}
	}
	if len(s.outboundCons) > 0 {
		msg.OutboundCons = make(map[string]EdgeConnectionMsg)
		for depURL, outboundCon := range s.outboundCons {
			msg.OutboundCons[depURL] = EdgeConnectionMsg{
				Status: outboundCon.status.Get(),
				DepURL: outboundCon.depURL.String(),
			}
		}
	}
	if len(s.inboundEdges) > 0 {
		msg.InboundEdges = make(map[string]EdgeConnectionMsg)
		for depURL, inboundEdge := range s.inboundEdges {
			msg.InboundEdges[depURL] = EdgeConnectionMsg{
				Status: inboundEdge.status.Get(),
				DepURL: inboundEdge.depURL.String(),
			}
		}
	}
	if len(s.outboundEdges) > 0 {
		msg.OutboundEdges = make(map[string]EdgeConnectionMsg)
		for depURL, outboundEdge := range s.outboundEdges {
			msg.OutboundEdges[depURL] = EdgeConnectionMsg{
				Status: outboundEdge.status.Get(),
				DepURL: outboundEdge.depURL.String(),
			}
		}
	}
	if len(s.inbounds) > 0 {
		msg.Inbounds = make(map[string]EdgeConnectionMsg)
		for depURL, inbound := range s.inbounds {
			edge := EdgeConnectionMsg{
				Status: inbound.status.Get(),
				DepURL: inbound.depURL.String(),
			}
			if edge.Status == CONNECTION_STATUS_CONNECTED {
				edge.CacheHash = inbound.cacheHash
			}
			msg.Inbounds[depURL] = edge
		}
	}
	if len(s.outbounds) > 0 {
		msg.Outbounds = make(map[string]EdgeConnectionMsg)
		for depURL, outbound := range s.outbounds {
			edge := EdgeConnectionMsg{
				Status: outbound.status.Get(),
				DepURL: outbound.depURL.String(),
			}
			if edge.Status == CONNECTION_STATUS_CONNECTED {
				edge.CacheHash = outbound.cacheHash
			}
			msg.Outbounds[depURL] = edge
		}
	}
	return msg
}

func (c *Context) Print() {
	for contextURL, deps := range *c {
		fmt.Printf("        ContextURL: %s, deps: %d\n", contextURL, len(deps))
		for depDeref, depStatus := range deps {
			fmt.Printf("          DepURL: %s, Protocol: %s\n", depDeref, depStatus.protocol)
			fmt.Printf("              ManagerCon: %s\n", depStatus.managerCon.status.String())
			fmt.Printf("              Inbounds:   %v\n", len(depStatus.inbounds) > 0)
			fmt.Printf("              Outbounds:  %v\n", len(depStatus.outbounds) > 0)
			fmt.Printf("              InboundCons:  %v, InboundEdges:  %v\n", len(depStatus.inboundCons) > 0, len(depStatus.inboundEdges) > 0)
			fmt.Printf("              OutboundCons: %v, OutboundEdges: %v\n", len(depStatus.outboundCons) > 0, len(depStatus.outboundEdges) > 0)
		}
	}
}
