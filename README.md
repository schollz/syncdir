# syncdir

<a href="https://travis-ci.org/schollz/syncdir"><img src="https://travis-ci.org/schollz/syncdir.svg?branch=master&style=flat-square" alt="Build Status"></a>
<a href="https://github.com/schollz/syncdir/releases/latest"><img src="https://img.shields.io/badge/version-1.0.0-brightgreen.svg?style=flat-square" alt="Version"></a>
<a href="https://goreportcard.com/report/github.com/schollz/syncdir"><img src="https://goreportcard.com/badge/github.com/schollz/syncdir" alt="Go Report Card"></a>

Easily directories on local networks in sync.

*syncdir* allows any two computers to stay in sync on a local network. Just run in the directory you want to sync on each computer and they will stay in sync.

## Install

To install, download [the latest release](https://github.com/schollz/syncdir) or install from source:

```
$ go get -v github.com/schollz/syncdir/...
```

## Run 

On `Computer 1`:

```
$ cd directory_to_sync && syncdir
```

And on `Computer 2`:

```
$ cd directory_to_sync && syncdir
```

*syncdir* will automatically discover the other computers and sync the files. The synchronization will create/delete based on the directory on the computer that created the last change. Dotfiles are ignored.

## License 

MIT