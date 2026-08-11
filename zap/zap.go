// Package zap implements the process-wide ZeroMQ Authentication Protocol handler.
package zap

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/noPerfection/service/mushroom"
	"github.com/noPerfection/topology"
	zmq "github.com/pebbe/zmq4"
)

const curveAllowAny = "*"
const zapTopologyTimeout = 500 * time.Millisecond

var (
	zapTopologyClient *topology.Client
	zapHandler        *zmq.Socket
	zapQuit           *zmq.Socket

	zapInit         = false
	zapVerboseValue = false
	zapVerboseLock  sync.RWMutex

	zapAllow           = make(map[string]map[string]bool)
	zapDeny            = make(map[string]map[string]bool)
	zapDynamicAllow    = make(map[string]bool)
	zapAllowNet        = make(map[string][]*net.IPNet)
	zapDenyNet         = make(map[string][]*net.IPNet)
	zapUsers           = make(map[string]map[string]string)
	zapPubkeys         = make(map[string]map[string]bool)
	zapPubkeyRefs      = make(map[string]map[string]string)    // domain -> dereferenced allowed url -> pubkey
	domainTopologyURLs = make(map[string]mushroom.TopologyURL) // cache of parsed domain topology URLs
	zapMetaHandler     = zapMetaHandlerDefault
)

func zapVerbose() bool {
	zapVerboseLock.RLock()
	value := zapVerboseValue
	zapVerboseLock.RUnlock()
	return value
}

func zapVerboseSet(value bool) {
	zapVerboseLock.Lock()
	zapVerboseValue = value
	zapVerboseLock.Unlock()
}

func zapMetaHandlerDefault(version, requestID, domain, address, identity, mechanism string, credentials ...string) map[string]string {
	return map[string]string{}
}

func zapIsIP(addr string) bool {
	if net.ParseIP(addr) != nil {
		return true
	}
	if _, _, err := net.ParseCIDR(addr); err == nil {
		return true
	}
	return false
}

func zapIsAllowed(domain, address string) bool {
	for _, d := range []string{domain, "*"} {
		if a, ok := zapAllow[d]; ok {
			if a[address] {
				return true
			}
		}
	}
	addr := net.ParseIP(address)
	if addr != nil {
		for _, d := range []string{domain, "*"} {
			if a, ok := zapAllowNet[d]; ok {
				for _, m := range a {
					if m.Contains(addr) {
						return true
					}
				}
			}
		}
	}
	return false
}

func zapIsDenied(domain, address string) bool {
	for _, d := range []string{domain, "*"} {
		if a, ok := zapDeny[d]; ok {
			if a[address] {
				return true
			}
		}
	}
	addr := net.ParseIP(address)
	if addr != nil {
		for _, d := range []string{domain, "*"} {
			if a, ok := zapDenyNet[d]; ok {
				for _, m := range a {
					if m.Contains(addr) {
						return true
					}
				}
			}
		}
	}
	return false
}

func zapHasAllow(domain string) bool {
	for _, d := range []string{domain, "*"} {
		if a, ok := zapAllow[d]; ok {
			if len(a) > 0 || len(zapAllowNet[d]) > 0 {
				return true
			}
		}
	}
	return false
}

func zapHasDeny(domain string) bool {
	for _, d := range []string{domain, "*"} {
		if a, ok := zapDeny[d]; ok {
			if len(a) > 0 || len(zapDenyNet[d]) > 0 {
				return true
			}
		}
	}
	return false
}

