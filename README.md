Redis Proxy
===========


Current state
-------------

 - TODO: add TLS to listener and admin
 - TODO: add TLS to uplink (including reloads)
 - TODO: keepalive?
 - TODO: disconnect controller from RedisProxy completely
 - TODO: get rid of circular dependency between RedisProxy and ProxyController

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
# unpause
curl http://localhost:7011/cmd/ -d cmd=unpause
# reload config (acts like pause + reload + unpause)
curl http://localhost:7011/cmd/ -d cmd=reload
```
