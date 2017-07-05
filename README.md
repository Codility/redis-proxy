Redis Proxy
===========

Build status (Travis)
---------------------

  * Master: ![status: master](https://travis-ci.org/Codility/redis-proxy.svg?branch=master)
  * Develop: ![status: develop](https://travis-ci.org/Codility/redis-proxy.svg?branch=develop)

Build
-----

The repository includes all dependencies other than standard library.
You need go 1.8.x.


    $ git clone git@github.com:Codility/redis-proxy.git
    $ cd redis-proxy
    $ make


Branching
---------

Use a minimal git-flowish branch model:

   * `master` means production-ready, change it only by merging
     hotfixes or `develop`; MR or direct merge from `develop` is okay,
     there's no need for full git-flow release process,
   * `develop` branch is the main integration point, merge finished
     feature branches here,
   * feature branches: use Codility standard `author|team/branch-desc`
     names.


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