func zapDoHandler() {
	for {
		msg, err := zapHandler.RecvMessage(0)
		if err != nil {
			if zapVerbose() {
				log.Println("ZAP: Quitting:", err)
			}
			break
		}

		if msg[0] == "QUIT" {
			if zapVerbose() {
				log.Println("ZAP: Quitting: received QUIT message")
			}
			_, err := zapHandler.SendMessage("QUIT")
			if err != nil && zapVerbose() {
				log.Println("ZAP: Quitting: bouncing QUIT message:", err)
			}
			break
		}

		version := msg[0]
		if version != "1.0" {
			panic("ZAP: version != 1.0")
		}

		requestID := msg[1]
		domain := msg[2]
		address := msg[3]
		identity := msg[4]
		mechanism := msg[5]
		credentials := msg[6:]

		username := ""
		password := ""
		clientKey := ""
		switch mechanism {
		case "PLAIN":
			username = msg[6]
			password = msg[7]
		case "CURVE":
			s := msg[6]
			if len(s) != 32 {
				panic("ZAP: len(client_key) != 32")
			}
			clientKey = zmq.Z85encode(s)
		}

		allowed := false
		denied := false

		if zapHasAllow(domain) {
			if zapIsAllowed(domain, address) {
				allowed = true
				if zapVerbose() {
					log.Printf("ZAP: PASSED (whitelist) domain=%q address=%q\n", domain, address)
				}
			} else {
				denied = true
				if zapVerbose() {
					log.Printf("ZAP: DENIED (not in whitelist) domain=%q address=%q\n", domain, address)
				}
			}
		} else if zapHasDeny(domain) {
			if zapIsDenied(domain, address) {
				denied = true
				if zapVerbose() {
					log.Printf("ZAP: DENIED (blacklist) domain=%q address=%q\n", domain, address)
				}
			} else {
				allowed = true
				if zapVerbose() {
					log.Printf("ZAP: PASSED (not in blacklist) domain=%q address=%q\n", domain, address)
				}
			}
		}

		if !denied {
			switch mechanism {
			case "NULL":
				if !allowed {
					if zapVerbose() {
						log.Printf("ZAP: ALLOWED (NULL)\n")
					}
					allowed = true
				}
			case "PLAIN":
				allowed = zapAuthenticatePlain(domain, username, password)
			case "CURVE":
				allowed = zapAuthenticateCurve(domain, clientKey)
			}
		}

		if allowed {
			m := zapMetaHandler(version, requestID, domain, address, identity, mechanism, credentials...)
			userID := ""
			if uid, ok := m["User-Id"]; ok {
				userID = uid
				delete(m, "User-Id")
			}
			metadata := make([]byte, 0)
			for key, value := range m {
				if len(key) < 256 {
					metadata = append(metadata, zapMetaBlob(key, value)...)
				}
			}
			_, _ = zapHandler.SendMessage(version, requestID, "200", "OK", userID, metadata)
		} else {
			_, _ = zapHandler.SendMessage(version, requestID, "400", "NO ACCESS", "", "")
		}
	}

	if err := zapHandler.Close(); err != nil && zapVerbose() {
		log.Println("ZAP: Quitting: Close:", err)
	}
	if zapVerbose() {
		log.Println("ZAP: Quit")
	}
}

func zapAuthenticatePlain(domain, username, password string) bool {
	for _, dom := range []string{domain, "*"} {
		if m, ok := zapUsers[dom]; ok {
			if m[username] == password {
				if zapVerbose() {
					log.Printf("ZAP: ALLOWED (PLAIN) domain=%q username=%q password=%q\n", dom, username, password)
				}
				return true
			}
		}
	}
	if zapVerbose() {
		log.Printf("ZAP: DENIED (PLAIN) domain=%q username=%q password=%q\n", domain, username, password)
	}
	return false
}

func zapAuthenticateCurve(domain, clientKey string) bool {
	for _, dom := range []string{domain, "*"} {
		if m, ok := zapPubkeys[dom]; ok {
			if _, ok := m[curveAllowAny]; ok {
				if zapVerbose() {
					log.Printf("ZAP: ALLOWED (CURVE any client) domain=%q\n", dom)
				}
				return true
			}
			if _, ok := m[clientKey]; ok {
				if zapVerbose() {
					log.Printf("ZAP: ALLOWED (CURVE) domain=%q client_key=%q\n", dom, clientKey)
				}
				return true
			}
		}
	}
	if dynamicAllowsCurve(domain, clientKey) {
		if zapVerbose() {
			log.Printf("ZAP: ALLOWED (CURVE dynamic) domain=%q client_key=%q\n", domain, clientKey)
		}
		return true
	}
	if zapVerbose() {
		log.Printf("ZAP: DENIED (CURVE) domain=%q client_key=%q\n", domain, clientKey)
	}
	return false
}

// Start binds the process-wide ZAP handler on inproc://zeromq.zap.01.
func Start() error {
	if zapInit {
		return nil
	}

	var err error
	zapHandler, err = zmq.NewSocket(zmq.REP)
	if err != nil {
		return err
	}
	_ = zapHandler.SetLinger(0)
	if err = zapHandler.Bind("inproc://zeromq.zap.01"); err != nil {
		_ = zapHandler.Close()
		return err
	}

	zapQuit, err = zmq.NewSocket(zmq.REQ)
	if err != nil {
		_ = zapHandler.Close()
		return err
	}
	_ = zapQuit.SetLinger(0)
	if err = zapQuit.Connect("inproc://zeromq.zap.01"); err != nil {
		_ = zapHandler.Close()
		_ = zapQuit.Close()
		return err
	}

	if err := initTopologyClient(); err != nil && zapVerbose() {
		log.Printf("ZAP: topology client: %v\n", err)
	}

	go zapDoHandler()

	if zapVerbose() {
		log.Println("ZAP: Starting")
	}

	zapInit = true
	return nil
}

