# noPerfection's topology protocol
This file describes the rules on how noPerfection services communicate to other services to authenticate, encrypt and manage services according to the command dependency and handler dependency. Service managing proxies, and extensions is called topology and this file explains topology's protocol.

**Intended for maintainers, and for those who wants to implement in other programming languages**

All topology work is handled in the service's manager. Its implementation will be in this source code's [manager directory](./manager/).

## Credentials of this protocol
Author: Medet Ahmetson [medet@ara.foundation]
Version: 1.0.0
Date: 14 oct. 2026

# Handshake
**Handshake** is the process of the service's communicating to other service via managers.
The handshake implementation is in this source code's [manager/handshake.go](./manager/handshake.go) file.

Handshake is running in the goroutine every 5 seconds to keep alive the services and if they crash, then to revive encrypted management.

**Is it possible to change the 5 seconds?** Not implemented, at least for now.

Topology management goes in the handshake and repetitive, so it will be omitted about repititon, but keep in mind its every 5 seconds.

Handshake has two phases:
1. Establishing connection to the managers
2. Evaluating status of the topology and based on status, doing specific actions to keep connections healthy.

## Handshake data types
Every manager creates a topology context, where context key is the service url, and its value is the Proxies and Extensions in the context. Since proxies and extensions might be connecting to each other but its relationship defined by the one single service, the context is that single to track the awareness in indirect connections.

That context organizer is called **root** service.
The proxies and extensions are called **dependency** services.

## Phase 1) connecting managers
Its done using connect manager which is working only between root and dependency.
But it needs to be done between dependency to dependency as well.

Defined in [manager/handshake.go?func=connectManager()](./manager/handshake.go).

### Algorithm
if `STATUS_MANAGER_UNSET` || `STATUS_MANAGER_NO_WHITELIST`, then call `whitelistCaller`.
    we assume that public keys are set already therefore we try to whitelist it.

if `STATUS_MANAGER_NOT_AUTH`, then reload the topology then call `allowCaller`
    ## Todo: if service and context mismatch then set the error to the STATUS_MANAGER_ERR.
# todo: if `STATUS_MANAGER_ERR`, then it skips the message by simply ticking it.
in any other cases we probe a request to see is remote service is running and accessible or not.
    if yes, we set it as connected.
    otherwise, we see why its not available, and set the status. And simply return.

It also tracks the status, and if after connection manager the status changes from connected to anything else it resets the inbound connection or outbound connection as well so dependency to dependency connection will be reset as well.
And if it has inbound connection, then before resetting it, it will reset the inbound dependency's outbound connection as well.

