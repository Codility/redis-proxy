Switch test
===========

`switch-test` is a testing utility for `redis-proxy`.

It starts a single Redis server (redis-a), two `redis-proxy` instances
(proxy-a, proxy-b), and configures them to:

* proxy-a uses redis-a as upstream
* proxy-b daisychains proxy-b

When it receives SIGHUP it will start another Redis process from
scratch (redis-b), go through the replacement procedure, so that at
the end:

* proxy-a daisychains proxy-b
* proxy-b uses redis-b as upstream

all without breaking client connections or losing data.

Subsequent SIGHUPs alternate between those two setups, with either
redis-a or redis-b as upstream, and every time `switch-test` recreates
the Redis instance from scratch going through the full DB replacement
process.

Simple way to test `redis-proxy`:

Start `switch-test`, look for connection details in output:

    $ ./switch-test
    18:27:55 Starting redis-server [- --dbfilename save-6001-1497889675.rdb --requirepass pass-6001 --port 6001]
    [...]
    18:27:56 Starting redis-proxy at 7101, password: 'pass-7101'
    18:27:56 Starting redis-proxy at 7201, password: 'pass-7201'
    [...]
    18:27:57 A: 7101 -> 6001, running, active: 0;  B: 7201 -> 7101, running, active: 0

It contains, among other things, connection information for both proxy
instances, and their status.  The last line above means: proxy A is
running, forwarding from port 7101 to 6001, with 0 active requests;
proxy B is running, forwarding from port 7201 to 7101, 0 active
requests.

In another terminal [tab], start a `redis-cli` command that will
bounce data off of that Redis server via one of the two proxy
instances:

    $ redis-cli -a pass-7101 -p 7101 -r 9999 -i 1 \
      eval 'local i = redis.call("get", "i");
            if i == false then i = 0 end;
            redis.call("set", "i", i+1);
            return i' 0
    (integer) 0
    "1"
    "2"
    "3"
    "4"
    [...]

In a third tab, send SIGHUP to `switch-test`:

    $ pkill -HUP switch-test

You should see a reaction in the tab with `switch-test`, showing how
it goes through the replacement procedure, and if all goes well it
should end with:

    18:31:49 A: 7101 -> 7201, running, active: 0;  B: 7201 -> 6002, running, active: 0

Meaning both proxy instances are running, but now A forwards from port
7101 to 7201, and B forwards from 7201 to 6002.

During this time the `redis-cli` command should continue to count
consecutive integer numbers without any disruption other than delayed
response to some queries (NOTE: `redis-cli` will reconnect
automatically in case of errors, TODO: see if we can easily replace it
with something that would not).
