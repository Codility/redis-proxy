Redis Proxy
===========


Current state
-------------

 - TODO: authentication
   - authenticate clients in proxy
   - read Redis password from config file
 - TODO: keep track of SELECTed db, re-select after switch
 - TODO: add TLS to listener and admin
 - TODO: add TLS to uplink (including reloads)
 - TODO: keepalive?
 - TODO: get rid of circular dependency between RedisProxy and ProxyController

 - TODO: switch-test: wait for replication to really catch up
 - TODO: move switchover logic to proxy (old proxy can handle the entire process)


Possible optimizations
----------------------

 - Do not recreate the entire response in memory, pass it directly to
   the client instead.
 - Parse just enough of the message to figure out if it's one of
   commands to handle (SELECT, AUTH), otherwise pass it directly to
   uplink.

Requirements
------------

- Accept TLS connections
- Forward all connections to a Redis uplink (TLS or non-TLS, depending
  on configuration)
- Provide a PAUSE operation:
  - suspend execution of any new commands
  - wait for currently-executing commands to finish
  - [later] put a hard timeout on that wait, drop connections that
    take longer
  - this operation needs to be synchronous: we need to be able to tell
    when all clients have been effectively suspended
- reload configuration and switch between uplink Redis servers without
  severing client connections
- [later] keep client connections stateless: other than short-time
  blocking reads like BLPOP, reject all commands that might change the
  state (like PSUBSCRIBE)


All the commands we're interested in match the simple request-response
pattern.  There is no need to handle push data from the server.


Usage
-----

Create config.json based on config_example.json

To reload config.json, send HUP to the process or use http interface:

```
# pause (returns immediately)
curl http://localhost:7011/cmd/ -d cmd=pause
# pause-and-wait (returns after all connections are suspended)
curl http://localhost:7011/cmd/ -d cmd=pause-and-wait
# unpause
curl http://localhost:7011/cmd/ -d cmd=unpause
# reload config (acts like pause + reload + unpause)
curl http://localhost:7011/cmd/ -d cmd=reload
```
