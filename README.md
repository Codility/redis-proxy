Redis Proxy
===========

Build status (Travis)
---------------------

  * Master: [![status: master](https://travis-ci.org/Codility/redis-proxy.svg?branch=master)](https://travis-ci.org/Codility/redis-proxy)
  * Develop: [![Build Status](https://travis-ci.org/Codility/redis-proxy.svg?branch=develop)](https://travis-ci.org/Codility/redis-proxy)


Overview
--------

Redis-proxy fulfills a very specific set of requirements:

* TLS termination: expose TLS socket, forward requests to non-TLS
  Redis.
* Forward connections to upstream Redis (TLS or non-TLS, both work).
  Every client gets its own, isolated connection to uplink Redis.
* PAUSE operation:
  * suspend execution of any new commands,
  * wait for currently-executing commands to finish.
* RELOAD operation:
  * verify new configuration, reject it in case of errors (this
    includes an attempt to connect to uplink),
  * PAUSE,
  * load new configuration and resume normal operation without
    breaking client connections.
* Enforce a hard limit on all commands, disconnect clients who break
  it.  This allows pause-and-reload to finish in guaranteed time.
* Separate authentication on client and upstream side: you can
  configure the proxy to require clients to authenticate, and the
  proxy can authenticate with upstream Redis.  These passwords are
  independent, and the proxy does NOT forward AUTH commands from the
  client to upstream Redis.
* Keep track of SELECTed database for every client and re-SELECT it
  after replacing upstream Redis.
* HTTP[S] API for reading and controlling Proxy state.

The tradeoff is that it does not support PUB/SUB, because it would
require moving too much of client state between upstream connections.


Configuration file
------------------

JSON.  Example of full file (comments are here for clarity, but are not
supported in actual configuration):

    {
      "uplink": {                   # <- Upstream Redis.  This is where Proxy
        "addr": "localhost:6379",   #   forwards requests.
        "pass": "redis-password",   # <- Optional.  Clients must AUTH if set.
        "tls": true,
        "cacertfile": "cacert.pem", # <- TLS requires cacertfile, unless skipverify is set.
        "skipverify": false         # <- Optionally disable cert verification.
      },
      "listen": {                   # <- Proxy server.  This is where Proxy
        "addr": "127.0.0.1:7010",   #    clients connect.
        "pass": "client-password",  # <- Optional.  Clients must AUTH if set.
        "tls": true,
        "certfile": "cert.pem",     # <- TLS requires certfile and keyfile.
        "keyfile": "key.pem"
      },
      "listen_raw": {               # <- Raw Proxy server.  Provides unmanaged,
        "addr": "127.0.0.1:7010",   #    full-dupliex proxy to uplink.
        "tls": true,
        "certfile": "cert.pem",     # <- TLS requires certfile and keyfile.
        "keyfile": "key.pem"
      },
      "admin": {                    # <- Admin UI (http[s]).  This is where
        "addr": "127.0.0.1:7011",   #    you can see and control proxy state.
        "tls": true,
        "certfile": "cert.pem",     # <- TLS requires certfile and keyfile.
        "keyfile": "key.pem"
      },
      "log_messages": false,        # <- Log all traffic to stderr.
      "read_time_limit_ms": 5000    # <- Hard limit on forwarded requests.
    }

The proxy validates config file at startup, and also when told to
reload while running.

At this point the proxy supports changes only to `listen` subtree on
reload.  Changes in any other place will result in the proxy rejecting
the config file.

Use SIGHUP to reload configuration.  It is safe to do without pausing:
it will not terminate any requests, any ongoing requests will


TLS
---

The proxy will validate server if uplink is configured for TLS.  You
must provide the right CA cert to have TLS uplink.

It does not support TLS-level client authentication on any connection.


HTTP[s] API
-----------

Open `admin.addr` to see proxy status.

Command overview:

* pause: suspend all client connections, return immediately
* unpause: resume client connections, return immediately
* reload: reload configuration, return when complete
* terminate-raw-connections: terminate all connections made via listen_raw

To execute any command, POST to `<admin.addr>/cmd/` with
`cmd=<command>`.  For example:

    curl http://127.0.0.1:7011/cmd/ -d cmd=pause

All commands return HTTP status code 200 if successful, but in case of
`-async` commands it means only that the request was sent sucessfully,
or one of 4xx or 5xx HTTP codes otherwise.  The response body empty
JSON:

    {"ok": true}

if successful, OR:

    {
        "ok": false,
        "error": "<error>"
    }

otherwise.


Usage
-----

Create config file based on config_example.json, start `redis-proxy -f
<config-file>`.  See doc/redis-proxy.md for details.


Current state
-------------

 - TODO: stop admin UI when stopping proxy
 - TODO: allow changing `log_messages` and `read_time_limit_ms` on
   config reload (or at least reject those changes)
 - TODO: strict `Config.ValidateSwitchTo`: whitelist instead of blacklisting.
 - TODO: http auth in admin UI
 - TODO: a command to verify a config file without attempting to load
   it
 - TODO: give feedback in reload API if new config is broken
 - TODO: use TLS in switch-test
 - TODO: add TLS client verification (in listen, admin, uplink)
 - TODO: switch-test: wait for replication to really catch up
 - TODO: move switchover logic to proxy (old proxy can handle the entire process)
 - TODO: nicer Proxy api (get rid of proxy.controller.* calls from the outside)
 - TODO: allow IP addresses in test certificates (so that tests can use 127.0.0.1 instead of localhost)


Possible optimizations
----------------------

 - Do not recreate the entire response in memory, pass it directly to
   the client instead.


Development process
-------------------

See doc/dev-process.md