// Stop shuts down the process-wide ZAP handler.
func Stop() {
	if !zapInit {
		if zapVerbose() {
			log.Println("ZAP: Not running, can't stop")
		}
		return
	}
	if zapVerbose() {
		log.Println("ZAP: Stopping")
	}
	if _, err := zapQuit.SendMessageDontwait("QUIT"); err != nil && zapVerbose() {
		log.Println("ZAP: Stopping: SendMessageDontwait(\"QUIT\"):", err)
	}
	if _, err := zapQuit.RecvMessage(0); err != nil && zapVerbose() {
		log.Println("ZAP: Stopping: RecvMessage:", err)
	}
	if err := zapQuit.Close(); err != nil && zapVerbose() {
		log.Println("ZAP: Stopping: Close:", err)
	}
	closeTopologyClient()
	if zapVerbose() {
		log.Println("ZAP: Stopped")
	}

	zapInit = false
}

func zapAllowForDomain(domain string, addresses ...string) {
	if _, ok := zapAllow[domain]; !ok {
		zapAllow[domain] = make(map[string]bool)
		zapAllowNet[domain] = make([]*net.IPNet, 0)
	}
	for _, address := range addresses {
		if _, ipnet, err := net.ParseCIDR(address); err == nil {
			zapAllowNet[domain] = append(zapAllowNet[domain], ipnet)
		} else if net.ParseIP(address) != nil {
			zapAllow[domain][address] = true
		} else if zapVerbose() {
			log.Printf("ZAP: Allow for domain %q: %q is not a valid address or network\n", domain, address)
		}
	}
}

func zapDenyForDomain(domain string, addresses ...string) {
	if _, ok := zapDeny[domain]; !ok {
		zapDeny[domain] = make(map[string]bool)
		zapDenyNet[domain] = make([]*net.IPNet, 0)
	}
	for _, address := range addresses {
		if _, ipnet, err := net.ParseCIDR(address); err == nil {
			zapDenyNet[domain] = append(zapDenyNet[domain], ipnet)
		} else if net.ParseIP(address) != nil {
			zapDeny[domain][address] = true
		} else if zapVerbose() {
			log.Printf("ZAP: Deny for domain %q: %q is not a valid address or network\n", domain, address)
		}
	}
}

// AuthAllow whitelists addresses for a domain (see pebbe zmq4 AuthAllow).
func AuthAllow(domain string, addresses ...string) {
	if zapIsIP(domain) {
		zapAllowForDomain("*", domain)
		zapAllowForDomain("*", addresses...)
	} else {
		zapAllowForDomain(domain, addresses...)
	}
}

func zapIsDynamicAllow(domain string) bool {
	for _, d := range []string{domain, "*"} {
		if zapDynamicAllow[d] {
			return true
		}
	}
	return false
}

// AuthDeny blacklists addresses for a domain (see pebbe zmq4 AuthDeny).
func AuthDeny(domain string, addresses ...string) {
	if zapIsIP(domain) {
		zapDenyForDomain("*", domain)
		zapDenyForDomain("*", addresses...)
	} else {
		zapDenyForDomain(domain, addresses...)
	}
}

// AuthPlainAdd registers PLAIN credentials for a domain.
func AuthPlainAdd(domain, username, password string) {
	if _, ok := zapUsers[domain]; !ok {
		zapUsers[domain] = make(map[string]string)
	}
	zapUsers[domain][username] = password
}

// AuthPlainRemove removes PLAIN users for a domain.
func AuthPlainRemove(domain string, usernames ...string) {
	if u, ok := zapUsers[domain]; ok {
		for _, username := range usernames {
			delete(u, username)
		}
	}
}

// AuthPlainRemoveAll removes all PLAIN users for a domain.
func AuthPlainRemoveAll(domain string) {
	delete(zapUsers, domain)
}

func AuthDynamicAllow(domain string) {
	zapDynamicAllow[domain] = true
}

