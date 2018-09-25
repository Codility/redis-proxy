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
