# Rules for handshaking branch based services

Given: *6 services* in a one topology defined by the hello-world:

(t) topology of hello-world app

1. (h) hello-world independent service
  inbound: hello <- upper-case.main.hello
  outbound: nil
3. (u) upper-case proxy
  inbound: hello <- default-name.main.hello
  outbound: hello -> hello-world.main.hello
4. (d) default-name proxy
  inbound: nil
  outbound: hello -> upper-case.main.hello

### Workflow

1. Service is starting:
  Read the topology of (h), since all other topoliges are empty.
  create `(topologyInbounds)` where it has following:
  h)
     (h).main.hello <- (u).main.hello
  u)
     (u).main.hello <- (d).main.hello
  - [expected] Total 2 inbounds for 3 services
   Create `topologyOutbounds` where it has following:
   (d)
     (d).main.hello -> (u).main.hello
   (u)
     (u).main.hello -> (h).main.hello
   -- [expected] Total 2 outbounds for 3 services
   Create `(topologyDeps[(h)])` where it has following:
   (e)
   (m)
   (d)
   (u)
   (h)
   -- [expected] Total 3 dependencies for 3 services
   Create publicKeys[handlerURL] = pubKey
   Create `serviceStatus[context: (h)]`:
   (d) => ServiceStatus{
     managerCon => {
        status: `ConnectionStatus`=`not-set`
        client: loadClient((d).manager.endpoint)
        hmac: message.GenerateSecret()
        publicKey: (d).parameters.public-key = ""
        lockGoroutine: nil
        hash: ""
     },
     inboundCons: nil
     outboundCons: {
        (u).manager -> {
            status: `*serviceStatus[context: h].u.inboundCons[u].status`,
            publicKey: publicKeys[context: h].u.manager
        }
     },
     inboundEdges: nil
     outboundEdges: {
     `*serviceStatus[context: h].u.inboundEdges`
         m.main.any: {
             e.main.any,
             status
         }
     }
     inbounds: nil
     outbounds: nil
   }
   (u) => ServiceStatus{
     managerCon => {*(e.managerCon)},
     inboundCons: {
     (d).manager <- {
         status: `not-set`
         hash: ""
     }
     }
     outboundCons: nil,
     inboundEdges: {
        (u).main.hello: []{
            inboundURL: (d).main.hello
            status: `not-set`
            hash: ""
        }
     }
     outboundEdges: nil
     inbounds: nil
     outbounds: {
        (u).main.hello: []{
            outboundURL: (h).main.hello
            status: `not-set`
            hash: ""
            hmac
        }
     }
   }
   -- [expected] the initial status
   -- for every dep, go through the inbounds and prepare in itself
   -- for every dep go through the outbounds and prepare in itself
       at the end the main handler's public key must be in the service too.
       at the end for every inbound is h, it needs to prepare hmac too.

// Requires:
// topologyNode.IsServiceRunning(service-url) // return nothing
// topologyNode.ServiceStatus(service-url, context-url, status-req)
2) Handshake
    For each context // for contextURL, contextServices := range serviceContexts
        For each dep, run the goroutine in parallel: // for depURL, depService := range contextServices
            handshakeDep(contextURL, depURL, depService) // h.manager, d.manager, contextServices[h]
            handshakeDep(contextURL, depURL, depService) // h.manager, u.manager, contextServices[e]