// AuthCurveAdd registers a CURVE client public key (Z85) permitted for a domain.
// When url is given, any existing pubkey for that allowed reference is removed
// first, then the ref and pubkey entries are overwritten for the domain.
func AuthCurveAdd(domain, pubkey string, url ...mushroom.TopologyURL) {
	if _, ok := zapPubkeys[domain]; !ok {
		zapPubkeys[domain] = make(map[string]bool)
	}
	if len(url) > 0 {
		if _, ok := zapPubkeyRefs[domain]; !ok {
			zapPubkeyRefs[domain] = make(map[string]string)
		}
		urlStr := url[0].AsDereference().String()
		if existing, ok := zapPubkeyRefs[domain][urlStr]; ok {
			delete(zapPubkeys[domain], existing)
		}
		zapPubkeyRefs[domain][urlStr] = pubkey
	}
	zapPubkeys[domain][pubkey] = true
}

// AuthCurveRemove removes a CURVE client public key for a domain.
// When url is given, the allowed reference entry is removed as well.
func AuthCurveRemove(domain, pubkey string, url ...mushroom.TopologyURL) {
	p, ok := zapPubkeys[domain]
	if !ok {
		return
	}
	if len(url) > 0 {
		if refs, ok := zapPubkeyRefs[domain]; ok {
			urlStr := url[0].AsDereference().String()
			if pk, ok := refs[urlStr]; ok {
				delete(p, pk)
				delete(refs, urlStr)
			}
		}
	}
	delete(p, pubkey)
	removePubkeyRef(domain, pubkey)
}

func removePubkeyRef(domain, pubkey string) {
	refs, ok := zapPubkeyRefs[domain]
	if !ok {
		return
	}
	for urlStr, pk := range refs {
		if pk == pubkey {
			delete(refs, urlStr)
		}
	}
}

// AuthCurveRemoveAll removes all CURVE client public keys for a domain.
func AuthCurveRemoveAll(domain string) {
	delete(zapPubkeys, domain)
	delete(zapPubkeyRefs, domain)
}

// AuthSetVerbose enables verbose ZAP tracing.
func AuthSetVerbose(verbose bool) {
	zapVerboseSet(verbose)
}

// AuthSetMetadataHandler sets metadata returned on successful ZAP authentication.
func AuthSetMetadataHandler(handler func(version, requestID, domain, address, identity, mechanism string, credentials ...string) map[string]string) {
	zapMetaHandler = handler
}

func zapMetaBlob(name, value string) []byte {
	l1 := len(name)
	l2 := len(value)
	b := make([]byte, l1+l2+5)
	b[0] = byte(l1)
	b[l1+1] = byte(l2 >> 24 & 255)
	b[l1+2] = byte(l2 >> 16 & 255)
	b[l1+3] = byte(l2 >> 8 & 255)
	b[l1+4] = byte(l2 & 255)
	copy(b[1:], []byte(name))
	copy(b[5+l1:], []byte(value))
	return b
}

func initTopologyClient() error {
	client, err := topology.NewClient()
	if err != nil {
		return err
	}
	client.Timeout(zapTopologyTimeout)
	client.Attempt(1)
	zapTopologyClient = client
	return nil
}

func closeTopologyClient() {
	_ = zapTopologyClient.Close()
	zapTopologyClient = nil
}

func dynamicAllowsCurve(domain, clientKey string) bool {
	if clientKey == "" || !zapIsDynamicAllow(domain) {
		return false
	}
	if zapTopologyClient == nil {
		if zapVerbose() {
			log.Printf("ZAP: topology client not initialized\n")
		}
		return false
	}

	handlerURL, ok := domainTopologyURLs[domain]
	var err error
	if !ok {
		handlerURL, err = mushroom.Parse(domain)
		if err != nil {
			if zapVerbose() {
				log.Printf("ZAP: invalid handler url domain=%q: %v\n", domain, err)
			}
		} else {
			domainTopologyURLs[domain] = handlerURL
		}
		return false
	}

	category := handlerURL.HandlerCategory()
	serviceURL := handlerURL.As(mushroom.SERVICE).AsDereference().String()

	if allowedClientPublicKey(serviceURL, category, clientKey) {
		return true
	}

	if err := zapTopologyClient.Reload(); err != nil {
		if zapVerbose() {
			log.Printf("ZAP: topology reload failed domain=%q: %v\n", domain, err)
		}
		return false
	}
	return allowedClientPublicKey(serviceURL, category, clientKey)
}

func allowedClientPublicKey(serviceURL, category, clientKey string) bool {
	svc, err := zapTopologyClient.Service(serviceURL)
	if err != nil {
		if zapVerbose() {
			log.Printf("ZAP: topology Service(%q): %v\n", serviceURL, err)
		}
		return false
	}
	return mushroom.IsAllowedClientPublicKey(&svc, clientKey, category)
}
