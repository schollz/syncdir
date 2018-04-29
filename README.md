# syncdir

Easily keep directories on local networks in sync.

*syncdir* allows any two computers to stay in sync on a local network. Just run in the directory you want to sync on each computer and they will stay in sync. Each computer will discover another and then they will update each other on a file change (file creation/deletion/modification and permissions change). The first directory to change will change all the others.

_Experimental! Try it but make sure to backup your folder first..._
## Example

_Press Ctl + F5 to have these gifs run simultaneously._

**Computer 1:**

![computer 1](https://raw.githubusercontent.com/schollz/syncdir/master/cmd/1.gif)

**Computer 2:**

![computer 1](https://raw.githubusercontent.com/schollz/syncdir/master/cmd/2.gif)


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