## Phase 2) evaluating status of the topology
Each topology context stores information between **root** and a **dependency**. Its defined in the `ServiceStatus`.
The **root** during the handshake converts **serviceStatus** to `ServiceStatusMsg` and sends to **dependency** in the `EvaluateStatus` request. The handling of the status evaluation is defined in the [EvaluateStatus](#evaluatestatus-handle-function) section.

The `ServiceStatusMsg` drops from the service status the manager connection which holds phase 1 information, drops the secret keys, private keys and the dependency URL. Secret keys is sensitive so its not constantly shared and dependency url is already known by dependency itself and its just unused.

The dependency returns back the service status message by comparing it into its own internal status. Dependency evaluates it and returns back the result. The handshake then has to follow up action depending on the evaluated result.

The status has 6 mapping. Follow up is different for each mapping.

### Status 1. outbounds
The outbounds is the **root** as the outbound end that **dependency** sends messages too. In other words **root** receives message from **dependency**.

Outbounds is per route between handlers as the route URL mappings.

##### Algorithm
Given: independent `i(h)` with proxy `p(u)`.
1. `i(h)` calls `EvaluateStatus` and sends outbounds.
2. `p(u)` evaluates and returns result back. Since its first call and `p(u)` doesn't know about any outbounds it returns outbounds status as `STATUS_ROUTE_NOT_FOUND`.
3. `i(h)` calls `secureOutbounds`; *if status is STATUS_ROUTE_NOT_FOUND only*; `p(u)` calls `onSecureOutbounds`. Upon successful return and comparing its generated hash with its own, it sets status as `STATUS_ROUTE_SECURED`.

#### SecureOutbound
Is establishing the hmac and whitelisting for the outbound defined in outbound status.

It runs in a goroutine.
If `i(h)` own status for outbounds is `STATUS_ROUTE_CHECKING` skips otherwise sets this status.

Sends the outbound route URL mapping along the public key of root's handler, and hmac the root generated for this route.

It uses the following format

```
outbounds[] = 
    dependency route url => (
        dependency url: root route url,
        status: status, by default `NOT_SET`
    )
```

#### OnSecureOutbound
Initiates own topology context but inverts the way how it stores data. 

It stores outbounds as inbounds of the root in the following format

```
d = topology context [i(h)][i(h)]

d.inbounds[i(h)'s route] = (status, p(u)'s route etc)
```

It inverts the dependency's outbound send by root, to the root's inbound in the dependency.

For proxies it adds the outbound credentials to the proxy's handler.
For extensions it adds them to the npac (so any client will have it automatic authentication)

##### Why invert in the topology
Because of the principle of the noPerfection's design. Every service treats topology from its own point of view.

**EVERY SERVICE HAS OWN POINT OF VIEW**

With this, whether its own topology or in a topology context the algorithm remains the same.
It also makes noPerfection services as p2p. Same code everywhere in any topology.

### Status 2. inbounds
The inbounds is where dependency receives message from the root.
It's applied in the extensions, or proxy to proxy, or extension to extension.

### Status 3. outbound connections
It's when dependency A sends message to dependency B within the context of the root.
Root evaluating the outbound connections targets the dependency A.

In noPerfection route to route security goes from inbound to outbound. It means dependency B will generate and store credentials and send it to the dependency A.

outbound connecting securing lets know dependency A about the public key of the dependency B that root knows. And dependency A must allow its connection and simply waits it.

#### Why from inbound to outbound?
To avoid confusion in the distributed topology. Since all follow the same rule its easier to debug and restore the missing connections.

#### Algorithm
Given: independent `i(h)` with proxy `p(u)`, `p(d)`, where `p(d)->p(u)`.
It means the `p(d)` has outbound connection pointing to `p(u)`.
1. `i(h)` calls `EvaluateStatus` and sends outbounds connections.
2. `p(d)` evaluates and returns result back. Since its first call and `p(d)` doesn't know about any outbound connections. it returns outbound connection status as `STATUS_MANAGER_NOT_FOUND`.
3. `i(h)` calls `secureOutboundConnection`; *if status is STATUS_MANAGER_NOT_FOUND only*; `p(d)` calls `onSecureOutboundConnection`. Upon successful return sets status as `STATUS_MANAGER_WAITING`.

#### SecureOutboundConnection
Is lets know the dependency a private key that it allows other dependency to access it within the context of the root only.

It runs in a goroutine.
If status from `p(d)` is `STATUS_MANAGER_WAITING` then it skips it.
If `i(h)` own status for outbound connection is `STATUS_MANAGER_CHECKING` skips otherwise sets it

It gets the public key of `p(u)`: if `p(u)`'s manager status is not secured it returns an nil instead public key. In this case secure outbound connection halts by setting status to `STATUS_MANAGER_UNSET`.

It uses the following format

```
outbound connection = 
    p(d)'s manager url => (
        dependency url: p(u)'s manager url,
        status: status, by default `STATUS_MANAGER_UNSET`
    )
```

#### OnSecureOutboundConnection
Initiates own topology context but inverts the way how it stores data. 

It stores outbound connection as the connection manager with the client as nil in a following format

```
d = topology context [i(h)][p(u)]

d.manager connection = (
    client = nil, 
    status = unset,
    public key
)
```

It sets the status to `STATUS_MANAGER_WAITING`
It inverts the dependency's outbound send by root, to the root's inbound in the dependency.

For proxies it adds the outbound credentials to the proxy's handler.
For extensions it adds them to the npac (so any client will have it automatic authentication)

#### How dependency updates the status of outbound connection?
Dependency with the outbound connection updates the status of outbound connection in two ways.

1. If it receives whitelist caller reqiest from outbound itself.
The outbound dependency in whitelisting request sends context = i(h), service = (outbound dependency), hmac 
generated by outbound dependency.

The dependency with outbound connection upon receiving the whitelist caller sets the status to `STATUS_MANAGER_SECURED`.

It also computes the connection hash, if after security for some reason it changes the parameter and it stops
to use the authentication credentials.

### Status 4. inbound connections
It's when dependency B receives a message from a dependency A within the context of the root.
Root evaluating the inbound connections targets the dependency B.

It's the pair with the [outbound connections](#secureoutbound). See also [why inbound to outbound?](#why-from-inbound-to-outbound) since it explains relationship between these two pairs.

#### Algorithm
Given: independent `i(h)` with proxy `p(u)`, `p(d)`, where `p(d)->p(u)`.
It means the `p(u)` has inbound connection pointing to `p(d)`.
1. `i(h)` calls `EvaluateStatus` and sends inbound connection.
2. `p(u)` evaluates the status and returns result back. Since its first call and `p(u)` doesn't know about any inbound connections. it returns inbound connection status as `STATUS_MANAGER_NOT_FOUND`.
3. `i(h)` calls `secureInboundConnection`; *if status is STATUS_MANAGER_NOT_FOUND only*; `p(u)` calls `onSecureInboundConnection`. Upon successful return sets status as `STATUS_MANAGER_WAITING`.

#### SecureInboundConnection
Is establishing the hmac and whitelisting for the inbound defined in inbound connection status.

It runs in a goroutine.
If status from `p(u)` is NOT `STATUS_MANAGER_WAITING` then it skips it.
If `i(h)` own status for inbound connection is `STATUS_MANAGER_CHECKING` skips otherwise sets it

It gets the public key of `p(d)`: if `p(d)`'s manager status is not secured it returns a nil instead public key. In this case secure outbound connection halts by setting status to `STATUS_MANAGER_UNSET`.

It uses the following format

```
inbound connection = 
    p(u)'s manager url => (
        dependency url: p(d)'s manager url,
        status: status, by default `STATUS_MANAGER_UNSET`
        public key:
        endpoint: p(d)'s endpoint
    )
```

#### OnSecureInboundConnection
Initiates own topology context but inverts the way how it stores data. 

It stores inbound connection as the connection manager with the client in a following format

```
d = topology context [i(h)][p(d)]

d.manager connection = (
    client = new form, 
    status = unset,
    public key
)
```

##### avoiding data race in case overwriting
before storing the inbound connection, it checks does it exist already in case it must overwrite it.
If manager connection exists, then client and client's lock flag should exist too.
In this case it might be used by the handshake of the `p(u)`.

if the client is locked, then set the `halt` flag of the manager connection. set context status to `STATUS_MANAGER_BUSY`.

---
Otherwise it recreates the client and generates the hmac, computes the hash.
It sets the status to `STATUS_MANAGER_WAITING` and returns the computed hash as well.

#### How dependency updates the status of inbound connection?
The `p(u)` has a dependency.

1. it's set in the handshakeDep using the connect manager.
see the phase 1.

---

## Phase 2 methods

### `EvaluateStatus` handling function
Receives the status as `ServiceStatusMsg` with the 6 parts.
Each of them evaluated differently.

#### part 1: outbounds
if status from service is CHECKING, simply skip it.
invert the dependency's outbounds sent by route to route's inbounds in dependency (that evaluates status).

#### part 2: outbound connections
if status from service is CHECKING, simply skip it.
if context and calling service urls match then:
    inverts the dependency's outbound connection sent by route to outbound target's manager connection in dependency.

    the manager connection will not have the client to send out. only a public key, hmac, status, and computed hash.

    if manager connection doesn't exist it will return `STATUS_MANAGER_NOT_FOUND`.
    if manager connection is `STATUS_MANAGER_WAITING`, it will return it back to the root.
    if manager connection is `STATUS_MANAGER_SECURED`:
        then, checks the status of the outbound connection in the root.
        if root's outbound connection status is `STATUS_MANAGER_UNSET`, it will return `STATUS_MANAGER_SECURED` and computed hash. it means the root is crashed and lost status, evaluating status restores the state for root.
        if root's outbound connection status is also `STATUS_MANAGER_SECURED`, it will compare the computed hash.
            if hash mismatches it returns `STATUS_MANAGER_MISMATCH`.
            if hash matches it returns `STATUS_MANAGER_SECURED`.
if context and calling service urls mismatch then:
    checks does it exist in the topology context as context, calling service and does it exist manager connection without client. if not skips otherwise checks the status. if status is secured, ticks it.
    Otherwise, it returns `STATUS_MANAGER_NOT_FOUND`.

---
the caller will tick its own topology context if its secured and evaluating status also returns secured status.

##### part 3: inbound connections
if status from service is CHECKING, simply skip it.
inverts the dependency's inbound connection sent by route to inbound's target's manager connection in the dependency.
it checks for the manager client, and if its not set (like in first time) it will return `STATUS_MANAGER_NOT_FOUND`.
if manager status is not secured it will return that status simply.
if manager status is secured, it will compare the hash (that is received from the root) and if mismatch will return status `STATUS_MANAGER_MISMATCH`. Otherwise it will return `STATUS_MANAGER_SECURED`.

### `WhitelistCaller` handling function
It receives the hmac from the caller and calls own manager's whitelist function.
The manager of the caller then will be allowed to call this manager's functions.

If context url and service url are different, then whitelist caller checks the topology context.
If it doesn't exist it returns error, indicating protocol error.

If it exists it updates the status to `STATUS_MANAGER_SECURED` and sets the hmac there too.
It also computes the hash using hmac, own public key, remote public key as well as own url and remote url.

This hash is then used for the evalutating the status so its kept internally and also returned to the caller.

---
the caller `i(h)` checks the returned result of evaluation.
if its mismatch it will reset the hash both in the outbound connection and in the inbound connection. and also it will reload the topology.
if its secured, then it will store the computed hash in itself and in the outbound connection as well.

#### Format
context URL, service URL, hmac

## Phase 2 Status codes

STATUS_ROUTE_* is set by root and dependency for the inbounds and outbounds.
STATUS_ROUTE_UNSET is set by the root initially. It means need to call secureInbounds or secureOutbounds.
STATUS_ROUTE_CHECKING is set by secureOutbounds or secureInbounds in the beginning. If the connection to dependency takes longer time, then next handshake will not call secure* methods again until previous function not completes.
STATUS_ROUTE_NOT_FOUND is set by dependency if topology context is not given for the given outbounds or inbounds.
STATUS_ROUTE_SECURED is set by dependency if topology context exist and its correct.

STATUS_MANAGER_* is set between manager to manager connection in any topology context (root to dependency or inbound/outbound connections)
STATUS_MANAGER_UNSET is set by the root initially. It means its not yet done anything for the security.
STATUS_MANAGER_NOT_FOUND is set by the dependency regarding the outbound or inbound connections.
STATUS_MANAGER_SECURED is set when manager connection is established and its correct.
STATUS_MANAGER_CHECKING is set by secureOutboundConnection, secureInboundConnection in the beginning. If the connection to dependency takes longer time, then next handshake will not call secure* methods again by until previous function not completes.
STATUS_MANAGER_WAITING is set by the secureOutboundConnection internally. It indicates that it has topology context from root, and waits actual connection from the service.
STATUS_MANAGER_MISMATCH is set by the secureOutboundConnection internally. It indicates that some hack and attempt to rewrite the public keys or hmac. The computed hash is for the edge cases when an author wants to change the compromised hmac or private keys. Although private key changing will automatically resolve it, but its not the same for hmac. I hope it will be just a scare code but no one actually reaches it.
STATUS_MANAGER_BUSY is set by secureInboundConnection if root tries to overwrite it. It indicates the dependency is handshaking, and asks to come back.
STATUS_MANAGER_ERR is the protocol error.
STATUS_MANAGER_NO_WHITELIST is indicating that service denied access due to HMAC error. Needs to whitelist it.
STATUS_MANAGER_NOT_AUTH is indicating that service denied access at the protocol level due to CURVE keypair error. It needs to allow(), and/or secure() to authenticate using curve keypair.

---

# Todo

2. Change the status names as they are chaotic
CONNECTION_STATUS_* for manager connection, and outbounds and every status is confusing.

Need to clarify and a way to tell it says about progress.

STATUS_MANAGER_* for manager connections.
STATUS_ROUTE_* for outbounds, and inbounds. It should have names as MISMATCH, NOT_FOUND, and CONNECTED should be renamed to SECURED.
STATUS_EDGE_* for the edges

3. SecureOutbounds runs a goroutine for each outbound route, network makes it slow need to make it one request known as SecureOutbounds.

4. Implement [status 2. inbounds](#status-2-inbounds), requires todo 8

5. Rename Handshake command and onHanshake handle function, along whitelistSelfinDeps to one name otherwise confusing.
Rename command to WhitelistCaller and handling function to onWhitelistCaller.

6. Update the evaluate status to look for connection manager if context and service mismatch.

7. optimize the public key sharing, what if its hardcoded in the configuration, and doesn't need to manage during manager connection? for now it always manages it. (least priority).

8. implement the extension's second feature working with the siblings, plus include in showcase/tutorial.

9. implement the live update of the topology via manager: first adding new proxy, adding new extension. removing them. removing predefined one. And also using this two to change the endpoint of the existing parameters as well. It needs to design the conflict resolution with the hardcoded configuration.

10. implement live update of the nested proxies or nested extensions. implement working of the tree within a nested proxy or extension does socket resolution works well.

11. rename in current code the secureInboundEdgeCon and secureOutboundEdgeCon to secureInboundCon and secureOutboundCon.

12. add `halt` flag to handshakeDep function. the connect manager checks the halt and if it exists skips the process entirely. After connect manager completes and before evaluating the status (phase 2), check the halt flag if it exists it stops without going to second phase.

14. Rename allowSelfInDep to allowCaller()