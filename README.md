Redis Proxy
===========

**NOTE**: This project uses git-flow[ish] layout: use `develop` branch
as main integration point, and `master` as production-ready.


Build
-----

The repository includes all dependencies other than standard library.
You need go 1.8.x.


    $ git clone git@gitlab.codility.net:marcink/redis-proxy.git
    $ cd redis-proxy
    $ make


Documentation
-------------

See doc/redis-proxy.md.

For simple testing framework see doc/switch-test.md.

Usage
-----

Create config file based on config_example.json, start `redis-proxy -f
<config-file>`.  See doc/redis-proxy.md for details.


Current state
-------------

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
