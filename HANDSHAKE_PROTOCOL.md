# Rules for handshaking branch based services

Given: *6 services* in a one topology defined by the hello-world:

(t) topology of hello-world app

1. (h) hello-world independent service
 outbound: manager.any -> ai.main.any
 inbound: hello <- upper-case.main.hello, age <- metrics.main.age
2. (a) ai extension
 inbound: any <- hello-world.manager.any
 outbound: nil
3. (u) upper-case proxy
 inbound: hello <- default-name.main.hello
 outbound: hello <- hello-world.main.hello
4. (d) default-name proxy
 inbound: hello <- metrics.main.hello
 outbound: hello -> upper-case.main.hello
5.(m) metrics proxy
 inbound: any <- entrypoint.main.any
 outbound: hello -> default-name.main.hello; age -> hello-world.main.age
6. (e) entrypoint proxy
 outbound: any -> metrics.main.any

## TODO:

#### Is-Service-Running is renamed to status

So it returns the `ServiceStatusRep`. Currently it returns running = true.

Request Parameters:

- service-url - service that calls it.
- *optional* context-url - service that sets the connection.
- `ServiceStatusReq`

Returns:

- `ServiceStatusRep` embeds `ServiceStatus`

### Workflow

1) Service is starting:
    Read the topology of (h), since all other topoliges are empty.
    Create `(topologyInbounds)` where it has following:
        (h)
            (h).main.hello <- (u).main.hello
            (h).main.age <- (m).main.age
        (a)
            (a).main.any <- (h).manager.any
        (m)
            (m).main.any <- (e).main.any
        (d)
            (d).main.hello <- (m).main.any
        (u)
            (u).main.hello <- (d).main.hello
    -- [expected] Total 5 inbounds for 6 services

    Create `topologyOutbounds` where it has following:
        (e)
            (e).main.any -> (m).main.any
        (m)
            (m).main.hello -> (d).main.hello
            (m).main.age -> (h).main.age
        (d)
            (d).main.hello -> (u).main.hello
        (u)
            (u).main.hello -> (h).main.hello
        (h)
            (h).manager.any -> (a).main.any
    -- [expected] Total 5 outbounds for 6 services

    Create `(topologyDeps[(h)])` where it has following:
        (e)
        (m)
        (d)
        (u)
        (h)
    -- [expected] Total 5 dependencies for 6 services

    Create publicKeys[handlerURL] = pubKey
    Create `serviceStatus[context: (h)]`:
        (e) => ServiceStatus{
            managerCon => {
                status: `ConnectionStatus`=`not-set`
                client: loadClient((e).manager.endpoint)
                hmac: message.GenerateSecret()
                publicKey: (e).parameters.public-key = ""
                lockGoroutine: nil
                hash: ""
            },
            inboundCons: nil
            outboundCons: {
                (m).manager -> {
                    status: `*serviceStatus[context: h].m.inboundCons[e].status`,
                    publicKey: publicKeys[context: h].m.manager
                }
            },
            inboundEdges: nil
            outboundEdges: {
                `*serviceStatus[context: h].m.inboundEdges`
                    m.main.any: {
                        e.main.any,
                        status
                    }
            }
            inbounds: nil
            outbounds: nil
        }
        (m) => ServiceStatus{
            managerCon => {*(e.managerCon)},
            inboundCons: {
                (e).manager <- {
                    status: `not-set`
                    hash: ""
                }
            }
            outboundCons: *serviceStatus[context: h].d.inboundCons[m] & d.public-key,
            inboundEdges: {
                (m).main.any: []{
                    inboundURL: (e).main.any
                    status: `not-set`
                    hash: ""
                }
            }
            outboundEdges: *serviceStatus[context: h].d.inboundEdges[m],
            inbounds: nil
            outbounds: {
                (m).main.age: []{
                    outboundURL: (h).main.hello
                    status: `not-set`
                    hash: ""
                    hmac
                }
            }
        }
        (d) => ServiceStatus{
            managerCon => {*(e.managerCon)},
            inboundCons: {
                (m).manager <- {
                    status: `not-set`
                    hash: ""
                }
            }
            outboundCons: *serviceStatus[context: h].u.inboundCons & publicKeys[u]
            inboundEdges: {
                (d).main.hello: []{
                    inboundURL: (m).main.hello
                    status: `not-set`
                    hash: ""
                }
            }
            outboundEdges: *serviceStatus[context: h].u.inboundEdges
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
        (a) => ServiceStatus{
            managerCon => {*(e.managerCon)},
            inboundCons: nil
            outboundCons: nil,
            inboundEdges: nil
            outboundEdges: nil
            inbounds: {
                (a).main.any: []{
                    inboundURL: (h).manager.any
                    status: `not-set`
                    hash: ""
                    hmac
                }
            }
            outbounds: nil
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
            handshakeDep(contextURL, depURL, depService) // h.manager, e.manager, contextServices[e]
            handshakeDep(contextURL, depURL, depService) // h.manager, m.manager, contextServices[e]
            handshakeDep(contextURL, depURL, depService) // h.manager, d.manager, contextServices[e]
            handshakeDep(contextURL, depURL, depService) // h.manager, u.manager, contextServices[e]
            handshakeDep(contextURL, depURL, depService) // h.manager, a.manager, contextServices[e]
    
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

//  Variation with syncing outboundCons
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

            // Update data
            set status to contextServices[context].service = status
        }()

3.1) handshakeDep(h, m, status):
    // ...connectManager
    checkStatus
    inbounds == nil, skip
    outbounds has 2:
        verifyOutbounds(h, m, status) // m.main.age -> h.main.age, m.main.country -> h.main.country
    inboundCons has 1:
        verifyInboundCons()  // e, not-set, e.main.any -> m.main.any
