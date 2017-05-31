Redis Proxy
===========


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


Ideas
-----

PAUSE: expose an HTTP interface with current status on GET, and POST
operations for PAUSE and UNPAUSE.

All the commands we're interested in match the simple request-response
pattern.  There is no need to handle push data from the server.


Config file
-----------

```
{
  "uplink_addr": "localhost:6379",
  "listen_on": "127.0.0.1:7010",
  "admin_on": "127.0.0.1:7011"
}
```