// for every inbounds, and outbounds it has to set them internally.
3) handshakeDep(context: h, serviceURL: depURL: e, *status StatusReq)
    if !isHandshakeable() { // false
        return
    }
    if status.managerCon.lock
        status.managerCon.status.Tick()
        return
    status.managerCon.lock = true
    go {
        connectManager(h, e, status.managerCon)
        status.managerCon.lock = false
        if err:
            print warning
            return
        else:
            statusRep = checkStatus(h, h, statusReq)
            if err == curve-err | access-err:
                managerCon.status = err
                return

```
        if status.inbounds == nil
                skip
        else if inbounds.lockGoroutine == true
            status.inbounds.status.Tick()
            skip
        else
            status.verifyInbounds(statusRep.inbounds)
                
        if outbounds == nil
            skip
        else if outbounds.lockGoroutine == true
            status.outbounds.status.Tick()
            skip
        else
            status.verifyOutbounds(statusRep.outbounds)
                
        //-----------------------------//
        // Dep to dep
        //-----------------------------//

        if inboundCons nil                  // for e, skip
            skip
        else if inboundCons.lockGoroutine
            status.inboundCons.Tick()
            skip
        else
            verification = status.verifyInboundCons(status.inboundCons)
            update inboundCons

        if outboundCons nil
            skip
        else if outboundCons.lockGoroutine
            status.outboundCons.Tick()
            skip
        else
            verification = status.verifyOutboundCons(status.outboundCons, statusRep.outboundCons)
            update outboundCons
    }()
```

//  Variation with syncing outboundCons

#### d.handshake:

```
for each context: h
    for each dep (m): // its not handshakeable
        managerCon: not-set
        inbounds: nil
        outbounds: 1
        outboundCons: nil
        outboundEdges: nil
        inboundCons: nil
```
#### u.handshake

```
dep (h):
    is handshakeable? false
    inbounds: nil
    outbounds: 1
dep (d):
    is handshakeable? true
    inbounds: 1
    outbounds: nil
```

// flow:

h -> u
    d.onCheckStatus(inboundCons: 1, outbounds: 1)
    h.secureOutbounds
    h.secureInboundEdges[d]
        create context[h].deps[d] with inbounds
    u.handshake -> d
        connectManager[d]
        u.secureOutbounds[d]
h -> d
    d.onCheckStatus(outboundCons: 1) returns not-found
    m.secureOutbounds[d] adds the status
    d.onCheckStatus() returns the status or hash

---

#### d.checkStatus(h, h, status)

```
#### d.handshake(h, h, hmac, status)

#### d.allowInDep()

###### whitelistInDep(h, e, statusReq):

```
generate hmac if not presented.
add to client socket the hmac.
create signature and add to parameters
request handshake(service-url: h, context-url: h, hmac, signature, command: 'whitelist-in-dep')
return error
```



###### allowInDep(h, d, statusReq):

```
check is e's allowed h's manager curve key?
if not set it.
check is e's public-key mismatch the one in publicKeys[]? If so, update it.
```

#### isHandshakeable()

```
return len(status.inbounds) > 0 || len(status.outboundCons) > 0
    || len(status.inboundCons) > 0
```

#### verifyInboundCons(status)

```
inbounds = []
.... same as 
```

#### verifyOutboundCons(status, statusRep.outboundCons) // for e

```
outbounds = []
for each statusRep.outboundCons:
    is outboundCons.status connected? skip  // not-set
    if m.managerCon.status == connected? skip // not yet connected, so skip
    if m.managerCon.status == connected?
        add to outbounds[m] = {publicKey, routeURL: e.main.any, outboundURL: m.main.any} 
if outbounds[] == empty
    return statusTick
else:
    lockGoroutine
    status.managerCon.client.setOutboundEdges(contextURL: h, outbounds)
    unlockGoroutine
```

#### checkStatus(contextURL: h, serviceURL: h, checkStatus)

```
// onCheckStatus

// for e
first check outboundCons
outboundCons[m] not exists in internal status, so add with initial status.
else if outboundCons exists return its status

check outboundEdges
outboundEdges[main.any] not exists in internal status, so add with initial status.

set publicKeys in context as well.

// for m
check outboundCons
outboundCons[d] not exists in internal status so add with initial status

// for inbounds, it checks is it secure or not.
// if inboundCons is set in internal status, then set inbound checks too.
...
```



#### client.setOutboundEdges(contextURL: h, outbounds)

```
// for e
for each outbounds: // e.main.any
    check is publicKeys if not equal set outbounds.publicKey
    check the outboundCon, and add the public key to allowance to its manager. Update managerCon.
    check the outbounds and add them.
return status
```



### Data types



##### ServiceStatus

```
managerCon:
    status: ConnectionStatus // step | hash
    hmac: Secret Key
    client: *topology.Client
    mutex
    handshake()
pubKeys: map[handlerURL]: publicKey
inbounds:
    map[routURL]SocketStatus
```

SocketStatus:
    hmac
    status
    mutex
    check()

##### ServiceStatusReq

```
inbounds: map[routeURL]InboundStatusMsg // inboundURL, status
outbounds: map[routeURL]OutboundStatusMsg // outboundURL, status
outbound-cons: map[serviceURL]status
inbound-cons: map[serviceURL]status
outbound-edges: map[routeURL]OutboundStatusMsg
inbound-edges: map[routeURL]InboundStatusMsg
```

ServiceStatusReq(default-name proxy): // from tutorial/010-security in showcase
    manager-con: status
    inbounds:
        nil
    outbounds:
        nil
    inbound-cons:
        map(metric) -> status
    outbound-cons:
        map(upper-case) -> status
    inbound-edges:
        map(default-name.main.hello) -> inboundURL: metrics.main.hello, status
    outbound-edges:
        map(default-name.main.hello) -> outboundURL: upper-case.main.hello, status

ServiceStatusReq(metrics proxy): // from tutorial/010-security in showcase
    inbounds:
        nil
    outbounds:
        map(metrics.main.age) -> outboundURL: hello-world.main.age, status
        map(metrics.main.country) -> outboundURL: hello-world.main.country
    inbound-cons:
        nil
    outbound-cons:
        map(default-name): status
    outbound-edges:
        map(metrics.main.hello) -> outboundURL: default-name.main.hello
    inbound-edges:
        nil

ServiceStatusReq(ai extension):
    inbounds:
    outbounds:
        nil
    manager-con: status
    inboundCons:
        nil
    outboundCons:
        nil
    inboundEdges:
        nil

ServiceStatusReq(default-name proxy): // from tutorial/003-default-name-proxy in showcase
    inbounds:
        nil
    outbounds:
        map(default-name.main.hello) -> {outboundURL: hello-world.main.hello, status: ServiceStatusStep}
    ...

ServiceStatusReq(default-name proxy): // from tutorial/010-security in showcase
    manager-con: status
    inbounds:
        nil
    outbounds:
        nil
    inbound-cons:
        map(metric) -> status
    outbound-cons:
        map(upper-case) -> status
    inbound-edges:
        map(default-name.main.hello) -> inboundURL: metrics.main.hello, status
    outbound-edges:
        map(default-name.main.hello) -> outboundURL: upper-case.main.hello, status

ServiceStatusReq(metrics proxy): // from tutorial/010-security in showcase
    inbounds:
        nil
    outbounds:
        map(metrics.main.age) -> outboundURL: hello-world.main.age, status
        map(metrics.main.country) -> outboundURL: hello-world.main.country
    inbound-cons:
        nil
    outbound-cons:
        map(default-name): status
    outbound-edges:
        map(metrics.main.hello) -> outboundURL: default-name.main.hello
    inbound-edges:
        nil

#### Make the live updates possible using topology config

For hardcoded service config, do not update the:

- parameters instead overwride.
- handler-deps is not touched.
- for proxy do not touch outbounds instead over-write.
- for extension do not touch inbounds instead over-write.

For hardcoded handler config, do not update the:

- command-deps instead overwrite unless user didn't pass one.