---
    before status:
    services[m] {
        inbounds: nil
        outbounds: {
            h.main.age: not-set, publicKeys[h.main] = '', outbounds.hmac,
            h.main.country: not-set, publicKeys[h.main] = '', outbounds.hmac
        },
        inboundCons: {
            m.main.any: {
                e.main.any: not-set,
                managerCon: 'not-set'
            }
        },
        outboundCons: {
            h.main.hello: {
                managerCon: not-set,
                d.main.hello: not-set
            }
        }
    }
    after status:
    services[m] {
        inbounds: nil
        outbounds: {
            status: hash
            status: hash
        }
        inboundCons: {
            m.main.any: {
                not-set
                managerCon
            }
        },
        outboundCons: {
            h.main.hello: {
                not-set
                d.main.hello: not-set
            }
        }
    }
    // internally, the m starts the handshake but first generates hmac and secret for inbound.
    // and for the outside as well.

    // I found issue, its how to share the public keys between the sockets.

#### e.handshake:
    for each context: h
        for each dep (m): // its not handshakeable
            managerCon: not-set
            inbounds: nil
            outbounds: 1
            outboundCons: nil
            outboundEdges: nil
            inboundCons: nil
#### m.handshake:
    for each context: h
        dep (h):
            if handshakeable? skip
            managerCon: not-set
            inbounds: nil
            outbounds: 1
            outboundCons: nil
            outboundEdges: nil
            inboundCons: nil
        dep (e):
            if handshakeable? yes
            managerCon: not-set
            inbounds: e.main.any -> m.main.any
        dep (d):
            if handshakeable? skip
            inbounds: nil
            outbounds: 1
            outboundCons: nil
#### a.handshake:
    for each context: a
        dep (m):
            is handshakeable? true
            managerCon: not-set
            inbounds: 1
            outbounds: nil
#### d.handshake:
    dep (m):
        is handshakeable? true
        managerCon
        inbounds: 1
        outbounds: nil
    dep (u):
        is handshakeable? false
        managerCon: not-set
        inbounds: nil
        outbounds: 1
