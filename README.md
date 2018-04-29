# syncdir

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