#### u.handshake
    dep (h):
        is handshakeable? false
        inbounds: nil
        outbounds: 1
    dep (d):
        is handshakeable? true
        inbounds: 1
        outbounds: nil

// flow:
h -> m
    m.onCheckStatus(outbounds: 2, inboundCons: 1, outboundCons: 1), returns not-found, not-found, not-found.
    h.secureOutbounds(context: h, outbounds: 2 + public keys, hmacs)
    m.onSecureOutbounds:
        add outbounds in status
        for m.main.country -> h.main.country, add client
        for m.main.age -> h.main.age, add client
        add to public keys the m.main
        generate the hash(hmac, route, outbound, route public, outbound public) and return it.
    h.secureInboundEdges[m] returns public keys that it sets internally
    m.onSecureInboundEdges
        context[h].deps[e] = generate hmac, get public-key for m.main and add to publicKeys.
            + inbounds: m.main.any <- e.main.any
    m.onCheckStatus returns not-found for outbound cons, but sets connecting to inbounds. and hash + public keys for outbounds.
    m.handshake -> e
        connectManager[e]
        m.handshakeInbound[e]
    m.handshakeInbound[e]:
        e.secureOutbounds[e]
    e.onSecureOutbounds:
        context[h].dep[m].outbounds = {}
        add to public keys the e.main

    m.onCheckStatus returns (inboundCons: hash, outboundCons: not-found, outbounds: hash)
h -> d
    d.onCheckStatus(inboundCons: 1, outboundCons: 1), returns inboundCons:not-found, outboundCons: not-found
    d.secureInboundEdges[m] adds to d.handshake, generates hmac
    d.onCheckStatus returns not-found for outbounds, but sets connecting to inbounds
    d.handshake -> m
        send the hmac, and updates the inbound status
        d.secureOutbounds[m]
        calculate the hash and set in status
    d.onCheckStatus returns (inboundCons: connecting)
    d.onCheckStatus returns (inboundCons: hash)
h -> u
    d.onCheckStatus(inboundCons: 1, outbounds: 1)
    h.secureOutbounds
    h.secureInboundEdges[d]
        create context[h].deps[d] with inbounds
    u.handshake -> d
        connectManager[d]
        u.secureOutbounds[d]
h -> e
    e.onCheckStatus(outboundCons: 1) returns not-found
    m.secureOutbounds[e] adds the status
    e.onCheckStatus() returns the status or hash
h -> a
    a.onCheckStatus(inbounds: 1) returns not-found
    h.secureInboundEdges[a]
        connectManager[h]
        a.secureOutbounds[h]
    h.onSecureOutbounds
        since its context and service are matching
            it secures the context[h].dep[a].inbounds = {}

---
what if m is crashed
h -> m
    connectManager sets not-running
    skips
h -> e
    checkStatus(outboundCons: *m.inboundCons)
    e.onCheckStatus
        if outboundCons.status != context.status
            update the context.status to not-running
d -> m
    connectManager sets not-running
    skips
h -> d
    checkStatus(inboundCons: 1) returns not-running

m is spawned
h -> m          // new 2 public keys
    connectManager
    checkStatus(outbounds: 2, inboundCons: 1, outboundCons: 1) // not-found
    if status not-found secureOutbounds
    h.secureInboundEdges[m] // updates the m.main public key
    m.handshake[e]
        send the m.main public key and rewrite it in the e.
h -> d
    checkStatus(inboundCons: 1)
    onCheckStatus says its mismatch
    so, d.secureInboundEdges:
        it updates the information.
    d.handshake -> m
        checkStatus(outbounds: *d.inbounds)
        secureOutbounds
----
#### e.checkStatus(h, h, status)
    context = h
    caller = h
    status: outboundCons: 1
    check is context[h].deps[outboundCons[0]] = 0 exists in local?
    no, add // so now it will have the context[h].deps[outboundCons[0]]
    // since its added, prepare the outbounds.\

    // what to do with the status?

#### e.handshake(h, h, hmac, status)
#### e.allowInDep()

###### whitelistInDep(h, e, statusReq):
    generate hmac if not presented.
    add to client socket the hmac.
    create signature and add to parameters
    request handshake(service-url: h, context-url: h, hmac, signature, command: 'whitelist-in-dep')
    return error

###### allowInDep(h, e, statusReq):
    check is e's allowed h's manager curve key?
    if not set it.
    check is e's public-key mismatch the one in publicKeys[]? If so, update it.

// Has to be fast
##### connectManager(h as serviceURL, e as depURL, status.managerCon) (running, status.managerCon, err)
    is managerCon.status == in (not-set || access-err)
        call whitelistInDep(h, e, statusReq); err:
            if err:
                set status
                if access-err:
                    return false, managerCon, err
                connectManager()
            else
                status.managerCon = connected
                return running, status.managerCon, err
    else if managerCon.status == curve-err
        allowInDep(h, e, statusReq)
        if err:
            set status
            if curve-err:
                return false, managerCon, err
            return connectManager()
        else
            set status = not-set
            return connectManager()
    else if connected
        status.tick
        return

// either it has inbounds, or it has dependency connection it needs to keep up
#### isHandshakeable()
    return len(status.inbounds) > 0 || len(status.outboundCons) > 0
        || len(status.inboundCons) > 0


---
What happens if m is down, after all is started?

// how to detect it?
h.handshake -> m
    sends checkStatus() it gets back as err-not-running
    so it sets status to not-running and skips remaining
    for each m.inboundCons: // e
        set manager status to not-running
d.handshake -> m
    sends checkStatus() it gets back as err-not-running
    set its status to not-running.
h.handhsake -> e
    sends checkStatus()
        it sees the status to outbounds is disconnected, while internally its connected.
        so, it updates the information, since the publicKey is the same.

m is up.

h.handshake -> m
    it fixes the connection.
    then tells to m, checkStatus:
        outbounds: status, hash // h.main.age: {}, h.main.country: {}
        outboundCons: 1 // m.main.hello -> {d.main.hello, d.manager.public-key}
        inboundCons: 1 // m.main.any <- {e.main.any, e.manager.public-key} 
        inbounds: nil
    checkStatus returns says its not set.
        call verifyOutboundCons(d.public-key) // sets proxified handler.
        call verifyInboundCons by passing the public-key of the e
        call verifyOutbounds() by passing the hmac
h.handshake -> e
    sends checkStatus(publicKey of m, we don't store the outbound status, instead we get what was returned by m which is curve-err)
    it sees its mismatch of the curve key, so sets the outbound client.
m.handshake -> e
    first attempt returns curve no key.
    second attempt returns connected, but access-err, so it initiates the hmac.
    third one sends status, and stores it internally and returns to client.
d.handshake ->
    first attempt returns not-running
    second attempt returns err-no-key
m.checkStatus -> e
    sends the internally generated hash as the status + m.main.public-key
    e sees the public-key changed so updates it. Then e tries to see hash it mismatches
    so it sets the status internally hash-mismatch
    it sees hash-mismatch and calls the m.secureInbounds(e, m.main.any, hmac, publickey)
d.checkStatus -> m
    sends the internally generated hash as the status + m.main.public-key
    m sees the public key set so skips it.
    d sends hash that it checks m.outbounds mismatches since internally its different, so tells
    its

e depends on m, e's checkStatus returns all-set

#### verifyOutbounds(h, m, status)
    // first check the outbound

#### verifyInboundCons(status)
    inbounds = []
    .... same as 

// whitelistManagerInDeps in manager.go
#### verifyOutboundCons(status, statusRep.outboundCons) // for e
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

#### checkStatus(contextURL: h, serviceURL: h, checkStatus)
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

#### client.setOutboundEdges(contextURL: h, outbounds)
    // for e
    for each outbounds: // e.main.any
        check is publicKeys if not equal set outbounds.publicKey
        check the outboundCon, and add the public key to allowance to its manager. Update managerCon.
        check the outbounds and add them.
    return status

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

